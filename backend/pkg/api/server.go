package api

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"proximax-sirius-core/pkg/chain"
	"proximax-sirius-core/pkg/config"
	"proximax-sirius-core/pkg/crypto"
	"proximax-sirius-core/pkg/supervisor"
	"proximax-sirius-core/pkg/migrator"
	"proximax-sirius-core/pkg/network"
	"proximax-sirius-core/pkg/snapshot"
	"proximax-sirius-core/pkg/storage"
	"proximax-sirius-core/pkg/updater"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0"
	},
}

type Server struct {
	configMgr        *config.ConfigManager
	supervisor       *supervisor.ProcessSupervisor
	chainMon         *chain.ChainMonitor
	harvesterTracker *chain.HarvesterTracker
	updateMgr        *updater.UpdateManager
	engineUpdater    *updater.EngineUpdater
	storageMgr       *storage.StorageManager
	networkMgr       *network.NetworkManager
	migrator         *migrator.Migrator
	snapshotMgr      *snapshot.SnapshotManager
	staticFS         fs.FS
	apiToken         string
	wsMutex          sync.Mutex
	activeWsConns    int
	nativePickerMu   sync.Mutex
}

func NewServer(configMgr *config.ConfigManager, supervisor *supervisor.ProcessSupervisor, chainMon *chain.ChainMonitor, staticFS fs.FS) *Server {
	ht := chain.NewHarvesterTracker(configMgr.GetResourcesPath())
	if cfg, err := configMgr.LoadNodeConfig(); err == nil && cfg.HarvestPublicKey != "" {
		ht.SetHarvestPublicKey(cfg.HarvestPublicKey)
	}
	ht.StartBackgroundScanner(chainMon)

	um := updater.NewUpdateManager(configMgr.GetResourcesPath(), supervisor)
	go func() {
		_, _ = um.CheckUpdate()
	}()

	manifestPath := filepath.Join(filepath.Dir(configMgr.GetResourcesPath()), "engine.compat.json")
	eu := updater.NewEngineUpdater(supervisor.GetBinPath(), manifestPath, supervisor, nil)
	go func() {
		if !eu.IsEngineInstalled() {
			log.Printf("[Sirius Engine] No native engine binary detected in %s. Initiating automatic initial engine setup...", supervisor.GetBinPath())
			targetVer := eu.GetStatus().TargetVersion
			if targetVer == "" || targetVer == "none" {
				targetVer = "v1.9.8"
			}
			dataPath := configMgr.GetDataPath()
			if err := eu.DownloadAndApplyUpdate(targetVer, dataPath); err != nil {
				log.Printf("[Sirius Engine] Initial engine setup failed: %v", err)
			}
		} else {
			_, _ = eu.CheckUpdate("")
		}
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			_, _ = eu.CheckUpdate("")
		}
	}()

	bootKeyGetter := func() string {
		if cfg, err := configMgr.LoadNodeConfig(); err == nil {
			return cfg.BootKey
		}
		return ""
	}

	sm := storage.NewStorageManager(configMgr.GetResourcesPath(), configMgr.GetDataPath, bootKeyGetter)
	nm := network.NewNetworkManager()
	mig := migrator.New()

	releasePubKey := "68b1a927c47850a5d4244305b2670960d9fe6f048e5a458d801959b606f3e917"
	if snapManifestBytes, err := os.ReadFile("chainconfig/snapshot.compat.json"); err == nil {
		var smManifest struct {
			ReleasePublicKeyHex string `json:"releasePublicKeyHex"`
		}
		if json.Unmarshal(snapManifestBytes, &smManifest) == nil && smManifest.ReleasePublicKeyHex != "" {
			releasePubKey = smManifest.ReleasePublicKeyHex
		}
	} else if eu != nil {
		if m, err := eu.LoadManifest(); err == nil && m.ReleasePublicKeyHex != "" {
			releasePubKey = m.ReleasePublicKeyHex
		}
	}
	snapMgr := snapshot.NewSnapshotManager(supervisor, releasePubKey, nil)

	// Start self-healing process watchdog
	supervisor.StartWatchdog(configMgr.GetDataPath)

	apiToken := configMgr.GetOrCreateApiToken()
	log.Printf("[Sirius Core] Secure API token initialized (%d chars)", len(apiToken))

	return &Server{
		configMgr:        configMgr,
		supervisor:       supervisor,
		chainMon:         chainMon,
		harvesterTracker: ht,
		updateMgr:        um,
		engineUpdater:    eu,
		storageMgr:       sm,
		networkMgr:       nm,
		migrator:         mig,
		snapshotMgr:      snapMgr,
		staticFS:         staticFS,
		apiToken:         apiToken,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Authentication Token Route
	mux.HandleFunc("/api/auth/token", s.handleAuthToken)

	// API Routes
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/harvesting/stats", s.handleHarvestStats)
	mux.HandleFunc("/api/harvesting/check-now", s.handleHarvestCheckNow)
	mux.HandleFunc("/api/harvesting/reset", s.handleHarvestReset)
	mux.HandleFunc("/api/node/start", s.handleNodeStart)
	mux.HandleFunc("/api/node/stop", s.handleNodeStop)
	mux.HandleFunc("/api/node/restart", s.handleNodeRestart)
	mux.HandleFunc("/api/node/watchdog/toggle", s.handleWatchdogToggle)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/logs/stream", s.handleLogsStream)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/config/save", s.handleConfig)
	mux.HandleFunc("/api/config/raw", s.handleConfigRaw)
	mux.HandleFunc("/api/config/files", s.handleConfigFilesList)
	mux.HandleFunc("/api/keys/generate", s.handleKeyGenerate)
	mux.HandleFunc("/api/keys/parse", s.handleKeyParse)
	mux.HandleFunc("/api/harvesting/link", s.handleHarvestingLink)
	mux.HandleFunc("/api/peers", s.handlePeers)
	mux.HandleFunc("/api/network/peers-detail", s.handlePeersDetail)
	mux.HandleFunc("/api/network/public-ip", s.handlePublicIp)
	mux.HandleFunc("/api/network/peer-json", s.handlePeerJson)
	mux.HandleFunc("/api/network/port-check", s.handlePortCheck)
	mux.HandleFunc("/api/network/validator-stats", s.handleNetworkValidatorStats)
	mux.HandleFunc("/api/network/upnp/remap", s.handleUPnPRemap)
	mux.HandleFunc("/api/harvesting/check", s.handleHarvestingCheck)
	mux.HandleFunc("/api/maintenance/reset", s.handleMaintenanceReset)
	mux.HandleFunc("/api/maintenance/snapshot", s.handleMaintenanceSnapshot)
	mux.HandleFunc("/api/maintenance/snapshot/cancel", s.handleMaintenanceSnapshotCancel)
	mux.HandleFunc("/api/maintenance/snapshot/status", s.handleMaintenanceSnapshotStatus)
	mux.HandleFunc("/api/maintenance/data-backup", s.handleMaintenanceDataBackup)
	mux.HandleFunc("/api/maintenance/data-backup/status", s.handleMaintenanceDataBackupStatus)
	mux.HandleFunc("/api/maintenance/data-backup/cancel", s.handleMaintenanceDataBackupCancel)
	mux.HandleFunc("/api/maintenance/storage-convert", s.handleMaintenanceStorageConvert)
	mux.HandleFunc("/api/maintenance/clean-logs", s.handleMaintenanceCleanLogs)
	mux.HandleFunc("/api/snapshot/status", s.handleSnapshotStatus)
	mux.HandleFunc("/api/snapshot/cancel", s.handleSnapshotCancel)
	mux.HandleFunc("/api/snapshot/reset", s.handleSnapshotReset)
	mux.HandleFunc("/api/snapshot/create", s.handleSnapshotCreate)
	mux.HandleFunc("/api/snapshot/restore/local", s.handleSnapshotRestoreLocal)
	mux.HandleFunc("/api/snapshot/restore/remote", s.handleSnapshotRestoreRemote)
	mux.HandleFunc("/api/snapshot/mock/", s.handleSnapshotMock)
	mux.HandleFunc("/api/system/updates/check", s.handleUpdatesCheck)
	mux.HandleFunc("/api/system/updates/apply", s.handleUpdatesApply)
	mux.HandleFunc("/api/system/updates/diff", s.handleConfigsDiff)
	mux.HandleFunc("/api/maintenance/update/check", s.handleUpdatesCheck)
	mux.HandleFunc("/api/maintenance/update/apply", s.handleUpdatesApply)
	mux.HandleFunc("/api/maintenance/update/diff", s.handleConfigsDiff)
	mux.HandleFunc("/api/maintenance/configs/diff", s.handleConfigsDiff)
	mux.HandleFunc("/api/engine/status", s.handleEngineStatus)
	mux.HandleFunc("/api/engine/manifest", s.handleEngineManifest)
	mux.HandleFunc("/api/engine/check", s.handleEngineCheck)
	mux.HandleFunc("/api/engine/apply", s.handleEngineApply)
	mux.HandleFunc("/api/engine/reset", s.handleEngineReset)
	mux.HandleFunc("/api/system/backup/export", s.handleBackupExport)
	mux.HandleFunc("/api/system/backup/restore", s.handleBackupRestore)
	mux.HandleFunc("/api/system/settings/export", s.handleBackupExport)
	mux.HandleFunc("/api/system/settings/restore", s.handleBackupRestore)
	mux.HandleFunc("/api/system/recovery/export", s.handleRecoveryExport)
	mux.HandleFunc("/api/system/recovery/restore", s.handleRecoveryRestore)
	mux.HandleFunc("/api/system/browse-dirs", s.handleSystemBrowseDirs)
	mux.HandleFunc("/api/system/native-pick-dir", s.handleNativePickDir)
	mux.HandleFunc("/api/system/disk-space", s.handleSystemDiskSpace)
	mux.HandleFunc("/api/system/shutdown", s.handleSystemShutdown)
	mux.HandleFunc("/api/console/query", s.handleConsoleQuery)
	mux.HandleFunc("/api/system/wsl/status", s.handleSystemWSLStatus)
	mux.HandleFunc("/api/system/wsl/install", s.handleSystemWSLInstall)
	mux.HandleFunc("/api/system/wsl/setup-distro", s.handleSystemWSLSetupDistro)

	// Storage & Replicator (DFMS) Routes
	mux.HandleFunc("/api/storage/status", s.handleStorageStatus)
	mux.HandleFunc("/api/storage/config", s.handleStorageConfig)
	mux.HandleFunc("/api/storage/key/generate", s.handleStorageKeyGenerate)
	mux.HandleFunc("/api/storage/sandboxes/clean", s.handleStorageSandboxesClean)
	mux.HandleFunc("/api/storage/peers", s.handleStoragePeers)
	mux.HandleFunc("/api/storage/onboard", s.handleStorageOnboard)

	// Static UI file server
	if s.staticFS != nil {
		fileServer := http.FileServer(http.FS(s.staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}

			f, err := s.staticFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
			if err != nil || r.URL.Path == "/" {
				indexContent, err := fs.ReadFile(s.staticFS, "index.html")
				if err == nil {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
					w.Header().Set("Pragma", "no-cache")
					w.Header().Set("Expires", "0")
					http.SetCookie(w, &http.Cookie{
						Name:     "sirius_token",
						Value:    s.apiToken,
						Path:     "/",
						SameSite: http.SameSiteStrictMode,
						HttpOnly: false,
					})
					w.Write(indexContent)
					return
				}
			}
			if f != nil {
				f.Close()
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return s.securityAndLoggingMiddleware(mux)
}

type statusLoggingResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (lrw *statusLoggingResponseWriter) WriteHeader(code int) {
	if !lrw.wroteHeader {
		lrw.statusCode = code
		lrw.wroteHeader = true
		lrw.ResponseWriter.WriteHeader(code)
	}
}

func (lrw *statusLoggingResponseWriter) Write(b []byte) (int, error) {
	if !lrw.wroteHeader {
		lrw.statusCode = http.StatusOK
		lrw.wroteHeader = true
	}
	return lrw.ResponseWriter.Write(b)
}

func (lrw *statusLoggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := lrw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

func (lrw *statusLoggingResponseWriter) Flush() {
	if fl, ok := lrw.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func (s *Server) securityAndLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &statusLoggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		defer func() {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				duration := time.Since(start)
				clientIP := r.RemoteAddr
				if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
					clientIP = host
				}
				log.Printf("[API Audit] %s | %s | %s %s | %d (%v)",
					time.Now().Format("2006-01-02 15:04:05"),
					clientIP,
					r.Method,
					r.URL.Path,
					lrw.statusCode,
					duration,
				)
			}
		}()

		// Set protective HTTP security headers
		lrw.Header().Set("X-Content-Type-Options", "nosniff")
		lrw.Header().Set("X-Frame-Options", "SAMEORIGIN")
		lrw.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		lrw.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' ws: wss: http: https:;")

		// Restrict CORS to verified localhost / local IP origins
		origin := r.Header.Get("Origin")
		if origin != "" {
			if u, err := url.Parse(origin); err == nil {
				h := u.Hostname()
				if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0" {
					lrw.Header().Set("Access-Control-Allow-Origin", origin)
					lrw.Header().Set("Vary", "Origin")
				}
			}
		}

		lrw.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		lrw.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Sirius-Token")

		if r.Method == "OPTIONS" {
			lrw.WriteHeader(http.StatusOK)
			return
		}

		// Security: Protections for state-changing requests on /api/
		isApi := strings.HasPrefix(r.URL.Path, "/api/")
		isStateChanging := r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH"

		if isApi && isStateChanging {
			// 1. CSRF Protection: Require valid Origin or Referer header matching localhost/127.0.0.1 (Fail closed)
			hasValidOrigin := false
			if origin != "" {
				if u, err := url.Parse(origin); err == nil {
					h := u.Hostname()
					if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0" {
						hasValidOrigin = true
					}
				}
			} else if referer := r.Header.Get("Referer"); referer != "" {
				if u, err := url.Parse(referer); err == nil {
					h := u.Hostname()
					if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0" {
						hasValidOrigin = true
					}
				}
			}

			if !hasValidOrigin {
				http.Error(lrw, `{"error":"Forbidden: missing or invalid Origin/Referer header"}`, http.StatusForbidden)
				return
			}

			// 2. CSRF Simple Request Blocker: Enforce Content-Type application/json
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				http.Error(lrw, `{"error":"Unsupported Media Type: application/json required"}`, http.StatusUnsupportedMediaType)
				return
			}

			// 3. API Token Authentication: Required unconditionally for state-changing requests
			clientToken := r.Header.Get("X-Sirius-Token")
			if clientToken == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					clientToken = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}
			if clientToken == "" {
				if cookie, err := r.Cookie("sirius_token"); err == nil {
					clientToken = cookie.Value
				}
			}

			if subtle.ConstantTimeCompare([]byte(clientToken), []byte(s.apiToken)) != 1 {
				http.Error(lrw, `{"error":"Unauthorized: Invalid or missing API token"}`, http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(lrw, r)
	})
}

