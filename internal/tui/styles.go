package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	tabActive   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")).Padding(0, 2)
	tabInactive = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 2)

	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	headStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	activeMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)
