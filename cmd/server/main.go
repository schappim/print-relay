package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"printrelay/cloudserver"
)

func main() {
	// CLI flags
	port := flag.String("port", "8080", "Port to listen on")
	apiKey := flag.String("api-key", "", "API key for HTTP API authentication (can specify multiple comma-separated)")
	clientKey := flag.String("client-key", "", "Key for client WebSocket authentication")
	dataDir := flag.String("data-dir", "./data", "Directory for persistent data storage")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Generate keys if not provided
	apiKeys := []string{}
	if *apiKey == "" {
		key := cloudserver.GenerateKey()
		apiKeys = append(apiKeys, key)
		log.Printf("Generated API Key: %s", key)
	} else {
		apiKeys = strings.Split(*apiKey, ",")
		for i, k := range apiKeys {
			apiKeys[i] = strings.TrimSpace(k)
		}
	}

	clientKeyVal := *clientKey
	if clientKeyVal == "" {
		clientKeyVal = cloudserver.GenerateKey()
		log.Printf("Generated Client Key: %s", clientKeyVal)
	}

	log.Println("--------------------------------------")
	log.Println("Use API Key for HTTP API authentication (Basic Auth with key as username)")
	log.Println("Use Client Key when starting PrintRelay clients")
	log.Println("--------------------------------------")

	// Ensure data directory exists
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create and configure server
	srv, err := cloudserver.New(cloudserver.Config{
		APIKeys:   apiKeys,
		ClientKey: clientKeyVal,
		DataDir:   *dataDir,
		Verbose:   *verbose,
	})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	addr := ":" + *port
	log.Printf("PrintRelay server starting on http://localhost%s", addr)
	log.Printf("WebSocket endpoint: ws://localhost%s/ws", addr)

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