func (s *Server) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Defense-in-depth: Reject cross-site callers via Sec-Fetch-Site
	secFetchSite := r.Header.Get("Sec-Fetch-Site")
	if secFetchSite == "cross-site" {
		http.Error(w, `{"error":"Forbidden: cross-site access rejected"}`, http.StatusForbidden)
		return
	}

	// Validate Origin / Referer if present
	origin := r.Header.Get("Origin")
	if origin != "" {
		if u, err := url.Parse(origin); err != nil || !(u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" || u.Hostname() == "0.0.0.0") {
			http.Error(w, `{"error":"Forbidden: cross-origin access rejected"}`, http.StatusForbidden)
			return
		}
	} else if referer := r.Header.Get("Referer"); referer != "" {
		if u, err := url.Parse(referer); err != nil || !(u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" || u.Hostname() == "0.0.0.0") {
			http.Error(w, `{"error":"Forbidden: cross-origin access rejected"}`, http.StatusForbidden)
			return
		}
	}

	jsonResponse(w, map[string]string{
		"token": s.apiToken,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	dataPath := s.configMgr.GetDataPath()
	metrics, err := s.supervisor.GetMetrics(dataPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	netHeight, _ := s.chainMon.GetNetworkHeight()
	metrics.NetworkHeight = netHeight

	localHeight, _ := s.chainMon.GetLocalHeight(dataPath)
	logHeight := s.supervisor.GetLatestLogHeight()
	if logHeight > localHeight {
		metrics.BlockHeight = logHeight
	} else {
		metrics.BlockHeight = localHeight
	}

	peerCount := s.supervisor.GetConnectedPeersCount()
	if peerCount == 0 && metrics.Status == "running" {
		peers, _ := s.chainMon.GetPeers()
		peerCount = len(peers)
	}
	metrics.PeersCount = peerCount

	cfg, _ := s.configMgr.LoadNodeConfig()

	resp := map[string]interface{}{
		"status":                metrics.Status,
		"blockHeight":           metrics.BlockHeight,
		"networkHeight":         metrics.NetworkHeight,
		"peersCount":            metrics.PeersCount,
		"metrics":               metrics,
		"config":                cfg,
		"harvestStats":          s.harvesterTracker.GetStats(),
		"networkValidatorStats": s.harvesterTracker.GetNetworkValidatorStats(),
		"storageStatus":         s.storageMgr.GetStatus(),
		"portCheck":             s.networkMgr.GetLastResult(),
		"autoRecovery":          s.supervisor.IsAutoRecoveryEnabled(),
		"updateInfo":            s.updateMgr.GetUpdateInfo(),
		"engineStatus":          s.engineUpdater.GetStatus(),
		"wslStatus":             s.supervisor.ProbeWSLStatus(),
	}

	jsonResponse(w, resp)
}

func (s *Server) handleNetworkValidatorStats(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, s.harvesterTracker.GetNetworkValidatorStats())
}

func (s *Server) handleHarvestStats(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, s.harvesterTracker.GetStats())
}

