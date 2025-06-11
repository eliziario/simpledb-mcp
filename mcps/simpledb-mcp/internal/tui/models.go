package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eliziario/simpledb-mcp/internal/config"
	"github.com/eliziario/simpledb-mcp/internal/credentials"
	"github.com/eliziario/simpledb-mcp/internal/database"
)

type AppState int

const (
	StateMenu AppState = iota
	StateConnections
	StateAddConnection
	StateEditConnection
	StateSettings
	StateLogs
	StateService
)

type Model struct {
	state  AppState
	config *config.Config
	width  int
	height int

	// Menu
	menuCursor  int
	menuOptions []string

	// Connections
	connections      []string
	connectionCursor int

	// Forms
	formInputs   []string
	formCursor   int
	formLabels   []string
	tempConn     config.Connection
	tempConnName string

	// Messages
	message     string
	messageType string // success, error, warning

	// Service status
	serviceStatus string
}

func NewModel() Model {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	model := Model{
		state:  StateMenu,
		config: cfg,
		menuOptions: []string{
			"Manage Connections",
			"Service Control",
			"Settings",
			"View Logs",
			"Exit",
		},
		formInputs: make([]string, 6), // name, host, port, database, username, password
		formLabels: []string{
			"Connection Name",
			"Host",
			"Port",
			"Database",
			"Username",
			"Password",
		},
		serviceStatus: "Unknown",
	}

	// Check initial service status
	if model.isServiceRunning() {
		model.serviceStatus = "Running"
	} else {
		model.serviceStatus = "Stopped"
	}

	return model
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeypress(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

func (m Model) handleKeypress(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateMenu:
		return m.handleMenuKeys(key)
	case StateConnections:
		return m.handleConnectionsKeys(key)
	case StateAddConnection, StateEditConnection:
		return m.handleFormKeys(key)
	case StateSettings:
		return m.handleSettingsKeys(key)
	case StateLogs:
		return m.handleLogsKeys(key)
	case StateService:
		return m.handleServiceKeys(key)
	}
	return m, nil
}

func (m Model) handleMenuKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.menuCursor = (m.menuCursor - 1 + len(m.menuOptions)) % len(m.menuOptions)
	case "down", "j":
		m.menuCursor = (m.menuCursor + 1) % len(m.menuOptions)
	case "enter":
		switch m.menuCursor {
		case 0: // Manage Connections
			m.state = StateConnections
			m.loadConnections()
		case 1: // Service Control
			m.state = StateService
		case 2: // Settings
			m.state = StateSettings
		case 3: // View Logs
			m.state = StateLogs
		case 4: // Exit
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) handleConnectionsKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc":
		m.state = StateMenu
		m.clearMessage()
	case "up", "k":
		if len(m.connections) > 0 {
			m.connectionCursor = (m.connectionCursor - 1 + len(m.connections)) % len(m.connections)
		}
	case "down", "j":
		if len(m.connections) > 0 {
			m.connectionCursor = (m.connectionCursor + 1) % len(m.connections)
		}
	case "a":
		m.state = StateAddConnection
		m.clearForm()
		m.clearMessage()
	case "e":
		if len(m.connections) > 0 {
			m.state = StateEditConnection
			m.loadConnectionForm(m.connections[m.connectionCursor])
			m.clearMessage()
		}
	case "d":
		if len(m.connections) > 0 {
			connName := m.connections[m.connectionCursor]
			if err := m.config.RemoveConnection(connName); err != nil {
				m.setErrorMessage(fmt.Sprintf("Failed to delete connection: %v", err))
			} else {
				m.setSuccessMessage(fmt.Sprintf("Connection '%s' deleted", connName))
				m.loadConnections()
			}
		}
	case "t":
		if len(m.connections) > 0 {
			connName := m.connections[m.connectionCursor]
			m.testConnection(connName)
		}
	}
	return m, nil
}

