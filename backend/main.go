package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"proximax-sirius-core/pkg/api"
	"proximax-sirius-core/pkg/chain"
	"proximax-sirius-core/pkg/config"
	"proximax-sirius-core/pkg/supervisor"
)

//go:embed dist/*
var embeddedDist embed.FS

func main() {
	// Raise open file descriptor limits for RocksDB state cache on macOS
	var rLimit syscall.Rlimit
	rLimit.Cur = 65536
	rLimit.Max = 65536
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		rLimit.Cur = 10240
		rLimit.Max = 10240
		_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
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
		// Look in current dir or parent dir
		candidates := []string{"./chainconfig", "../chainconfig", "/app/chainconfig"}
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

	log.Printf("[Sirius Core] Initializing with chainconfig directory: %s", basePath)

	configMgr := config.NewConfigManager(basePath)
	supervisorCtrl := supervisor.NewProcessSupervisor(basePath)
	chainMon := chain.NewChainMonitor()

	// Graceful shutdown on SIGINT / SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		log.Println("[Sirius Core] Received termination signal. Stopping Sirius node engine...")
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

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("==================================================================")
	log.Printf("  ProximaX Sirius Core Node Manager is running!")
	log.Printf("  Open your browser at: http://localhost:%d", *port)
	log.Printf("==================================================================")

	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
