// Package tui is the Bubble Tea control panel for multi: a dashboard over all
// registered brains (git state, note counts, lint, sync) plus config for the
// global registry and the current directory's brain binding.
package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/mxcd/multiverse/internal/config"
)

type viewID int

const (
	dashView viewID = iota
	brainsView
	scopeView
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeAddName
	modeAddPath
	modeRename
	modeSetPath
	modeConfirmDelete
)

type brainRow struct {
	name    string
	path    string
	active  bool
	info    brain.GitInfo
	notes   int
	loading bool
	lint    string // "", "ok", "fail:N", "err"
	err     string
}

// Model is the Bubble Tea model for the control panel.
type Model struct {
	cfg    *config.Config
	view   viewID
	width  int
	height int

	rows   []brainRow
	cursor int

	cwd          string
	bindingOrigin string
	srcSel       map[string]bool
	tgtSel       map[string]bool

	mode     inputMode
	input    textinput.Model
	pendName string

	status string
}

// Run launches the control panel.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(newModel(cfg), tea.WithAltScreen()).Run()
	return err
}

func newModel(cfg *config.Config) Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.CharLimit = 256
	ti.Width = 60

	cwd, _ := os.Getwd()
	m := Model{
		cfg:    cfg,
		rows:   buildRows(cfg),
		cwd:    cwd,
		srcSel: map[string]bool{},
		tgtSel: map[string]bool{},
		input:  ti,
	}
	m.loadBinding()
	return m
}

func buildRows(cfg *config.Config) []brainRow {
	rows := make([]brainRow, 0, len(cfg.Brains))
	for _, b := range cfg.Brains {
		rows = append(rows, brainRow{name: b.Name, path: b.Path, active: b.Name == cfg.Active, loading: true})
	}
	return rows
}

// loadBinding pre-fills the scope selection from the effective binding for the cwd.
func (m *Model) loadBinding() {
	path, bnd, err := config.FindBinding()
	if err != nil || bnd == nil {
		m.bindingOrigin = "none"
		return
	}
	m.bindingOrigin = path
	for _, s := range bnd.Sources {
		m.srcSel[s] = true
	}
	targets := bnd.Targets
	if len(targets) == 0 {
		targets = bnd.Sources // effective default: writable = readable
	}
	for _, t := range targets {
		m.tgtSel[t] = true
	}
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, r := range m.rows {
		cmds = append(cmds, loadStats(r.name, r.path))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case statsMsg:
		for i := range m.rows {
			if m.rows[i].name == msg.name {
				m.rows[i].loading = false
				if msg.err != nil {
					m.rows[i].err = msg.err.Error()
				} else {
					m.rows[i].info = msg.info
					m.rows[i].notes = msg.notes
				}
			}
		}
		return m, nil

	case lintMsg:
		for i := range m.rows {
			if m.rows[i].name == msg.name {
				switch {
				case msg.err != nil:
					m.rows[i].lint = "err"
				case msg.pass:
					m.rows[i].lint = "ok"
				default:
					m.rows[i].lint = fmt.Sprintf("fail:%d", msg.findings)
				}
			}
		}
		return m, nil

	case syncMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("%s: sync error: %v", msg.name, msg.err)
		} else {
			m.status = fmt.Sprintf("%s: %s", msg.name, syncSummary(msg.res))
		}
		return m, loadStats(msg.name, m.pathFor(msg.name))

	case tea.KeyMsg:
		if m.mode != modeNormal {
			return m.updateInput(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab", "right", "l":
		if msg.String() == "l" && m.view == dashView {
			break // 'l' is lint on the dashboard
		}
		m.view = (m.view + 1) % 3
		m.clampCursor()
		return m, nil
	case "shift+tab", "left", "h":
		m.view = (m.view + 2) % 3
		m.clampCursor()
		return m, nil
	case "up", "k":
		m.moveCursor(-1)
		return m, nil
	case "down", "j":
		m.moveCursor(1)
		return m, nil
	}

	switch m.view {
	case dashView:
		return m.updateDashboard(msg)
	case brainsView:
		return m.updateBrains(msg)
	case scopeView:
		return m.updateScope(msg)
	}
	return m, nil
}

func (m Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s":
		if r, ok := m.current(); ok {
			m.status = r.name + ": syncing…"
			return m, syncBrain(r.name, r.path)
		}
	case "S":
		var cmds []tea.Cmd
		for _, r := range m.rows {
			cmds = append(cmds, syncBrain(r.name, r.path))
		}
		m.status = "syncing all brains…"
		return m, tea.Batch(cmds...)
	case "l":
		if r, ok := m.current(); ok {
			m.markLint(r.name, "…")
			return m, lintBrain(r.name, r.path)
		}
	case "L":
		var cmds []tea.Cmd
		for i := range m.rows {
			m.rows[i].lint = "…"
			cmds = append(cmds, lintBrain(m.rows[i].name, m.rows[i].path))
		}
		return m, tea.Batch(cmds...)
	case "r":
		var cmds []tea.Cmd
		for i := range m.rows {
			m.rows[i].loading = true
			cmds = append(cmds, loadStats(m.rows[i].name, m.rows[i].path))
		}
		m.status = "refreshing…"
		return m, tea.Batch(cmds...)
	case "enter":
		return m.setActive()
	}
	return m, nil
}

