package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/credentials"
)

func main() {
	if len(os.Args) != 6 {
		fmt.Println("Usage: go run store_sf_creds.go <connection_name> <instance_url> <username> <password> <security_token>")
		fmt.Println("Example: go run store_sf_creds.go salesforce-prod https://mycompany.my.salesforce.com user@company.com mypassword AbC123dEf456GhI789")
		os.Exit(1)
	}

	connectionName := os.Args[1]
	instanceURL := os.Args[2]
	username := os.Args[3]
	password := os.Args[4]
	securityToken := os.Args[5]

	// Create credential manager
	credManager := credentials.NewManager(5 * time.Minute)

	// Store Salesforce credentials
	if err := credManager.StoreSalesforce(connectionName, username, password, securityToken); err != nil {
		log.Fatalf("Failed to store Salesforce credentials: %v", err)
	}

	fmt.Printf("✅ Salesforce credentials stored successfully for connection '%s'\n", connectionName)

	// Update config.yaml
	if err := updateConfigWithSalesforce(connectionName, instanceURL); err != nil {
		log.Printf("⚠️  Warning: Failed to update config.yaml: %v", err)
		fmt.Println("Please manually add this connection to your config.yaml:")
		fmt.Printf("  %s:\n    type: salesforce\n    host: %s\n", connectionName, instanceURL)
	} else {
		fmt.Printf("✅ Config updated: Added Salesforce connection '%s' to config.yaml\n", connectionName)
	}
}

func updateConfigWithSalesforce(connectionName, instanceURL string) error {
	// Load existing config or create default
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	// Add the Salesforce connection
	sfConnection := config.Connection{
		Type: "salesforce",
		Host: instanceURL,
		// Salesforce connections don't need port/database fields
	}
	
	if err := cfg.AddConnection(connectionName, sfConnection); err != nil {
		return fmt.Errorf("failed to add connection: %w", err)
	}
	
	return nil
}