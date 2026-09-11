package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"proximax-sirius-core/pkg/api"
	"proximax-sirius-core/pkg/chain"
	"proximax-sirius-core/pkg/config"
	"proximax-sirius-core/pkg/supervisor"
)

//go:embed dist/*
var embeddedDist embed.FS

func openBrowser(targetUrl string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", targetUrl).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetUrl).Start()
	default:
		_ = exec.Command("xdg-open", targetUrl).Start()
	}
}

func main() {
	// Raise open file descriptor limits for RocksDB state cache on POSIX
	raiseFileDescriptorLimit()

	// 1. If running outside the executable directory (e.g. launched via macOS Finder where CWD defaults to $HOME),
	// change working directory to the executable directory if chainconfig or bin is adjacent.
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		if stat, err := os.Stat(filepath.Join(execDir, "chainconfig")); err == nil && stat.IsDir() {
			_ = os.Chdir(execDir)
		}
	}

	port := flag.Int("port", 8080, "Port for web GUI server")
	chainConfigPath := flag.String("chainconfig", "", "Path to chainconfig directory")
	flag.Parse()

	// Locate chainconfig
	basePath := *chainConfigPath
	if basePath == "" {
		basePath = os.Getenv("CHAINCONFIG_PATH")
	}
	if basePath == "" {
		var candidates []string
		if execPath, err := os.Executable(); err == nil {
			execDir := filepath.Dir(execPath)
			candidates = append(candidates,
				filepath.Join(execDir, "chainconfig"),
				filepath.Join(execDir, "..", "chainconfig"),
			)
		}
		candidates = append(candidates, "./chainconfig", "../chainconfig", "/app/chainconfig")
		for _, c := range candidates {
			if stat, err := os.Stat(c); err == nil && stat.IsDir() {
				basePath, _ = filepath.Abs(c)
				break
			}
		}
	}
	if basePath == "" {
		basePath, _ = filepath.Abs("./chainconfig")
	}

	// Check if port is already in use or active
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// If Sirius Core is already running on this port, open the dashboard in browser and exit gracefully
		client := http.Client{Timeout: 1 * time.Second}
		resp, checkErr := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/status", *port))
		if checkErr == nil && resp.StatusCode == 200 {
			_ = resp.Body.Close()
			log.Printf("==================================================================")
			log.Printf("  Sirius Core Web Manager is ALREADY RUNNING on port %d!", *port)
			log.Printf("  Opening Web Dashboard in browser: http://localhost:%d", *port)
			log.Printf("==================================================================")
			openBrowser(fmt.Sprintf("http://localhost:%d", *port))
			return
		}

		log.Fatalf("Server failed to bind to %s: %v. Stop any conflicting process with ./stop.sh", addr, err)
	}

	log.Printf("[Sirius Core] Initializing with chainconfig directory: %s", basePath)

	logsDir := filepath.Join(basePath, "logs")
	_ = os.MkdirAll(logsDir, 0755)
	if logFile, err := os.OpenFile(filepath.Join(logsDir, "manager.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}

	configMgr := config.NewConfigManager(basePath)
	supervisorCtrl := supervisor.NewProcessSupervisor(basePath)
	chainMon := chain.NewChainMonitor()

	// Ignore SIGHUP so terminal or session closures do not kill the daemon
	signal.Ignore(syscall.SIGHUP)

	// Graceful shutdown on SIGINT / SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigChan
		log.Printf("[Sirius Core] Received termination signal (%v). Stopping Sirius node engine...", sig)
		_ = supervisorCtrl.StopNode()
		os.Exit(0)
	}()

	var staticFS fs.FS
	// Check if embedded dist has files
	if sub, err := fs.Sub(embeddedDist, "dist"); err == nil {
		if _, err := sub.Open("index.html"); err == nil {
			staticFS = sub
			log.Println("[Sirius Core] Serving embedded frontend UI")
		}
	}

	// Fallback to local frontend/dist if developing
	if staticFS == nil {
		if stat, err := os.Stat("./frontend/dist"); err == nil && stat.IsDir() {
			staticFS = os.DirFS("./frontend/dist")
			log.Println("[Sirius Core] Serving frontend UI from ./frontend/dist")
		} else if stat, err := os.Stat("../frontend/dist"); err == nil && stat.IsDir() {
			staticFS = os.DirFS("../frontend/dist")
			log.Println("[Sirius Core] Serving frontend UI from ../frontend/dist")
		}
	}

	server := api.NewServer(configMgr, supervisorCtrl, chainMon, staticFS)

	log.Printf("==================================================================")
	log.Printf("  ProximaX Sirius Core Node Manager is running!")
	log.Printf("  Open your browser at: http://localhost:%d", *port)
	log.Printf("==================================================================")

	// Open web browser automatically on launch
	go func() {
		time.Sleep(400 * time.Millisecond)
		openBrowser(fmt.Sprintf("http://localhost:%d", *port))
	}()

	srv := &http.Server{
		Handler: server.Routes(),
	}
	if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
