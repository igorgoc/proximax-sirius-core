package network

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type PortCheckResult struct {
	PublicIP       string `json:"publicIp"`
	LocalIP        string `json:"localIp"`
	RouterName     string `json:"routerName"`
	UPnPActive     bool   `json:"upnpActive"`
	UPnPMessage    string `json:"upnpMessage"`
	Port7900Open   bool   `json:"port7900Open"`
	Port7904Open   bool   `json:"port7904Open"`
	Port7900Status string `json:"port7900Status"`
	Port7904Status string `json:"port7904Status"`
	LastCheckTime  string `json:"lastCheckTime"`
}

type NetworkManager struct {
	mu           sync.RWMutex
	lastResult   PortCheckResult
	httpClient   *http.Client
	controlURL   string
	serviceType  string
	routerName   string
	localIP      string
	upnpSuccess  bool
}

func NewNetworkManager() *NetworkManager {
	nm := &NetworkManager{
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
		},
		lastResult: PortCheckResult{
			Port7900Status: "Checking...",
			Port7904Status: "Checking...",
			LastCheckTime:  time.Now().Format("2006-01-02 15:04:05 MST"),
		},
	}

	// Auto-forward ports in background on startup and poll reachability periodically
	go func() {
		nm.DiscoverAndMapUPnP()
		nm.CheckPortReachability()

		ticker := time.NewTicker(8 * time.Second)
		for range ticker.C {
			nm.CheckPortReachability()
		}
	}()

	return nm
}

// DiscoverAndMapUPnP discovers the home router using SSDP and adds port mappings for 7900 and 7904
func (nm *NetworkManager) DiscoverAndMapUPnP() {
	localIP := getOutboundIP()
	if localIP == "" {
		localIP = "127.0.0.1"
	}
	nm.localIP = localIP

	controlURL, serviceType, routerName, err := discoverUPnPRouter()
	nm.mu.Lock()
	nm.routerName = routerName
	nm.controlURL = controlURL
	nm.serviceType = serviceType
	nm.mu.Unlock()

	if err != nil {
		nm.mu.Lock()
		nm.upnpSuccess = false
		nm.lastResult.UPnPActive = false
		nm.lastResult.UPnPMessage = "UPnP router discovery not available (Manual port forwarding may be used)"
		nm.lastResult.RouterName = "Generic Router / Firewall"
		nm.mu.Unlock()
		return
	}

	// Add Port 7900 TCP (Consensus)
	err7900 := addPortMapping(controlURL, serviceType, localIP, 7900, "TCP", "Sirius Consensus P2P")
	// Add Port 7904 TCP & UDP (Storage Replicator)
	err7904Tcp := addPortMapping(controlURL, serviceType, localIP, 7904, "TCP", "Sirius Storage TCP")
	err7904Udp := addPortMapping(controlURL, serviceType, localIP, 7904, "UDP", "Sirius Storage UDP")

	nm.mu.Lock()
	if err7900 == nil || err7904Tcp == nil || err7904Udp == nil {
		nm.upnpSuccess = true
		nm.lastResult.UPnPActive = true
		nm.lastResult.UPnPMessage = fmt.Sprintf("Ports 7900 & 7904 automatically forwarded via %s", routerName)
		nm.lastResult.RouterName = routerName
	} else {
		nm.upnpSuccess = false
		nm.lastResult.UPnPActive = false
		nm.lastResult.UPnPMessage = fmt.Sprintf("Router (%s) found, but UPnP port mapping was declined by router", routerName)
		nm.lastResult.RouterName = routerName
	}
	nm.mu.Unlock()
}

// CheckPortReachability detects external public IP and checks connectivity
func (nm *NetworkManager) CheckPortReachability() PortCheckResult {
	pubIP := nm.detectPublicIP()
	localIP := nm.localIP
	if localIP == "" {
		localIP = getOutboundIP()
	}

	port7900Open := isPortListening(7900)
	port7904Open := isPortListening(7904)

	nm.mu.Lock()
	defer nm.mu.Unlock()

	status7900 := "Open (Listening)"
	if !port7900Open {
		status7900 = "Waiting for node container"
	} else if nm.upnpSuccess {
		status7900 = "Open & UPnP Forwarded"
	}

	status7904 := "Open (Listening)"
	if !port7904Open {
		status7904 = "Waiting for node container"
	} else if nm.upnpSuccess {
		status7904 = "Open & UPnP Forwarded"
	}

	res := PortCheckResult{
		PublicIP:       pubIP,
		LocalIP:        localIP,
		RouterName:     nm.routerName,
		UPnPActive:     nm.upnpSuccess,
		UPnPMessage:    nm.lastResult.UPnPMessage,
		Port7900Open:   port7900Open,
		Port7904Open:   port7904Open,
		Port7900Status: status7900,
		Port7904Status: status7904,
		LastCheckTime:  time.Now().Format("2006-01-02 15:04:05 MST"),
	}

	if res.RouterName == "" {
		res.RouterName = "Home Router / Gateway"
	}
	if res.UPnPMessage == "" {
		if res.UPnPActive {
			res.UPnPMessage = "UPnP active"
		} else {
			res.UPnPMessage = "Standard NAT Mode"
		}
	}

	nm.lastResult = res
	return res
}

func (nm *NetworkManager) GetLastResult() PortCheckResult {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.lastResult
}

