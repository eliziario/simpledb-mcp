package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eliziario/simpledb-mcp/internal/tui"
)

func main() {
	// Check for command line arguments
	if len(os.Args) > 1 {
		handleCLICommands()
		return
	}

	// Run TUI
	runTUI()
}

func handleCLICommands() {
	command := os.Args[1]
	
	switch command {
	case "config":
		runTUI()
	case "connection":
		handleConnectionCommands()
	case "service":
		handleServiceCommands()
	case "logs":
		handleLogsCommand()
	case "help", "--help", "-h":
		printHelp()
	case "version", "--version", "-v":
		printVersion()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func handleConnectionCommands() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: simpledb-cli connection <add|list|test|remove> [name]")
		os.Exit(1)
	}

	subcommand := os.Args[2]
	
	switch subcommand {
	case "add":
		fmt.Println("Use 'simpledb-cli config' for interactive connection management")
	case "list":
		listConnections()
	case "test":
		if len(os.Args) < 4 {
			fmt.Println("Usage: simpledb-cli connection test <connection-name>")
			os.Exit(1)
		}
		testConnection(os.Args[3])
	case "remove":
		if len(os.Args) < 4 {
			fmt.Println("Usage: simpledb-cli connection remove <connection-name>")
			os.Exit(1)
		}
		removeConnection(os.Args[3])
	default:
		fmt.Printf("Unknown connection command: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleServiceCommands() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: simpledb-cli service <status|start|stop|install|uninstall>")
		os.Exit(1)
	}

	subcommand := os.Args[2]
	
	switch subcommand {
	case "status":
		checkServiceStatus()
	case "start":
		startService()
	case "stop":
		stopService()
	case "install":
		installService()
	case "uninstall":
		uninstallService()
	default:
		fmt.Printf("Unknown service command: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleLogsCommand() {
	fmt.Println("Viewing server logs...")
	// TODO: Implement log viewing
	fmt.Println("Log viewing not yet implemented. Use 'simpledb-cli config' for interactive mode.")
}

func runTUI() {
	model := tui.NewModel()
	
	p := tea.NewProgram(
		model, 
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}

func printHelp() {
	fmt.Print(`SimpleDB MCP CLI - Database configuration and management tool

USAGE:
    simpledb-cli [COMMAND]

COMMANDS:
    config              Launch interactive configuration TUI (default)
    connection          Manage database connections
        add             Add a new connection (interactive)
        list            List configured connections
        test <name>     Test a connection
        remove <name>   Remove a connection
    service             Control the MCP server service
        status          Check service status
        start           Start the service
        stop            Stop the service
        install         Install as system service
        uninstall       Remove system service
    logs                View server logs
    help                Show this help message
    version             Show version information

EXAMPLES:
    simpledb-cli                           # Launch interactive TUI
    simpledb-cli config                    # Launch interactive TUI
    simpledb-cli connection list           # List all connections
    simpledb-cli connection test prod-db   # Test connection 'prod-db'
    simpledb-cli service status            # Check if service is running
    simpledb-cli service install           # Install as system service

For interactive configuration and management, run without arguments or use 'config'.
`)
}

func printVersion() {
	fmt.Println("SimpleDB MCP CLI v0.1.0")
	fmt.Println("A secure database exploration tool with biometric authentication")
}

// Placeholder implementations for CLI commands
func listConnections() {
	fmt.Println("Listing connections...")
	// TODO: Implement connection listing
}

func testConnection(name string) {
	fmt.Printf("Testing connection '%s'...\n", name)
	// TODO: Implement connection testing
}

func removeConnection(name string) {
	fmt.Printf("Removing connection '%s'...\n", name)
	// TODO: Implement connection removal
}

func checkServiceStatus() {
	fmt.Println("Checking service status...")
	// TODO: Implement service status check
}

func startService() {
	fmt.Println("Starting SimpleDB MCP service...")
	// TODO: Implement service start
}

func stopService() {
	fmt.Println("Stopping SimpleDB MCP service...")
	// TODO: Implement service stop
}

func installService() {
	fmt.Println("Installing SimpleDB MCP as system service...")
	// TODO: Implement service installation
}

func uninstallService() {
	fmt.Println("Uninstalling SimpleDB MCP service...")
	// TODO: Implement service uninstallation
}