func (m Model) updateBrains(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "a":
		m.mode = modeAddName
		m.pendName = ""
		m.input.SetValue("")
		m.input.Placeholder = "brain name"
		return m, m.input.Focus()
	case "e":
		if r, ok := m.current(); ok {
			m.mode = modeRename
			m.input.SetValue(r.name)
			m.input.Placeholder = "new name"
			return m, m.input.Focus()
		}
	case "p":
		if r, ok := m.current(); ok {
			m.mode = modeSetPath
			m.input.SetValue(r.path)
			m.input.Placeholder = "path"
			return m, m.input.Focus()
		}
	case "d":
		if _, ok := m.current(); ok {
			m.mode = modeConfirmDelete
			return m, nil
		}
	case "enter":
		return m.setActive()
	}
	return m, nil
}

func (m Model) updateScope(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	r, ok := m.current()
	switch msg.String() {
	case " ":
		if ok {
			m.srcSel[r.name] = !m.srcSel[r.name]
		}
	case "t":
		if ok {
			m.tgtSel[r.name] = !m.tgtSel[r.name]
		}
	case "w", "enter":
		m.status = m.writeScope()
		return m, nil
	case "c":
		m.status = m.clearScope()
		return m, nil
	}
	return m, nil
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			m.deleteSelected()
			m.mode = modeNormal
		case "n", "N", "esc", "q":
			m.mode = modeNormal
			m.status = "cancelled"
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.status = "cancelled"
		return m, nil
	case "enter":
		return m.commitInput()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) commitInput() (tea.Model, tea.Cmd) {
	val := m.input.Value()
	switch m.mode {
	case modeAddName:
		if val == "" {
			m.status = "name required"
			m.mode = modeNormal
			return m, nil
		}
		m.pendName = val
		m.mode = modeAddPath
		m.input.SetValue("")
		m.input.Placeholder = "path to brain directory"
		return m, m.input.Focus()

	case modeAddPath:
		m.mode = modeNormal
		abs, _ := filepath.Abs(val)
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			m.status = "not a directory: " + abs
			return m, nil
		}
		m.cfg.Add(config.Brain{Name: m.pendName, Path: abs})
		if m.cfg.Active == "" {
			m.cfg.Active = m.pendName
		}
		if err := m.cfg.Save(); err != nil {
			m.status = "save failed: " + err.Error()
			return m, nil
		}
		m.rows = buildRows(m.cfg)
		m.status = "added " + m.pendName
		return m, loadStats(m.pendName, abs)

	case modeRename:
		m.mode = modeNormal
		r, ok := m.current()
		if !ok || val == "" {
			return m, nil
		}
		old := r.name
		if e := m.cfg.Find(old); e != nil {
			e.Name = val
		}
		if m.cfg.Active == old {
			m.cfg.Active = val
		}
		if err := m.cfg.Save(); err != nil {
			m.status = "save failed: " + err.Error()
			return m, nil
		}
		m.rows = buildRows(m.cfg)
		m.status = fmt.Sprintf("renamed %s → %s (update any .multi.yaml referencing %q)", old, val, old)
		return m, nil

	case modeSetPath:
		m.mode = modeNormal
		r, ok := m.current()
		if !ok || val == "" {
			return m, nil
		}
		abs, _ := filepath.Abs(val)
		if e := m.cfg.Find(r.name); e != nil {
			e.Path = abs
		}
		if err := m.cfg.Save(); err != nil {
			m.status = "save failed: " + err.Error()
			return m, nil
		}
		m.rows = buildRows(m.cfg)
		m.status = "set path for " + r.name
		return m, loadStats(r.name, abs)
	}
	m.mode = modeNormal
	return m, nil
}