func (s *Server) handleHarvestCheckNow(w http.ResponseWriter, r *http.Request) {
	s.harvesterTracker.TriggerCheck(s.chainMon)
	jsonResponse(w, map[string]string{"status": "ok", "message": "Manual check triggered"})
}

func (s *Server) handleHarvestReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target := r.URL.Query().Get("target")
	if target == "" && r.Body != nil {
		var req struct {
			Target string `json:"target"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		target = req.Target
	}

	stats := s.harvesterTracker.ResetStats(target)
	jsonResponse(w, map[string]interface{}{
		"status":  "ok",
		"target":  target,
		"message": "Harvesting metrics reset successfully",
		"stats":   stats,
	})
}

func (s *Server) handleNodeStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := s.configMgr.LoadNodeConfig()
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed to load node configuration: %v", err), http.StatusInternalServerError)
		return
	}

	if !cfg.HasHarvestKey {
		jsonError(w, "Cannot start node: a valid 64-hex harvest key is mandatory before starting the node", http.StatusBadRequest)
		return
	}

	if err := s.supervisor.StartNode(s.configMgr.GetDataPath()); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]string{"status": "starting", "message": "Sirius Chain node started successfully"})
}

func (s *Server) handleNodeStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.supervisor.StopNode(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "stopped", "message": "Sirius Chain node stopped successfully"})
}

func (s *Server) handleNodeRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := s.configMgr.LoadNodeConfig()
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed to load node configuration: %v", err), http.StatusInternalServerError)
		return
	}

	if !cfg.HasHarvestKey {
		jsonError(w, "Cannot restart node: a valid 64-hex harvest key is mandatory before starting the node", http.StatusBadRequest)
		return
	}

	if err := s.supervisor.RestartNode(s.configMgr.GetDataPath()); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "restarting", "message": "Sirius Chain node restarted"})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	_, logs := s.supervisor.SubscribeLogs()
	jsonResponse(w, map[string]interface{}{
		"logs": logs,
	})
}

func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	// Security: Cap concurrent WebSocket log streams (max 5)
	s.wsMutex.Lock()
	if s.activeWsConns >= 5 {
		s.wsMutex.Unlock()
		http.Error(w, "Too many concurrent WebSocket log streams (max 5)", http.StatusTooManyRequests)
		return
	}
	s.activeWsConns++
	s.wsMutex.Unlock()

	defer func() {
		s.wsMutex.Lock()
		s.activeWsConns--
		s.wsMutex.Unlock()
	}()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ch, initialLogs := s.supervisor.SubscribeLogs()
	defer s.supervisor.UnsubscribeLogs(ch)

	for _, l := range initialLogs {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(l)); err != nil {
			return
		}
	}

	for line := range ch {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
			break
		}
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg, err := s.configMgr.LoadNodeConfig()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Security: Never return raw private keys over any API endpoint
		cfg.BootKey = ""
		cfg.HarvestKey = ""
		jsonResponse(w, cfg)
		return
	}

	if r.Method == "POST" {
		var cfg config.NodeConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			jsonError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := s.configMgr.SaveNodeConfig(&cfg); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if savedCfg, err := s.configMgr.LoadNodeConfig(); err == nil && savedCfg.HarvestPublicKey != "" {
			s.harvesterTracker.SetHarvestPublicKey(savedCfg.HarvestPublicKey)
		}

		jsonResponse(w, map[string]string{"status": "success", "message": "Configuration saved successfully"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleConfigRaw(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")

	if r.Method == "GET" {
		if filename == "" {
			jsonError(w, "file query parameter required", http.StatusBadRequest)
			return
		}
		content, err := s.configMgr.GetRawConfigFile(filename)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonResponse(w, map[string]string{"filename": filename, "content": content})
		return
	}

	if r.Method == "POST" {
		var payload struct {
			File    string `json:"file"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			jsonError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if filename == "" {
			filename = strings.TrimSpace(payload.File)
		}
		if filename == "" {
			jsonError(w, "file parameter required in query or request body", http.StatusBadRequest)
			return
		}

		if err := s.configMgr.SaveRawConfigFile(filename, payload.Content); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsonResponse(w, map[string]string{"status": "success", "message": "File saved successfully"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}


func (s *Server) handleConfigFilesList(w http.ResponseWriter, r *http.Request) {
	files, err := s.configMgr.ListConfigFiles()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, files)
}

func (s *Server) handleKeyGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, keyPair)
}

func (s *Server) handleKeyParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PrivateKey string `json:"privateKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	keyPair, err := crypto.KeyPairFromPrivateKey(req.PrivateKey)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, keyPair)
}

func (s *Server) handleHarvestingLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AccountPrivateKey crypto.SecretKeyBuffer `json:"accountPrivateKey"`
		RemoteHarvestKey  string                 `json:"remoteHarvestKey"`
		ApiNode           string                 `json:"apiNode"`
		Action            string                 `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Invariant: Explicitly zero the secret key buffer upon exit
	defer req.AccountPrivateKey.Wipe()

	result, err := crypto.PerformDelegatedHarvestingLink(req.AccountPrivateKey, req.RemoteHarvestKey, req.ApiNode, req.Action)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := s.configMgr.LoadNodeConfig()
	if err == nil {
		if result.RemotePrivateKey != "" {
			cfg.HarvestKey = result.RemotePrivateKey
			if cfg.BootKeySource == "harvest" || cfg.BootKeySource == "" {
				cfg.BootKey = result.RemotePrivateKey
			}
			if err := s.configMgr.SaveNodeConfig(cfg); err != nil {
				log.Printf("[Harvesting] Error saving harvestKey to config: %v", err)
			} else {
				log.Printf("[Harvesting] Successfully saved new harvestKey to config-harvesting.properties (Remote PubKey: %s)", result.RemotePublicKey)
			}
		}
	} else {
		log.Printf("[Harvesting] Error loading config: %v", err)
	}

	jsonResponse(w, result)
}

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	peers := s.supervisor.GetConnectedPeers()
	if len(peers) == 0 {
		cmPeers, err := s.chainMon.GetPeers()
		if err == nil && len(cmPeers) > 0 {
			jsonResponse(w, cmPeers)
			return
		}
	}
	jsonResponse(w, peers)
}

