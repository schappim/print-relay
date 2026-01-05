package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"printrelay/cloudserver"
)

func main() {
	// CLI flags
	port := flag.String("port", "8080", "Port to listen on")
	adminKey := flag.String("admin-key", "", "Admin key for tenant management API")
	apiKey := flag.String("api-key", "", "Default API key (creates default tenant)")
	clientKey := flag.String("client-key", "", "Default client key (creates default tenant)")
	dataDir := flag.String("data-dir", "./data", "Directory for persistent data storage")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Generate keys if not provided
	adminKeyVal := *adminKey
	if adminKeyVal == "" {
		adminKeyVal = cloudserver.GenerateKey()
		log.Printf("Generated Admin Key: %s", adminKeyVal)
	}

	apiKeyVal := *apiKey
	if apiKeyVal == "" {
		apiKeyVal = cloudserver.GenerateKey()
		log.Printf("Generated API Key: %s", apiKeyVal)
	}

	clientKeyVal := *clientKey
	if clientKeyVal == "" {
		clientKeyVal = cloudserver.GenerateKey()
		log.Printf("Generated Client Key: %s", clientKeyVal)
	}

	log.Println("--------------------------------------")
	log.Println("Admin Key: For tenant management (/admin/* endpoints)")
	log.Println("API Key: For HTTP API authentication (Basic Auth with key as username)")
	log.Println("Client Key: For PrintRelay client connections")
	log.Println("--------------------------------------")

	// Ensure data directory exists
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create and configure server
	srv, err := cloudserver.New(cloudserver.Config{
		AdminKey:         adminKeyVal,
		DefaultAPIKey:    apiKeyVal,
		DefaultClientKey: clientKeyVal,
		DataDir:          *dataDir,
		Verbose:          *verbose,
	})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	addr := ":" + *port
	log.Printf("PrintRelay server starting on http://localhost%s", addr)
	log.Printf("WebSocket endpoint: ws://localhost%s/ws", addr)
	log.Printf("Monitor endpoint: ws://localhost%s/monitor/{token}", addr)

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
