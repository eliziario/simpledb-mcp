package main

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/eliziario/simpledb-mcp/internal/credentials"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <connection-name> <username>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s salesforce presence-rw\n", os.Args[0])
		os.Exit(1)
	}

	connectionName := os.Args[1]
	username := os.Args[2]

	// Prompt for password
	fmt.Printf("Enter password for %s@%s: ", username, connectionName)
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError reading password: %v\n", err)
		os.Exit(1)
	}
	fmt.Println() // New line after password input

	password := string(passwordBytes)
	if password == "" {
		fmt.Fprintf(os.Stderr, "Password cannot be empty\n")
		os.Exit(1)
	}

	// Store credentials
	credManager := credentials.NewManager(5 * time.Minute)
	if err := credManager.Store(connectionName, username, password); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to store credentials: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Credentials stored successfully for %s@%s\n", username, connectionName)
}