func (s *Server) handleMaintenanceReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.supervisor.ResetChain(s.configMgr.GetDataPath()); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "success", "message": "Chain data successfully reset to genesis nemesis block"})
}

func (s *Server) handleMaintenanceSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Mode       string `json:"mode"`
		Url        string `json:"url"`
		SourcePath string `json:"sourcePath"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	sourcePath := strings.TrimSpace(req.SourcePath)
	targetUrl := strings.TrimSpace(req.Url)

	if mode == "local" || (sourcePath != "" && mode != "remote") {
		if sourcePath == "" {
			jsonError(w, "Local snapshot archive path cannot be empty", http.StatusBadRequest)
			return
		}
		go func() {
			_ = s.supervisor.RestoreLocalSnapshot(sourcePath, s.configMgr.GetDataPath())
		}()
		jsonResponse(w, map[string]string{"status": "in_progress", "message": "Local snapshot archive extraction initiated."})
		return
	}

	if targetUrl != "" {
		if !strings.HasPrefix(targetUrl, "http://") && !strings.HasPrefix(targetUrl, "https://") {
			jsonError(w, "Invalid snapshot URL protocol. Must use http:// or https://", http.StatusBadRequest)
			return
		}
	}

	go func() {
		_ = s.supervisor.RestoreSnapshot(targetUrl, s.configMgr.GetDataPath())
	}()

	jsonResponse(w, map[string]string{"status": "in_progress", "message": "Remote fast-sync snapshot stream initiated."})
}

func (s *Server) handleMaintenanceSnapshotCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.supervisor.CancelSnapshot(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "cancelled", "message": "Snapshot process stopped successfully."})
}

func (s *Server) handleMaintenanceSnapshotStatus(w http.ResponseWriter, r *http.Request) {
	status := s.supervisor.GetSnapshotStatus()
	jsonResponse(w, status)
}

func (s *Server) handleMaintenanceDataBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SourcePath string `json:"sourcePath"`
		TargetPath string `json:"targetPath"`
		Format     string `json:"format"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	srcPath := strings.TrimSpace(req.SourcePath)
	if srcPath == "" {
		srcPath = s.configMgr.GetDataPath()
	}

	targetPath := strings.TrimSpace(req.TargetPath)
	format := strings.TrimSpace(req.Format)

	if err := s.supervisor.CreateDataBackup(srcPath, targetPath, format); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"status":  "in_progress",
		"message": "Blockchain data backup started in background.",
	})
}

func (s *Server) handleMaintenanceDataBackupStatus(w http.ResponseWriter, r *http.Request) {
	status := s.supervisor.GetDataBackupStatus()
	jsonResponse(w, status)
}

func (s *Server) handleMaintenanceDataBackupCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.supervisor.CancelDataBackup(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "ok",
		"message": "Data backup cancelled.",
	})
}

func (s *Server) handleMaintenanceStorageConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DataPath string `json:"dataPath"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	targetPath := strings.TrimSpace(req.DataPath)
	if targetPath == "" {
		targetPath = s.configMgr.GetDataPath()
	}

	// Stop node first to ensure no file lock conflicts during migration
	_ = s.supervisor.StopNode()

	if err := s.migrator.StartMigration(targetPath); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "started",
		"message": fmt.Sprintf("Storage migration initiated for %s", targetPath),
	})
}

func (s *Server) handleMaintenanceStorageConvertStatus(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, s.migrator.GetStatus())
}

func (s *Server) handleMaintenanceStorageConvertCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.migrator.Cancel()
	jsonResponse(w, map[string]string{"status": "cancelled", "message": "Storage migration cancel requested."})
}

func (s *Server) handleMaintenanceCleanLogs(w http.ResponseWriter, r *http.Request) {
	dataPath := s.configMgr.GetDataPath()

	if r.Method == http.MethodGet {
		stats := s.supervisor.GetLogStats(dataPath)
		jsonResponse(w, stats)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	freedBytes, err := s.supervisor.CleanLogsAndCache(dataPath)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	freedMB := float64(freedBytes) / (1024 * 1024)
	stats := s.supervisor.GetLogStats(dataPath)
	jsonResponse(w, map[string]interface{}{
		"status":          "success",
		"freedBytes":      freedBytes,
		"freedMB":         freedMB,
		"message":         "Successfully cleaned logs and temporary cache.",
		"logCount":        stats.LogCount,
		"totalMB":         stats.TotalMB,
		"serverLockFound": stats.ServerLockFound,
		"logsDir":         stats.LogsDir,
		"lastPurgeTime":   stats.LastPurgeTime,
	})
}

func (s *Server) handleConsoleQuery(w http.ResponseWriter, r *http.Request) {
	endpoint := strings.TrimSpace(r.URL.Query().Get("endpoint"))
	if endpoint == "" {
		endpoint = "/chain/height"
	}

	// Security validation: Prevent SSRF and schema injection
	if strings.Contains(endpoint, "://") || strings.Contains(endpoint, "@") || strings.Contains(endpoint, "..") || !strings.HasPrefix(endpoint, "/") {
		jsonError(w, "Invalid endpoint path. Must be a relative Sirius REST path starting with /", http.StatusBadRequest)
		return
	}

	data, err := s.chainMon.ExecuteRestQuery(endpoint)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, data)
}

func (s *Server) handlePublicIp(w http.ResponseWriter, r *http.Request) {
	ip, err := s.chainMon.GetPublicIP()
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	jsonResponse(w, map[string]string{"publicIp": ip})
}

func (s *Server) handlePeerJson(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.configMgr.LoadNodeConfig()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	host := cfg.Host
	if host == "" {
		ip, err := s.chainMon.GetPublicIP()
		if err == nil && ip != "" {
			host = ip
		} else {
			host = "YOUR_NODE_PUBLIC_IP"
		}
	}

	name := cfg.FriendlyName
	if name == "" {
		name = "sirius-mainnet-peer"
	}

	peerCard := map[string]interface{}{
		"publicKey": cfg.BootPublicKey,
		"endpoint": map[string]interface{}{
			"host":     host,
			"port":     cfg.Port,
			"dbrbPort": cfg.DbrbPort,
		},
		"metadata": map[string]interface{}{
			"name":  name,
			"roles": "Peer",
		},
	}

	jsonResponse(w, peerCard)
}

func (s *Server) handleHarvestingCheck(w http.ResponseWriter, r *http.Request) {
	account := r.URL.Query().Get("account")
	apiNode := r.URL.Query().Get("apiNode")

	if account == "" {
		cfg, err := s.configMgr.LoadNodeConfig()
		if err == nil {
			account = cfg.HarvestPublicKey
			if account == "" {
				account = cfg.HarvestAddress
			}
		}
	}

	if account == "" {
		jsonError(w, "account or harvest public key required", http.StatusBadRequest)
		return
	}

	status, err := s.chainMon.CheckHarvesterStatus(account, apiNode)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, status)
}

func (s *Server) handleSystemBrowseDirs(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if reqPath == "" {
		reqPath = s.configMgr.GetDataPath()
		if reqPath == "" {
			reqPath = "./chainconfig/data"
		}
	}

	// Resolve target directory
	targetDir := reqPath
	if !filepath.IsAbs(targetDir) {
		abs, err := filepath.Abs(targetDir)
		if err == nil {
			targetDir = abs
		}
	}

	// If directory doesn't exist yet, try parent or current working directory
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		parent := filepath.Dir(targetDir)
		if _, errP := os.Stat(parent); errP == nil {
			targetDir = parent
		} else {
			targetDir, _ = os.Getwd()
		}
	}

	// Security filter: Prevent browsing sensitive system directories
	cleanTarget := filepath.Clean(targetDir)
	sep := string(filepath.Separator)
	forbiddenList := []string{
		"/etc", "/proc", "/sys", "/dev", "/root", "/var/log", "/private/etc", "/private/var",
	}
	if runtime.GOOS == "windows" {
		winDir := os.Getenv("SystemRoot")
		if winDir == "" {
			winDir = "C:\\Windows"
		}
		forbiddenList = append(forbiddenList,
			filepath.Join(winDir, "System32", "config"),
			"C:\\System Volume Information",
			"C:\\$Recycle.Bin",
		)
	}
	for _, forbidden := range forbiddenList {
		fClean := filepath.Clean(forbidden)
		if cleanTarget == fClean || strings.HasPrefix(cleanTarget, fClean+sep) {
			targetDir, _ = os.Getwd()
			break
		}
	}

	type DirItem struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		IsDir     bool   `json:"isDir"`
		SizeHuman string `json:"sizeHuman,omitempty"`
	}

	includeFiles := r.URL.Query().Get("includeFiles") == "true" || r.URL.Query().Get("mode") == "file"

	var items []DirItem
	entries, err := os.ReadDir(targetDir)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			fullPath := filepath.Join(targetDir, name)
			if e.IsDir() {
				items = append(items, DirItem{
					Name:  name,
					Path:  fullPath,
					IsDir: true,
				})
			} else if includeFiles {
				nameLower := strings.ToLower(name)
				if strings.Contains(nameLower, ".tar") || strings.Contains(nameLower, ".zst") || strings.Contains(nameLower, ".gz") || strings.Contains(nameLower, ".xz") || strings.Contains(nameLower, ".json") {
					var sizeStr string
					if info, errI := e.Info(); errI == nil {
						sz := info.Size()
						if sz >= 1024*1024*1024 {
							sizeStr = fmt.Sprintf("%.2f GB", float64(sz)/(1024*1024*1024))
						} else if sz >= 1024*1024 {
							sizeStr = fmt.Sprintf("%.1f MB", float64(sz)/(1024*1024))
						} else {
							sizeStr = fmt.Sprintf("%d KB", sz/1024)
						}
					}
					items = append(items, DirItem{
						Name:      name,
						Path:      fullPath,
						IsDir:     false,
						SizeHuman: sizeStr,
					})
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir // directories first
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	parentPath := filepath.Dir(targetDir)
	if parentPath == targetDir {
		parentPath = ""
	}

	// Common presets & Auto-detected snapshot archives
	var presets []DirItem
	if includeFiles {
		homeDir, _ := os.UserHomeDir()
		searchDirs := []string{homeDir, filepath.Join(homeDir, "Downloads"), "/Volumes/SSD"}
		for _, sDir := range searchDirs {
			if sEntries, errS := os.ReadDir(sDir); errS == nil {
				for _, se := range sEntries {
					if !se.IsDir() {
						sName := se.Name()
						sLower := strings.ToLower(sName)
						if strings.Contains(sLower, "snapshot") || strings.Contains(sLower, "sirius-data-backup") || (strings.HasPrefix(sLower, "sirius-") && strings.Contains(sLower, ".tar")) {
							var szHuman string
							if si, errSi := se.Info(); errSi == nil {
								sz := si.Size()
								if sz >= 1024*1024*1024 {
									szHuman = fmt.Sprintf(" (%.2f GB)", float64(sz)/(1024*1024*1024))
								} else if sz >= 1024*1024 {
									szHuman = fmt.Sprintf(" (%.1f MB)", float64(sz)/(1024*1024))
								}
							}
							presets = append(presets, DirItem{
								Name:      fmt.Sprintf("Snapshot Archive: %s%s", sName, szHuman),
								Path:      filepath.Join(sDir, sName),
								IsDir:     false,
								SizeHuman: szHuman,
							})
						}
					}
				}
			}
		}
	}

	presets = append(presets, DirItem{Name: "Default Project Data (./chainconfig/data)", Path: "./chainconfig/data", IsDir: true})

	// Cross-platform external storage presets:
	// 1. macOS (/Volumes)
	// 2. Linux (/media, /mnt)
	// 3. Windows (Drive letters C:\ through Z:\)
	switch runtime.GOOS {
	case "darwin":
		if vEntries, errV := os.ReadDir("/Volumes"); errV == nil {
			for _, v := range vEntries {
				name := v.Name()
				if strings.HasPrefix(name, ".") || strings.EqualFold(name, "com.apple.TimeMachine.localsnapshots") {
					continue
				}
				fullVolPath := filepath.Join("/Volumes", name)
				if fi, errFi := os.Stat(fullVolPath); errFi == nil && fi.IsDir() {
					presets = append(presets, DirItem{
						Name:  fmt.Sprintf("External Drive / Volume: /Volumes/%s", name),
						Path:  filepath.Join(fullVolPath, "Sirius_data"),
						IsDir: true,
					})
				}
			}
			presets = append(presets, DirItem{Name: "Browse /Volumes Directory", Path: "/Volumes", IsDir: true})
		}
	case "linux":
		for _, mountBase := range []string{"/media", "/mnt"} {
			if mEntries, errM := os.ReadDir(mountBase); errM == nil {
				for _, m := range mEntries {
					name := m.Name()
					if strings.HasPrefix(name, ".") {
						continue
					}
					fullMPath := filepath.Join(mountBase, name)
					if fi, errFi := os.Stat(fullMPath); errFi == nil && fi.IsDir() {
						displayName := fmt.Sprintf("Mounted Storage (%s): %s", mountBase, name)
						if mountBase == "/mnt" && len(name) == 1 {
							displayName = fmt.Sprintf("Drive %s: (WSL %s)", strings.ToUpper(name), fullMPath)
						}
						presets = append(presets, DirItem{
							Name:  displayName,
							Path:  filepath.Join(fullMPath, "Sirius_data"),
							IsDir: true,
						})
					}
				}
				presets = append(presets, DirItem{Name: fmt.Sprintf("Browse %s Directory", mountBase), Path: mountBase, IsDir: true})
			}
		}
	case "windows":
		for r := 'C'; r <= 'Z'; r++ {
			driveRoot := fmt.Sprintf("%c:\\", r)
			if _, errD := os.Stat(driveRoot); errD == nil {
				presets = append(presets, DirItem{
					Name:  fmt.Sprintf("Drive %c: Root (%s)", r, driveRoot),
					Path:  driveRoot,
					IsDir: true,
				})
				presets = append(presets, DirItem{
					Name:  fmt.Sprintf("Drive %c: (%sSirius_data)", r, driveRoot),
					Path:  filepath.Join(driveRoot, "Sirius_data"),
					IsDir: true,
				})
			}
		}
	}

	jsonResponse(w, map[string]interface{}{
		"currentPath": targetDir,
		"parentPath":  parentPath,
		"directories": items,
		"presets":     presets,
	})
}

const winPickerScript = `param(
	[string]$dlgTitle = "Select Sirius Blockchain Data Directory",
	[string]$dlgMode = "dir"
)

Add-Type -TypeDefinition @"
using System;
using System.Windows.Forms;
using System.Runtime.InteropServices;

public class Win32Picker {
    [DllImport("user32.dll")]
    public static extern IntPtr GetForegroundWindow();

    [DllImport("user32.dll")]
    public static extern bool SetForegroundWindow(IntPtr hWnd);

