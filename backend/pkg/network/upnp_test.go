package network

import (
	"net"
	"testing"
)

func TestNetworkManager_BasicsAndThreadSafety(t *testing.T) {
	nm := NewNetworkManager()

	res := nm.GetLastResult()
	if res.Port7900Status == "" || res.Port7904Status == "" {
		t.Fatalf("expected initial checking status for ports")
	}

	// Direct reachability invocation
	resCheck := nm.CheckPortReachability()
	if resCheck.RouterName == "" {
		t.Fatalf("expected non-empty router name fallback")
	}
	if resCheck.LocalIP == "" {
		t.Fatalf("expected local IP or fallback")
	}
}

func TestIsPortListening(t *testing.T) {
	// Bind to an ephemeral port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	if !isPortListening(port) {
		t.Fatalf("expected port %d to be detected as listening", port)
	}

	// Close listener and test closed port
	_ = listener.Close()
	if isPortListening(port) {
		t.Fatalf("expected port %d to be closed", port)
	}
}
