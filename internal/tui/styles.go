package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#00ADD8") // Go blue
	secondaryColor = lipgloss.Color("#5C6AC4")
	successColor   = lipgloss.Color("#00D084")
	errorColor     = lipgloss.Color("#FF6B6B")
	warningColor   = lipgloss.Color("#FFB74D")
	mutedColor     = lipgloss.Color("#64748B")

	// Base styles
	baseStyle = lipgloss.NewStyle().
		Padding(1, 2)

	// Header styles
	titleStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Align(lipgloss.Center).
		Padding(1, 0)

	subtitleStyle = lipgloss.NewStyle().
		Foreground(mutedColor).
		Align(lipgloss.Center).
		Margin(0, 0, 1, 0)

	// Button styles
	buttonStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(primaryColor).
		Padding(0, 3).
		Margin(0, 1)

	selectedButtonStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(secondaryColor).
		Padding(0, 3).
		Margin(0, 1).
		Bold(true)

	// List styles
	listItemStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Margin(0, 0, 1, 0)

	selectedListItemStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Background(lipgloss.Color("#E3F2FD")).
		Padding(0, 1).
		Margin(0, 0, 1, 0).
		Bold(true)

	// Form styles
	inputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(mutedColor).
		Padding(0, 1).
		Margin(0, 0, 1, 0)

	focusedInputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1).
		Margin(0, 0, 1, 0)

	labelStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Margin(0, 0, 0, 0)

	// Status styles
	successStyle = lipgloss.NewStyle().
		Foreground(successColor).
		Bold(true)

	errorStyle = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true)

	warningStyle = lipgloss.NewStyle().
		Foreground(warningColor).
		Bold(true)

	// Container styles
	borderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2)

	helpStyle = lipgloss.NewStyle().
		Foreground(mutedColor).
		Margin(1, 0, 0, 0)
)