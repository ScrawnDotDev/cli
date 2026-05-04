package ui

import "github.com/charmbracelet/lipgloss"

var (
	appStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("221"))
	sectionStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("221"))
	mutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	successStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	warnStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	failureStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	stepStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
	labelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	subtleRuleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	Heading      = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
)
