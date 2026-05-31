package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mxcd/multiverse/internal/brain"
)

// statsMsg carries a brain's git state and note count back to the model.
type statsMsg struct {
	name  string
	info  brain.GitInfo
	notes int
	err   error
}

// lintMsg carries a brain's lint result.
type lintMsg struct {
	name     string
	pass     bool
	findings int
	checked  int
	err      error
}

// syncMsg carries a brain's sync result.
type syncMsg struct {
	name string
	res  brain.SyncResult
	err  error
}

func loadStats(name, path string) tea.Cmd {
	return func() tea.Msg {
		b, err := brain.Open(path)
		if err != nil {
			return statsMsg{name: name, err: err}
		}
		return statsMsg{name: name, info: b.GitInfo(), notes: b.NoteCount()}
	}
}

func lintBrain(name, path string) tea.Cmd {
	return func() tea.Msg {
		b, err := brain.Open(path)
		if err != nil {
			return lintMsg{name: name, err: err}
		}
		rep, err := b.Lint(brain.AllRules())
		if err != nil {
			return lintMsg{name: name, err: err}
		}
		return lintMsg{name: name, pass: rep.OK(), findings: len(rep.Findings), checked: rep.Checked}
	}
}

func syncBrain(name, path string) tea.Cmd {
	return func() tea.Msg {
		b, err := brain.Open(path)
		if err != nil {
			return syncMsg{name: name, err: err}
		}
		res, err := b.Sync("")
		return syncMsg{name: name, res: res, err: err}
	}
}