func (m Model) setActive() (tea.Model, tea.Cmd) {
	r, ok := m.current()
	if !ok {
		return m, nil
	}
	m.cfg.Active = r.name
	if err := m.cfg.Save(); err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}
	for i := range m.rows {
		m.rows[i].active = m.rows[i].name == r.name
	}
	m.status = "active brain: " + r.name
	return m, nil
}

func (m *Model) deleteSelected() {
	r, ok := m.current()
	if !ok {
		return
	}
	kept := m.cfg.Brains[:0]
	for _, b := range m.cfg.Brains {
		if b.Name != r.name {
			kept = append(kept, b)
		}
	}
	m.cfg.Brains = kept
	if m.cfg.Active == r.name {
		m.cfg.Active = ""
	}
	if err := m.cfg.Save(); err != nil {
		m.status = "save failed: " + err.Error()
		return
	}
	m.rows = buildRows(m.cfg)
	m.clampCursor()
	m.status = "removed " + r.name
}

func (m Model) writeScope() string {
	var sources, targets []string
	for _, r := range m.rows {
		if m.srcSel[r.name] {
			sources = append(sources, r.name)
		}
		if m.tgtSel[r.name] {
			targets = append(targets, r.name)
		}
	}
	if len(sources) == 0 && len(targets) == 0 {
		return "select at least one source (space) before saving"
	}
	p, err := config.WriteBinding(m.cwd, config.Binding{Sources: sources, Targets: targets})
	if err != nil {
		return "write failed: " + err.Error()
	}
	return "bound " + p
}

func (m Model) clearScope() string {
	p := filepath.Join(m.cwd, config.BindingFile)
	if err := os.Remove(p); err != nil {
		return "nothing to clear here"
	}
	for k := range m.srcSel {
		delete(m.srcSel, k)
	}
	for k := range m.tgtSel {
		delete(m.tgtSel, k)
	}
	return "removed " + p
}

func (m *Model) markLint(name, v string) {
	for i := range m.rows {
		if m.rows[i].name == name {
			m.rows[i].lint = v
		}
	}
}

func (m Model) current() (brainRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return brainRow{}, false
	}
	return m.rows[m.cursor], true
}

func (m Model) pathFor(name string) string {
	for _, r := range m.rows {
		if r.name == name {
			return r.path
		}
	}
	return ""
}

func (m *Model) moveCursor(d int) {
	if len(m.rows) == 0 {
		return
	}
	m.cursor = (m.cursor + d + len(m.rows)) % len(m.rows)
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func syncSummary(res brain.SyncResult) string {
	var parts []string
	if res.Committed {
		parts = append(parts, "committed")
	}
	if res.Pulled {
		parts = append(parts, "pulled")
	}
	if res.Pushed {
		parts = append(parts, "pushed")
	}
	if len(parts) == 0 {
		parts = append(parts, "nothing to do")
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	if res.Note != "" {
		out += " (" + res.Note + ")"
	}
	return out
}