    [ComImport, Guid("DC1C5A9C-E88A-4dde-A5A1-60F82A20AEF7")]
    [ClassInterface(ClassInterfaceType.None)]
    private class FileOpenDialogRc {}

    [ComImport, Guid("d57c7288-d4ad-4768-be02-9d969532d960"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    private interface IFileOpenDialog {
        [PreserveSig] int Show(IntPtr parent);
        void SetFileTypes();
        void SetFileTypeIndex();
        void GetFileTypeIndex();
        void Advise();
        void Unadvise();
        void SetOptions(uint fos);
        void GetOptions(out uint fos);
        void SetDefaultFolder();
        void SetFolder(IntPtr psi);
        void GetFolder();
        void GetCurrentSelection();
        void SetFileName([MarshalAs(UnmanagedType.LPWStr)] string pszName);
        void GetFileName();
        void SetTitle([MarshalAs(UnmanagedType.LPWStr)] string pszTitle);
        void SetOkButtonLabel([MarshalAs(UnmanagedType.LPWStr)] string pszText);
        void SetFileNameLabel([MarshalAs(UnmanagedType.LPWStr)] string pszLabel);
        void GetResult(out IntPtr ppsi);
    }

    [ComImport, Guid("43826d1e-e718-42ee-bc55-a1e261c37bfe"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    private interface IShellItem {
        void BindToHandler();
        void GetParent();
        void GetDisplayName(uint sigdnName, [MarshalAs(UnmanagedType.LPWStr)] out string ppszName);
        void GetAttributes();
        void Compare();
    }

    public static string PickFolder(string title, IntPtr hwnd) {
        var dialog = (IFileOpenDialog)new FileOpenDialogRc();
        uint options;
        dialog.GetOptions(out options);
        dialog.SetOptions(options | 0x00000020 | 0x00000040);
        if (!string.IsNullOrEmpty(title)) {
            dialog.SetTitle(title);
        }
        int hr = dialog.Show(hwnd);
        if (hr == unchecked((int)0x800704C7)) {
            return null;
        }
        if (hr != 0 && hwnd != IntPtr.Zero) {
            hr = dialog.Show(IntPtr.Zero);
            if (hr == unchecked((int)0x800704C7)) {
                return null;
            }
        }
        if (hr == 0) {
            IntPtr ppsi;
            dialog.GetResult(out ppsi);
            var item = (IShellItem)Marshal.GetObjectForIUnknown(ppsi);
            string path;
            item.GetDisplayName(0x80028000, out path);
            Marshal.Release(ppsi);
            return path;
        }
        return null;
    }

    public static string PickFile(string title, IntPtr hwnd) {
        var dialog = (IFileOpenDialog)new FileOpenDialogRc();
        uint options;
        dialog.GetOptions(out options);
        dialog.SetOptions(options | 0x00000040 | 0x00001000);
        if (!string.IsNullOrEmpty(title)) {
            dialog.SetTitle(title);
        }
        int hr = dialog.Show(hwnd);
        if (hr == unchecked((int)0x800704C7)) {
            return null;
        }
        if (hr != 0 && hwnd != IntPtr.Zero) {
            hr = dialog.Show(IntPtr.Zero);
            if (hr == unchecked((int)0x800704C7)) {
                return null;
            }
        }
        if (hr == 0) {
            IntPtr ppsi;
            dialog.GetResult(out ppsi);
            var item = (IShellItem)Marshal.GetObjectForIUnknown(ppsi);
            string path;
            item.GetDisplayName(0x80028000, out path);
            Marshal.Release(ppsi);
            return path;
        }
        return null;
    }
}
"@ -ReferencedAssemblies "System.Windows.Forms" -ErrorAction SilentlyContinue

Add-Type -AssemblyName System.Windows.Forms
$owner = New-Object System.Windows.Forms.Form
$owner.TopMost = $true
$owner.StartPosition = [System.Windows.Forms.FormStartPosition]::CenterScreen
$owner.ShowInTaskbar = $false
$owner.Opacity = 0
$owner.Show()

try {
	if ($dlgMode -eq 'file') {
		$res = [Win32Picker]::PickFile($dlgTitle, $owner.Handle)
	} else {
		$res = [Win32Picker]::PickFolder($dlgTitle, $owner.Handle)
	}
	if ($res) {
		[Console]::Out.Write($res)
		exit 0
	}
} catch {
	if ($dlgMode -eq 'file') {
		$f = New-Object System.Windows.Forms.OpenFileDialog
		$f.Title = $dlgTitle
		if ($f.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) {
			[Console]::Out.Write($f.FileName)
		}
	} else {
		$f = New-Object System.Windows.Forms.FolderBrowserDialog
		$f.Description = $dlgTitle
		$f.RootFolder = [System.Environment+SpecialFolder]::MyComputer
		if ($f.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK) {
			[Console]::Out.Write($f.SelectedPath)
		}
	}
} finally {
	$owner.Dispose()
}
`

func (s *Server) handleNativePickDir(w http.ResponseWriter, r *http.Request) {
	if !s.nativePickerMu.TryLock() {
		jsonResponse(w, map[string]interface{}{
			"success":  false,
			"canceled": true,
			"error":    "Picker dialog already open",
		})
		return
	}
	defer s.nativePickerMu.Unlock()

	var selectedPath string
	var err error

	mode := r.URL.Query().Get("mode") // "file" or "dir" (default)
	prompt := r.URL.Query().Get("prompt")
	if prompt == "" {
		if mode == "file" {
			prompt = "Select Sirius File"
		} else {
			prompt = "Select Sirius Blockchain Data Directory"
		}
	}
	// Strict allowlist: only alphanumeric, spaces, hyphens, underscores, dots
	prompt = strings.Map(func(rn rune) rune {
		if (rn >= 'a' && rn <= 'z') || (rn >= 'A' && rn <= 'Z') || (rn >= '0' && rn <= '9') || rn == ' ' || rn == '-' || rn == '_' || rn == '.' {
			return rn
		}
		return -1
	}, prompt)
	if len(prompt) > 80 {
		prompt = prompt[:80]
	}

	switch runtime.GOOS {
	case "darwin":
		var script string
		if mode == "file" {
			script = "on run argv\nPOSIX path of (choose file with prompt (item 1 of argv))\nend run"
		} else {
			script = "on run argv\nPOSIX path of (choose folder with prompt (item 1 of argv))\nend run"
		}
		cmd := exec.Command("osascript", "-e", script, "--", prompt)
		out, e := cmd.Output()
		if e == nil {
			selectedPath = strings.TrimSpace(string(out))
		} else {
			err = e
		}
	case "windows":
		chainConfigDir := filepath.Dir(s.configMgr.GetResourcesPath())
		scriptPath := filepath.Join(chainConfigDir, "..", "scripts", "packaging", "windows", "pick-directory.ps1")
		if _, eStat := os.Stat(scriptPath); eStat != nil {
			if execPath, e2 := os.Executable(); e2 == nil {
				scriptPath = filepath.Join(filepath.Dir(execPath), "scripts", "packaging", "windows", "pick-directory.ps1")
			}
			if _, e3 := os.Stat(scriptPath); e3 != nil {
				scriptPath = filepath.Join(os.TempDir(), "sirius-pick-directory.ps1")
				_ = os.WriteFile(scriptPath, []byte(winPickerScript), 0644)
			}
		}
		cmd := exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-NoProfile", "-Sta", "-File", scriptPath, prompt, mode)
		out, e := cmd.Output()
		if e == nil {
			selectedPath = strings.TrimSpace(string(out))
		} else {
			err = e
		}
	case "linux":
		// WSL detection: if running under WSL, powershell.exe opens native Windows Explorer dialog
		if _, e := exec.LookPath("powershell.exe"); e == nil {
			chainConfigDir := filepath.Dir(s.configMgr.GetResourcesPath())
			scriptPath := filepath.Join(chainConfigDir, "..", "scripts", "packaging", "windows", "pick-directory.ps1")
			winScriptPath := scriptPath
			if wslOut, wErr := exec.Command("wslpath", "-w", scriptPath).Output(); wErr == nil {
				winScriptPath = strings.TrimSpace(string(wslOut))
			}
			cmd := exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-NoProfile", "-Sta", "-File", winScriptPath, prompt, mode)
			out, e := cmd.Output()
			if e == nil {
				winPath := strings.TrimSpace(string(out))
				if winPath != "" {
					if wslOut, wErr := exec.Command("wslpath", "-u", winPath).Output(); wErr == nil {
						selectedPath = strings.TrimSpace(string(wslOut))
					} else {
						selectedPath = winPath
					}
				}
			} else {
				err = e
			}
		} else if _, e := exec.LookPath("zenity"); e == nil {
			args := []string{"--file-selection", "--title", prompt}
			if mode != "file" {
				args = append(args, "--directory")
			}
			cmd := exec.Command("zenity", args...)
			out, e := cmd.Output()
			if e == nil {
				selectedPath = strings.TrimSpace(string(out))
			}
		} else if _, e := exec.LookPath("kdialog"); e == nil {
			flag := "--getexistingdirectory"
			if mode == "file" {
				flag = "--getopenfilename"
			}
			cmd := exec.Command("kdialog", flag, "--title", prompt)
			out, e := cmd.Output()
			if e == nil {
				selectedPath = strings.TrimSpace(string(out))
			}
		}
	}

	if selectedPath != "" {
		// Clean trailing slash if present (except root '/')
		cleanPath := selectedPath
		if len(cleanPath) > 1 && !strings.HasSuffix(cleanPath, `:\`) {
			cleanPath = strings.TrimRight(cleanPath, "/\\")
		}
		jsonResponse(w, map[string]interface{}{
			"success": true,
			"path":    cleanPath,
		})
		return
	}

	jsonResponse(w, map[string]interface{}{
		"success":  false,
		"canceled": true,
		"error":    fmt.Sprintf("%v", err),
	})
}

func (s *Server) handleSystemDiskSpace(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if reqPath == "" {
		reqPath = s.configMgr.GetDataPath()
	}
	info := s.supervisor.GetDiskSpace(reqPath)
	jsonResponse(w, info)
}

func (s *Server) handleWatchdogToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.supervisor.SetAutoRecovery(req.Enabled)
	jsonResponse(w, map[string]interface{}{
		"status":       "ok",
		"autoRecovery": s.supervisor.IsAutoRecoveryEnabled(),
		"message":      fmt.Sprintf("Auto-recovery watchdog set to %v", req.Enabled),
	})
}

func (s *Server) handleUpdatesCheck(w http.ResponseWriter, r *http.Request) {
	info, err := s.updateMgr.CheckUpdate()
	if err != nil {
		jsonResponse(w, info)
		return
	}
	jsonResponse(w, info)
}

func (s *Server) handleUpdatesApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dataPath := s.configMgr.GetDataPath()
	go func() {
		_ = s.updateMgr.ApplyOfficialUpdate(dataPath)
	}()

	jsonResponse(w, map[string]string{
		"status":  "ok",
		"message": "Official node update process started in background.",
	})
}

func (s *Server) handleConfigsDiff(w http.ResponseWriter, r *http.Request) {
	diff, err := s.updateMgr.CheckConfigsDiff()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, diff)
}

func (s *Server) handleEngineStatus(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, s.engineUpdater.GetStatus())
}

func (s *Server) handleEngineManifest(w http.ResponseWriter, r *http.Request) {
	manifest, err := s.engineUpdater.LoadManifest()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, manifest)
}

func (s *Server) handleEngineCheck(w http.ResponseWriter, r *http.Request) {
	simulate := r.URL.Query().Get("simulate")
	if r.Method == http.MethodPost && r.Body != nil {
		var req struct {
			Simulate string `json:"simulate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Simulate != "" {
			simulate = req.Simulate
		}
	}
	status, err := s.engineUpdater.CheckUpdate(simulate)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonResponse(w, status)
}

func (s *Server) handleEngineApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Version  string `json:"version"`
		Scenario string `json:"scenario"` // "normal", "rollback", "signature_fail"
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	targetVersion := req.Version
	if targetVersion == "" {
		curStatus := s.engineUpdater.GetStatus()
		targetVersion = curStatus.TargetVersion
		if targetVersion == "" {
			targetVersion = "v1.9.8"
		}
	}