func (m Model) handleFormKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.state = StateConnections
		m.clearMessage()
	case "up", "shift+tab":
		m.formCursor = (m.formCursor - 1 + len(m.formInputs)) % len(m.formInputs)
	case "down", "tab":
		m.formCursor = (m.formCursor + 1) % len(m.formInputs)
	case "enter":
		if m.formCursor == len(m.formInputs)-1 { // On password field, save
			m.saveConnection()
		} else {
			m.formCursor = (m.formCursor + 1) % len(m.formInputs)
		}
	case "backspace":
		if len(m.formInputs[m.formCursor]) > 0 {
			m.formInputs[m.formCursor] = m.formInputs[m.formCursor][:len(m.formInputs[m.formCursor])-1]
		}
	case "ctrl+u": // Clear current field
		m.formInputs[m.formCursor] = ""
	case "ctrl+v": // This won't actually trigger, but we handle pasted content below
		// Paste is handled in the default case
	default:
		// Handle both single characters and pasted text
		keyStr := key.String()
		if len(keyStr) >= 1 {
			// Filter out control characters but allow printable characters, spaces, and common password symbols
			filtered := ""
			for _, char := range keyStr {
				// Allow ASCII printable characters (32-126), including spaces and symbols
				// Also allow some extended ASCII for international characters
				if (char >= 32 && char <= 126) || char == '\t' {
					if char == '\t' {
						// Convert tab to spaces for better display
						filtered += "    "
					} else {
						filtered += string(char)
					}
				}
			}
			if filtered != "" {
				m.formInputs[m.formCursor] += filtered
			}
		}
	}
	return m, nil
}

func (m Model) handleSettingsKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc":
		m.state = StateMenu
	}
	return m, nil
}

func (m Model) handleLogsKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc":
		m.state = StateMenu
	}
	return m, nil
}

func (m Model) handleServiceKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc":
		m.state = StateMenu
	case "s":
		m.startService()
	case "p":
		m.stopService()
	case "r":
		m.checkServiceStatus()
	}
	return m, nil
}

// Helper methods
func (m *Model) loadConnections() {
	m.connections = m.config.ListConnections()
	m.connectionCursor = 0
}

func (m *Model) clearForm() {
	for i := range m.formInputs {
		m.formInputs[i] = ""
	}
	m.formCursor = 0
	m.tempConnName = ""
	m.tempConn = config.Connection{}
}

func (m *Model) loadConnectionForm(connName string) {
	m.tempConnName = connName
	conn, exists := m.config.GetConnection(connName)
	if !exists {
		return
	}

	m.tempConn = conn
	m.formInputs[0] = connName
	m.formInputs[1] = conn.Host
	m.formInputs[2] = fmt.Sprintf("%d", conn.Port)
	m.formInputs[3] = conn.Database
	m.formInputs[4] = conn.Username
	m.formInputs[5] = "" // Don't show password
	m.formCursor = 0
}

func (m *Model) saveConnection() {
	// Validate and create connection
	connName := strings.TrimSpace(m.formInputs[0])
	if connName == "" {
		m.setErrorMessage("Connection name is required")
		return
	}

	// For now, default to MySQL. In full implementation, add type selection
	conn := config.Connection{
		Type:     "mysql",
		Host:     strings.TrimSpace(m.formInputs[1]),
		Database: strings.TrimSpace(m.formInputs[3]),
		Username: strings.TrimSpace(m.formInputs[4]),
	}

	// Parse port
	if portStr := strings.TrimSpace(m.formInputs[2]); portStr != "" {
		var port int
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
			m.setErrorMessage("Invalid port number")
			return
		}
		conn.Port = port
	} else {
		conn.Port = 3306 // Default MySQL port
	}

	// Save connection
	if err := m.config.AddConnection(connName, conn); err != nil {
		m.setErrorMessage(fmt.Sprintf("Failed to save connection: %v", err))
		return
	}

	// Store password in keychain if provided
	if password := strings.TrimSpace(m.formInputs[5]); password != "" {
		credManager := credentials.NewManager(m.config.Settings.CacheCredentials)
		if err := credManager.Store(connName, conn.Username, password); err != nil {
			m.setErrorMessage(fmt.Sprintf("Failed to store credentials: %v", err))
			return
		}
	}

	m.setSuccessMessage(fmt.Sprintf("Connection '%s' saved successfully", connName))
	m.state = StateConnections
	m.loadConnections()
}