func (nm *NetworkManager) detectPublicIP() string {
	ipProviders := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}

	for _, provider := range ipProviders {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", provider, nil)
		if err != nil {
			cancel()
			continue
		}

		resp, err := nm.httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			cancel()
			ip := strings.TrimSpace(string(body))
			if parsed := net.ParseIP(ip); parsed != nil {
				return ip
			}
		}
		if resp != nil {
			resp.Body.Close()
		}
		cancel()
	}

	return "Unavailable"
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func isPortListening(port int) bool {
	targets := []string{
		fmt.Sprintf("sirius-mainnet-peer:%d", port),
		fmt.Sprintf("127.0.0.1:%d", port),
		fmt.Sprintf("host.docker.internal:%d", port),
	}
	for _, target := range targets {
		conn, err := net.DialTimeout("tcp", target, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

// ----------------------------------------------------
// SSDP & UPnP IGD Protocol Implementation
// ----------------------------------------------------

func discoverUPnPRouter() (controlURL, serviceType, routerName string, err error) {
	ssdpMsg := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"ST: urn:schemas-upnp-org:service:WANIPConnection:1\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n\r\n"

	socket, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return "", "", "", err
	}
	defer socket.Close()

	_ = socket.SetDeadline(time.Now().Add(2 * time.Second))
	dest := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	if _, err := socket.WriteTo([]byte(ssdpMsg), dest); err != nil {
		return "", "", "", err
	}

	buf := make([]byte, 2048)
	n, _, err := socket.ReadFrom(buf)
	if err != nil {
		// Try WANPPPConnection fallback
		ssdpMsg2 := "M-SEARCH * HTTP/1.1\r\n" +
			"HOST: 239.255.255.250:1900\r\n" +
			"ST: urn:schemas-upnp-org:service:WANPPPConnection:1\r\n" +
			"MAN: \"ssdp:discover\"\r\n" +
			"MX: 2\r\n\r\n"
		if _, wErr := socket.WriteTo([]byte(ssdpMsg2), dest); wErr == nil {
			_ = socket.SetDeadline(time.Now().Add(2 * time.Second))
			n, _, err = socket.ReadFrom(buf)
		}
		if err != nil {
			return "", "", "", err
		}
	}

	rawResp := string(buf[:n])
	location := ""
	for _, line := range strings.Split(rawResp, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "location:") {
			location = strings.TrimSpace(line[9:])
			break
		}
	}

	if location == "" {
		return "", "", "", fmt.Errorf("no UPnP location header found")
	}

	// Fetch device XML description
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(location)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	// Parse XML description
	var devDesc struct {
		Device struct {
			FriendlyName string `xml:"friendlyName"`
			ModelName    string `xml:"modelName"`
		} `xml:"device"`
	}
	_ = xml.Unmarshal(body, &devDesc)

	routerName = devDesc.Device.FriendlyName
	if routerName == "" {
		routerName = devDesc.Device.ModelName
	}
	if routerName == "" {
		routerName = "UPnP Gateway Router"
	}

	// Extract controlURL for WANIPConnection or WANPPPConnection
	bodyStr := string(body)
	serviceType = "urn:schemas-upnp-org:service:WANIPConnection:1"
	if strings.Contains(bodyStr, "urn:schemas-upnp-org:service:WANPPPConnection:1") {
		serviceType = "urn:schemas-upnp-org:service:WANPPPConnection:1"
	}

	ctrlTag := "<controlURL>"
	idx := strings.Index(bodyStr, ctrlTag)
	if idx == -1 {
		return "", "", "", fmt.Errorf("controlURL not found in device description")
	}
	endIdx := strings.Index(bodyStr[idx:], "</controlURL>")
	if endIdx == -1 {
		return "", "", "", fmt.Errorf("malformed controlURL tag")
	}

	relCtrlURL := strings.TrimSpace(bodyStr[idx+len(ctrlTag) : idx+endIdx])
	baseURL := location
	if uIdx := strings.Index(location[8:], "/"); uIdx != -1 {
		baseURL = location[:8+uIdx]
	}
	if strings.HasPrefix(relCtrlURL, "/") {
		controlURL = baseURL + relCtrlURL
	} else if strings.HasPrefix(relCtrlURL, "http://") || strings.HasPrefix(relCtrlURL, "https://") {
		controlURL = relCtrlURL
	} else {
		controlURL = baseURL + "/" + relCtrlURL
	}

	return controlURL, serviceType, routerName, nil
}

func addPortMapping(controlURL, serviceType, localIP string, port int, proto, desc string) error {
	soapBody := fmt.Sprintf(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
<s:Body>
<u:AddPortMapping xmlns:u="%s">
<NewRemoteHost></NewRemoteHost>
<NewExternalPort>%d</NewExternalPort>
<NewProtocol>%s</NewProtocol>
<NewInternalPort>%d</NewInternalPort>
<NewInternalClient>%s</NewInternalClient>
<NewEnabled>1</NewEnabled>
<NewPortMappingDescription>%s</NewPortMappingDescription>
<NewLeaseDuration>0</NewLeaseDuration>
</u:AddPortMapping>
</s:Body>
</s:Envelope>`, serviceType, port, proto, port, localIP, desc)

	req, err := http.NewRequest("POST", controlURL, bytes.NewBufferString(soapBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	req.Header.Set("SOAPAction", fmt.Sprintf("\"%s#AddPortMapping\"", serviceType))

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("router rejected port mapping with status: %s", resp.Status)
	}

	return nil
}