	dataPath := s.configMgr.GetDataPath()
	if req.Scenario == "rollback" || req.Scenario == "signature_fail" {
		s.engineUpdater.TriggerWorkflow(targetVersion, req.Scenario, dataPath)
	} else {
		go func() {
			if err := s.engineUpdater.DownloadAndApplyUpdate(targetVersion, dataPath); err != nil {
				log.Printf("[EngineUpdater] Update execution failed: %v", err)
			}
		}()
	}

	jsonResponse(w, map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Engine update to %s initiated (scenario: %s)", targetVersion, req.Scenario),
	})
}

func (s *Server) handleEngineReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.engineUpdater.ResetStatus()
	jsonResponse(w, s.engineUpdater.GetStatus())
}

func (s *Server) handleBackupExport(w http.ResponseWriter, r *http.Request) {
	// Authentication check for sensitive data export
	clientToken := r.Header.Get("X-Sirius-Token")
	if clientToken == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			clientToken = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if clientToken == "" {
		if cookie, err := r.Cookie("sirius_token"); err == nil {
			clientToken = cookie.Value
		}
	}
	if subtle.ConstantTimeCompare([]byte(clientToken), []byte(s.apiToken)) != 1 {
		http.Error(w, `{"error":"Unauthorized: Invalid or missing API token"}`, http.StatusUnauthorized)
		return
	}

	backup, err := s.configMgr.ExportBackup()
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed to export backup: %v", err), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("sirius-node-backup-%s.json", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(backup)
}

func (s *Server) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, "Failed reading backup payload", http.StatusBadRequest)
		return
	}

	if err := s.configMgr.ImportBackup(body); err != nil {
		jsonError(w, fmt.Sprintf("Restore failed: %v", err), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "ok",
		"message": "Configuration properties and statistics restored successfully!",
	})
}

func (s *Server) validateAuthToken(r *http.Request) bool {
	clientToken := r.Header.Get("X-Sirius-Token")
	if clientToken == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			clientToken = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if clientToken == "" {
		if cookie, err := r.Cookie("sirius_token"); err == nil {
			clientToken = cookie.Value
		}
	}
	return subtle.ConstantTimeCompare([]byte(clientToken), []byte(s.apiToken)) == 1
}

func (s *Server) handleRecoveryExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.validateAuthToken(r) {
		http.Error(w, `{"error":"Unauthorized: Invalid or missing API token"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Passphrase string `json:"passphrase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON request payload", http.StatusBadRequest)
		return
	}

	passBytes := []byte(req.Passphrase)
	req.Passphrase = ""
	defer config.ZeroBytes(passBytes)

	if len(passBytes) < 8 {
		jsonError(w, "Passphrase must be at least 8 characters long", http.StatusBadRequest)
		return
	}

	pkgData, err := s.configMgr.ExportDisasterRecoveryPackage(passBytes)
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed exporting disaster recovery package: %v", err), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("sirius-recovery-package-%s.drpkg", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pkgData)
}