func (m *Model) testConnection(connName string) {
	_, exists := m.config.GetConnection(connName)
	if !exists {
		m.setErrorMessage(fmt.Sprintf("Connection '%s' not found", connName))
		return
	}

	// Create a database manager to test the connection
	credManager := credentials.NewManager(m.config.Settings.CacheCredentials)
	dbManager := database.NewManager(m.config, credManager)
	defer dbManager.Close()

	// Test the connection
	if err := dbManager.TestConnection(connName); err != nil {
		m.setErrorMessage(fmt.Sprintf("Connection test failed: %v", err))
	} else {
		m.setSuccessMessage(fmt.Sprintf("Connection '%s' test successful!", connName))
	}
}

func (m *Model) startService() {
	// Check if service is already running
	if m.isServiceRunning() {
		m.setWarningMessage("Service is already running")
		return
	}

	// Start the service in background
	m.setSuccessMessage("Starting SimpleDB MCP service...")

	// Find the server binary path
	serverPath := "./bin/simpledb-mcp"
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		// Try relative to CLI location
		cliDir := filepath.Dir(os.Args[0])
		serverPath = filepath.Join(cliDir, "simpledb-mcp")
		if _, err := os.Stat(serverPath); os.IsNotExist(err) {
			m.setErrorMessage("SimpleDB MCP server binary not found")
			return
		}
	}

	// Use nohup to start service in background  
	cmd := exec.Command("nohup", serverPath)
	cmd.Dir = filepath.Dir(serverPath)

	if err := cmd.Start(); err != nil {
		m.setErrorMessage(fmt.Sprintf("Failed to start service: %v", err))
		return
	}

	// Give it a moment to start
	time.Sleep(500 * time.Millisecond)

	// Check if it actually started
	if m.isServiceRunning() {
		m.serviceStatus = "Running"
		m.setSuccessMessage("Service started successfully")
	} else {
		m.setErrorMessage("Service failed to start")
	}
}

func (m *Model) stopService() {
	// Check if service is running
	if !m.isServiceRunning() {
		m.setWarningMessage("Service is not running")
		return
	}

	m.setSuccessMessage("Stopping SimpleDB MCP service...")

	// Find and kill the service process
	cmd := exec.Command("pkill", "-f", "simpledb-mcp")
	if err := cmd.Run(); err != nil {
		// Try alternative approach with pgrep + kill
		pgrepCmd := exec.Command("pgrep", "-f", "simpledb-mcp")
		output, err := pgrepCmd.Output()
		if err != nil {
			m.setErrorMessage("Failed to find running service process")
			return
		}

		pids := strings.Fields(string(output))
		for _, pid := range pids {
			killCmd := exec.Command("kill", pid)
			killCmd.Run() // Ignore errors for individual kills
		}
	}

	// Give it a moment to stop
	time.Sleep(500 * time.Millisecond)
	
	// Verify it stopped
	if !m.isServiceRunning() {
		m.serviceStatus = "Stopped"
		m.setSuccessMessage("Service stopped successfully")
	} else {
		m.setWarningMessage("Service may still be running")
	}
}

func (m *Model) checkServiceStatus() {
	if m.isServiceRunning() {
		m.serviceStatus = "Running"
		m.setSuccessMessage("Service is running")
	} else {
		m.serviceStatus = "Stopped"
		m.setSuccessMessage("Service is stopped")
	}
}

// Helper function to check if the service is running
func (m *Model) isServiceRunning() bool {
	// Check if simpledb-mcp process is running
	cmd := exec.Command("pgrep", "-f", "simpledb-mcp")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Filter out our own CLI process
	pids := strings.Fields(string(output))
	myPid := fmt.Sprintf("%d", os.Getpid())

	for _, pid := range pids {
		if pid != myPid {
			// Check if this PID is actually the server by looking at command line
			cmdlineCmd := exec.Command("ps", "-p", pid, "-o", "args=")
			cmdOutput, err := cmdlineCmd.Output()
			if err != nil {
				continue
			}

			cmdline := string(cmdOutput)
			// Look for the server binary, not the CLI
			if strings.Contains(cmdline, "simpledb-mcp") && !strings.Contains(cmdline, "simpledb-cli") {
				return true
			}
		}
	}

	return false
}

func (m *Model) setSuccessMessage(msg string) {
	m.message = msg
	m.messageType = "success"
}

func (m *Model) setErrorMessage(msg string) {
	m.message = msg
	m.messageType = "error"
}

func (m *Model) setWarningMessage(msg string) {
	m.message = msg
	m.messageType = "warning"
}

func (m *Model) clearMessage() {
	m.message = ""
	m.messageType = ""
}
