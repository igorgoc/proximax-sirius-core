package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"proximax-sirius-core/pkg/chain"
)

func main() {
	resourcesDir := flag.String("resources", "", "Path to resources directory (containing config-harvesting.properties)")
	dataPath := flag.String("data", "", "Path to data directory")
	apiNodes := flag.String("api-nodes", "", "Comma-separated list of custom Sirius API endpoints")
	flag.Parse()

	if *resourcesDir == "" {
		if envRes := os.Getenv("RESOURCES_DIR"); envRes != "" {
			*resourcesDir = envRes
		} else if envData := os.Getenv("DATA_DIR"); envData != "" {
			*resourcesDir = filepath.Join(envData, "resources")
		} else {
			*resourcesDir = "./chainconfig/resources"
		}
	}

	if *dataPath == "" {
		if envData := os.Getenv("DATA_DIR"); envData != "" {
			*dataPath = filepath.Join(envData, "data")
		} else {
			*dataPath = "./chainconfig/data"
		}
	}

	absRes, err := filepath.Abs(*resourcesDir)
	if err != nil {
		absRes = *resourcesDir
	}
	absData, err := filepath.Abs(*dataPath)
	if err != nil {
		absData = *dataPath
	}

	var customNodes []string
	if *apiNodes != "" {
		for _, n := range strings.Split(*apiNodes, ",") {
			trimmed := strings.TrimSpace(n)
			if trimmed != "" {
				customNodes = append(customNodes, trimmed)
			}
		}
	}

	log.Printf("==================================================================")
	log.Printf("  ProximaX Sirius Autonomous Delegation Listener Daemon")
	log.Printf("  Resources Directory : %s", absRes)
	log.Printf("  Data Directory      : %s", absData)
	log.Printf("  Delegated Keys Path : %s", filepath.Join(absRes, "delegated_keys"))
	log.Printf("==================================================================")

	dl := chain.NewDelegationListener(absRes, absData, customNodes)
	dl.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("[Delegation Listener] Received signal %v, shutting down gracefully...", sig)
	dl.Stop()
	log.Printf("[Delegation Listener] Stopped.")
}
