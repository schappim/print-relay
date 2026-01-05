package main

import (
	"flag"
	"log"
	"os"

	"printrelay/client"
)

func main() {
	// CLI flags
	serverURL := flag.String("server", "http://localhost:8080", "PrintRelay server URL")
	clientKey := flag.String("key", "", "Client authentication key (required)")
	dataDir := flag.String("data-dir", "./data", "Directory for temporary files")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Validate required flags
	if *clientKey == "" {
		log.Fatal("Client key is required. Use -key flag.")
	}

	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Parse server URL
	wsURL, err := client.ParseServerURL(*serverURL)
	if err != nil {
		log.Fatalf("Invalid server URL: %v", err)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create client
	c, err := client.New(client.Config{
		ServerURL: wsURL,
		ClientKey: *clientKey,
		DataDir:   *dataDir,
		Verbose:   *verbose,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	log.Printf("PrintRelay Client starting...")
	log.Printf("Connecting to: %s", wsURL)

	// Run client (blocking, reconnects automatically)
	if err := c.Run(); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}
