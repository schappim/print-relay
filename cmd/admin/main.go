package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"printrelay/internal/admin"
)

var version = "dev"

func main() {
	// CLI flags
	serverURL := flag.String("server", "", "PrintRelay server URL")
	apiKey := flag.String("key", "", "API key for authentication")
	adminKey := flag.String("admin-key", "", "Admin key for tenant management")
	configFile := flag.String("config", "", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("printrelay-admin %s\n", version)
		os.Exit(0)
	}

	// Load configuration
	var cfg admin.Config

	// Try config file first
	if *configFile != "" {
		loadedCfg, err := admin.LoadConfig(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		cfg = *loadedCfg
	} else if foundConfig := admin.FindConfigFile(); foundConfig != "" {
		loadedCfg, err := admin.LoadConfig(foundConfig)
		if err == nil {
			cfg = *loadedCfg
		}
	}

	// Override with CLI flags
	if *serverURL != "" {
		cfg.ServerURL = *serverURL
	}
	if *apiKey != "" {
		cfg.APIKey = *apiKey
	}
	if *adminKey != "" {
		cfg.AdminKey = *adminKey
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nUsage: printrelay-admin -server URL -key API_KEY\n")
		fmt.Fprintf(os.Stderr, "   or: printrelay-admin -config /path/to/config.yaml\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create and run application
	model, err := admin.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