func (s *Server) handleRecoveryRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.validateAuthToken(r) {
		http.Error(w, `{"error":"Unauthorized: Invalid or missing API token"}`, http.StatusUnauthorized)
		return
	}

	var pkgBytes []byte
	var passBytes []byte

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			jsonError(w, "Failed parsing multipart form", http.StatusBadRequest)
			return
		}
		passStr := r.FormValue("passphrase")
		passBytes = []byte(passStr)
		file, _, err := r.FormFile("package")
		if err != nil {
			jsonError(w, "Missing 'package' file in form upload", http.StatusBadRequest)
			return
		}
		defer file.Close()
		pkgBytes, err = io.ReadAll(file)
		if err != nil {
			jsonError(w, "Failed reading uploaded package file", http.StatusBadRequest)
			return
		}
	} else {
		var req struct {
			Passphrase string          `json:"passphrase"`
			Package    json.RawMessage `json:"package"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON request body", http.StatusBadRequest)
			return
		}
		passBytes = []byte(req.Passphrase)
		req.Passphrase = ""
		pkgBytes = []byte(req.Package)
	}

	defer config.ZeroBytes(passBytes)

	if len(passBytes) == 0 {
		jsonError(w, "Passphrase is required to decrypt recovery package", http.StatusBadRequest)
		return
	}

	if err := s.configMgr.RestoreDisasterRecoveryPackage(pkgBytes, passBytes); err != nil {
		jsonError(w, fmt.Sprintf("Disaster recovery restore failed: %v", err), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "ok",
		"message": "Full disaster recovery package restored successfully! All 21 configuration templates, certificates, and harvesting identity are active. Please restart the node.",
	})
}

func (s *Server) handleSystemShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok", "message": "ProximaX Sirius Core shutting down completely..."})

	go func() {
		time.Sleep(300 * time.Millisecond)
		log.Println("[Sirius Core] Received full shutdown command. Stopping node engine and exiting...")
		_ = s.supervisor.StopNode()
		os.Exit(0)
	}()
}

type SeedPingInfo struct {
	Endpoint  string `json:"endpoint"`
	LatencyMs int64  `json:"latencyMs"`
	Status    string `json:"status"`
}

func (s *Server) handlePeersDetail(w http.ResponseWriter, r *http.Request) {
	peers := s.supervisor.GetConnectedPeers()

	// Ping seed nodes concurrently with bounded timeout
	seedPings := make([]SeedPingInfo, len(chain.PublicMainnetNodes))
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	for i, node := range chain.PublicMainnetNodes {
		wg.Add(1)
		go func(idx int, target string) {
			defer wg.Done()
			start := time.Now()
			resp, err := client.Get(fmt.Sprintf("%s/chain/height", target))
			lat := time.Since(start).Milliseconds()
			status := "Online"
			if err != nil || resp.StatusCode != http.StatusOK {
				status = "Unreachable"
				lat = 0
			}
			if resp != nil {
				resp.Body.Close()
			}

			seedPings[idx] = SeedPingInfo{
				Endpoint:  target,
				LatencyMs: lat,
				Status:    status,
			}
		}(i, node)
	}
	wg.Wait()

	jsonResponse(w, map[string]interface{}{
		"peers":     peers,
		"seedNodes": seedPings,
		"count":     len(peers),
	})
}

// ----------------------------------------------------
// Storage & Replicator (DFMS) Handlers
// ----------------------------------------------------

func (s *Server) handleStorageStatus(w http.ResponseWriter, r *http.Request) {
	status := s.storageMgr.GetStatus()
	jsonResponse(w, status)
}

func (s *Server) handleStorageConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg, err := s.storageMgr.LoadStorageConfig()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Security: Never return raw private key over API
		cfg.Key = ""
		jsonResponse(w, cfg)
		return
	}

	if r.Method == "POST" {
		if !s.validateAuthToken(r) {
			http.Error(w, `{"error":"Unauthorized: Invalid or missing API token"}`, http.StatusUnauthorized)
			return
		}

		var req struct {
			Key         string `json:"key"`
			Host        string `json:"host"`
			StoragePath string `json:"storagePath"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		cleanKey := strings.TrimSpace(req.Key)
		if cleanKey != "" && len(cleanKey) != 64 {
			jsonError(w, "Replicator private key must be 64 hexadecimal characters", http.StatusBadRequest)
			return
		}

		if err := s.storageMgr.SaveStorageConfig(cleanKey, req.Host, req.StoragePath); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsonResponse(w, map[string]string{
			"status":  "ok",
			"message": "Storage Replicator configuration saved successfully!",
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleStorageKeyGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.validateAuthToken(r) {
		http.Error(w, `{"error":"Unauthorized: Invalid or missing API token"}`, http.StatusUnauthorized)
		return
	}

	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		jsonError(w, fmt.Sprintf("Key generation failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Auto-save generated key into config-storage.properties preserving current storage path
	currentCfg, _ := s.storageMgr.LoadStorageConfig()
	currentStoragePath := ""
	if currentCfg != nil {
		currentStoragePath = currentCfg.StoragePath
	}
	if err := s.storageMgr.SaveStorageConfig(kp.PrivateKey, "0.0.0.0", currentStoragePath); err != nil {
		jsonError(w, fmt.Sprintf("Failed saving generated replicator key: %v", err), http.StatusInternalServerError)
		return
	}
	// Explicitly clear sensitive key string from memory
	kp.PrivateKey = ""

	// Security Invariant: NEVER return raw private keys over the API
	jsonResponse(w, map[string]interface{}{
		"status":    "ok",
		"publicKey": kp.PublicKey,
		"address":   kp.Address,
		"message":   "New Replicator KeyPair generated and applied to config-storage.properties!",
	})
}

func (s *Server) handleStorageSandboxesClean(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.validateAuthToken(r) {
		http.Error(w, `{"error":"Unauthorized: Invalid or missing API token"}`, http.StatusUnauthorized)
		return
	}

	freedBytes, err := s.storageMgr.CleanSandboxes()
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed cleaning sandboxes: %v", err), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"status":     "ok",
		"freedBytes": freedBytes,
		"message":    fmt.Sprintf("Cleaned storage sandboxes, freed %.2f MB of temp drive space.", float64(freedBytes)/(1024*1024)),
	})
}

func (s *Server) handleStoragePeers(w http.ResponseWriter, r *http.Request) {
	peers := s.storageMgr.GetBootstrapReplicators()
	jsonResponse(w, peers)
}

func (s *Server) handleStorageOnboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.validateAuthToken(r) {
		http.Error(w, `{"error":"Unauthorized: Invalid or missing API token"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		CapacityGB  uint64 `json:"capacityGB"`
		FeeStrategy string `json:"feeStrategy"`
		ApiNodeUrl  string `json:"apiNodeUrl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.CapacityGB == 0 {
		req.CapacityGB = 50 // default 50 GB
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := s.storageMgr.OnboardReplicator(ctx, req.CapacityGB, req.FeeStrategy, req.ApiNodeUrl)
	if err != nil {
		jsonError(w, fmt.Sprintf("Onboarding failed: %v", err), http.StatusBadRequest)
		return
	}

	jsonResponse(w, result)
}

func (s *Server) handlePortCheck(w http.ResponseWriter, r *http.Request) {
	result := s.networkMgr.CheckPortReachability()
	jsonResponse(w, result)
}

func (s *Server) handleUPnPRemap(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go s.networkMgr.DiscoverAndMapUPnP()
	result := s.networkMgr.CheckPortReachability()

	jsonResponse(w, map[string]interface{}{
		"status":    "ok",
		"message":   "UPnP router port re-mapping triggered",
		"portCheck": result,
	})
}

func (s *Server) handleSystemWSLStatus(w http.ResponseWriter, r *http.Request) {
	status := s.supervisor.ProbeWSLStatus()
	jsonResponse(w, status)
}

func (s *Server) handleSystemWSLInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := s.supervisor.InitiateWSLInstall()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]interface{}{
		"status":  "initiated",
		"message": "WSL installation initiated. Please complete any administrator prompts.",
	})
}

func (s *Server) handleSystemWSLSetupDistro(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Distro string `json:"distro"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	err := s.supervisor.SetupWSLDistro(req.Distro)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]interface{}{
		"status":  "ok",
		"message": "WSL distribution setup completed.",
	})
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
