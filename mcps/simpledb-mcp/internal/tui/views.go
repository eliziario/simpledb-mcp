package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	var content string

	switch m.state {
	case StateMenu:
		content = m.menuView()
	case StateConnections:
		content = m.connectionsView()
	case StateAddConnection:
		content = m.formView("Add Database Connection")
	case StateEditConnection:
		content = m.formView("Edit Database Connection")
	case StateSettings:
		content = m.settingsView()
	case StateLogs:
		content = m.logsView()
	case StateService:
		content = m.serviceView()
	}

	// Add message if present
	if m.message != "" {
		var msgStyle lipgloss.Style
		switch m.messageType {
		case "success":
			msgStyle = successStyle
		case "error":
			msgStyle = errorStyle
		case "warning":
			msgStyle = warningStyle
		default:
			msgStyle = lipgloss.NewStyle()
		}
		content += "\n" + msgStyle.Render(m.message)
	}

	return baseStyle.Render(content)
}

func (m Model) menuView() string {
	title := titleStyle.Render("🗄️  SimpleDB MCP Configuration")
	subtitle := subtitleStyle.Render("Secure database access with biometric authentication")

	var menuItems strings.Builder
	for i, option := range m.menuOptions {
		cursor := " "
		style := listItemStyle
		if i == m.menuCursor {
			cursor = ">"
			style = selectedListItemStyle
		}
		menuItems.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, option)) + "\n")
	}

	help := helpStyle.Render("↑/↓: Navigate • Enter: Select • q: Quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		subtitle,
		"",
		menuItems.String(),
		help,
	)
}

func (m Model) connectionsView() string {
	title := titleStyle.Render("Database Connections")

	var connectionsList strings.Builder
	if len(m.connections) == 0 {
		connectionsList.WriteString(helpStyle.Render("No connections configured"))
	} else {
		for i, conn := range m.connections {
			cursor := " "
			style := listItemStyle
			if i == m.connectionCursor {
				cursor = ">"
				style = selectedListItemStyle
			}

			// Get connection details for display
			if connConfig, exists := m.config.GetConnection(conn); exists {
				display := fmt.Sprintf("%s %s (%s) - %s:%d/%s", 
					cursor, conn, connConfig.Type, connConfig.Host, connConfig.Port, connConfig.Database)
				connectionsList.WriteString(style.Render(display) + "\n")
			} else {
				connectionsList.WriteString(style.Render(fmt.Sprintf("%s %s (error)", cursor, conn)) + "\n")
			}
		}
	}

	actions := helpStyle.Render("a: Add • e: Edit • d: Delete • t: Test • q: Back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		connectionsList.String(),
		"",
		actions,
	)
}

func (m Model) formView(title string) string {
	titleRendered := titleStyle.Render(title)

	var form strings.Builder
	for i, label := range m.formLabels {
		labelRendered := labelStyle.Render(label + ":")
		
		var inputRendered string
		value := m.formInputs[i]
		if label == "Password" && value != "" {
			value = strings.Repeat("●", len(value))
		}

		if i == m.formCursor {
			inputRendered = focusedInputStyle.Render(value + "│")
		} else {
			inputRendered = inputStyle.Render(value)
		}

		form.WriteString(labelRendered + "\n")
		form.WriteString(inputRendered + "\n")
	}

	help := helpStyle.Render("Tab/↑↓: Navigate • Enter: Save • Esc: Cancel • Ctrl+U: Clear • Paste: Cmd+V (Mac) or Ctrl+V")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleRendered,
		"",
		form.String(),
		help,
	)
}

func (m Model) settingsView() string {
	title := titleStyle.Render("Settings")

	settings := fmt.Sprintf(`Query Timeout: %s
Max Rows: %d
Cache Credentials: %s
Require Biometric: %t

Config Location: %s`,
		m.config.Settings.QueryTimeout,
		m.config.Settings.MaxRows,
		m.config.Settings.CacheCredentials,
		m.config.Settings.RequireBiometric,
		"~/.config/simpledb-mcp/config.yaml",
	)

	help := helpStyle.Render("q: Back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		borderStyle.Render(settings),
		"",
		help,
	)
}

func (m Model) logsView() string {
	title := titleStyle.Render("Server Logs")

	logs := `[2024-01-15 10:30:22] INFO: Starting SimpleDB MCP Server v0.1.0
[2024-01-15 10:30:22] INFO: Configuration loaded with 2 connections
[2024-01-15 10:30:23] INFO: Server listening on stdio
[2024-01-15 10:31:15] INFO: Tool 'list_connections' called
[2024-01-15 10:31:16] INFO: Tool 'list_databases' called with connection 'local-mysql'

(Live logs would be displayed here)`

	help := helpStyle.Render("q: Back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		borderStyle.Render(logs),
		"",
		help,
	)
}

func (m Model) serviceView() string {
	title := titleStyle.Render("Service Control")

	statusColor := successColor
	if m.serviceStatus == "Stopped" || m.serviceStatus == "Unknown" {
		statusColor = errorColor
	}

	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)
	status := fmt.Sprintf("Service Status: %s", statusStyle.Render(m.serviceStatus))

	controls := `Available Actions:
• [s] Start Service
• [p] Stop Service  
• [r] Refresh Status

Service will be installed as:
• macOS: ~/Library/LaunchAgents/com.simpledb-mcp.plist
• Windows: Windows Service 'SimpleDB MCP'`

	help := helpStyle.Render("s: Start • p: Stop • r: Refresh • q: Back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		status,
		"",
		borderStyle.Render(controls),
		"",
		help,
	)
}