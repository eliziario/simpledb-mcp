package main

import (
	"fmt"
	"os"

	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/credentials"
	"github.com/eliziario/simpledb-mcp/internal/database"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <connection-name> [password]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "If password is provided, it will be stored first\n")
		os.Exit(1)
	}

	connectionName := os.Args[1]

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Check if connection exists
	conn, exists := cfg.GetConnection(connectionName)
	if !exists {
		fmt.Fprintf(os.Stderr, "Connection '%s' not found in configuration\n", connectionName)
		os.Exit(1)
	}

	fmt.Printf("Testing connection '%s':\n", connectionName)
	fmt.Printf("  Type: %s\n", conn.Type)
	fmt.Printf("  Host: %s:%d\n", conn.Host, conn.Port)
	fmt.Printf("  Database: %s\n", conn.Database)
	fmt.Printf("  Username: %s\n", conn.Username)

	// If password provided, store it first
	if len(os.Args) >= 3 {
		password := os.Args[2]
		credManager := credentials.NewManager(cfg.Settings.CacheCredentials)
		fmt.Printf("Storing credentials...\n")
		if err := credManager.Store(connectionName, conn.Username, password); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to store credentials: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Credentials stored successfully\n")
	}

	// Test the connection
	credManager := credentials.NewManager(cfg.Settings.CacheCredentials)
	dbManager := database.NewManager(cfg, credManager)
	defer dbManager.Close()

	fmt.Printf("Testing database connection...\n")
	if err := dbManager.TestConnection(connectionName); err != nil {
		fmt.Printf("❌ Connection "+
			"test failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Connection test successful!\n")
}
