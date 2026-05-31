package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/mxcd/multiverse/internal/config"
)

func send(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func rune1(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestViewSwitchAndScopeBinding(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	a, _ := brain.Init(t.TempDir(), brain.Settings{Name: "alpha"}, false)
	b, _ := brain.Init(t.TempDir(), brain.Settings{Name: "beta"}, false)
	cfg := &config.Config{
		Active: "alpha",
		Brains: []config.Brain{{Name: "alpha", Path: a.Root}, {Name: "beta", Path: b.Root}},
	}

	m := newModel(cfg)
	if m.view != dashView {
		t.Fatalf("expected dashView, got %d", m.view)
	}

	// tab → Brains, tab → Scope
	m = send(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.view != brainsView {
		t.Fatalf("expected brainsView after tab, got %d", m.view)
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.view != scopeView {
		t.Fatalf("expected scopeView, got %d", m.view)
	}

	// cursor on alpha: toggle source (space) and target (t)
	m = send(m, tea.KeyMsg{Type: tea.KeySpace})
	m = send(m, rune1('t'))
	if !m.srcSel["alpha"] || !m.tgtSel["alpha"] {
		t.Fatalf("alpha should be selected as source and target: src=%v tgt=%v", m.srcSel, m.tgtSel)
	}
	// move down to beta, add it as a read-only source
	m = send(m, tea.KeyMsg{Type: tea.KeyDown})
	m = send(m, tea.KeyMsg{Type: tea.KeySpace})
	if !m.srcSel["beta"] || m.tgtSel["beta"] {
		t.Fatalf("beta should be source-only: src=%v tgt=%v", m.srcSel, m.tgtSel)
	}

	// save → writes ./.multi.yaml
	m = send(m, rune1('w'))
	bnd, err := config.ReadBindingAt(dir)
	if err != nil || bnd == nil {
		t.Fatalf("binding not written: %v", err)
	}
	if len(bnd.Sources) != 2 || len(bnd.Targets) != 1 || bnd.Targets[0] != "alpha" {
		t.Fatalf("unexpected binding: %+v", bnd)
	}
}

func TestViewRenders(t *testing.T) {
	t.Chdir(t.TempDir())
	a, _ := brain.Init(t.TempDir(), brain.Settings{Name: "alpha"}, false)
	cfg := &config.Config{Active: "alpha", Brains: []config.Brain{{Name: "alpha", Path: a.Root}}}
	m := newModel(cfg)
	for _, v := range []viewID{dashView, brainsView, scopeView} {
		m.view = v
		out := m.View()
		if out == "" {
			t.Fatalf("view %d rendered empty", v)
		}
	}
}

func TestSetActiveFromBrainsView(t *testing.T) {
	t.Chdir(t.TempDir())
	a, _ := brain.Init(t.TempDir(), brain.Settings{Name: "alpha"}, false)
	b, _ := brain.Init(t.TempDir(), brain.Settings{Name: "beta"}, false)
	cfgDir := t.TempDir()
	t.Setenv("MULTI_CONFIG_DIR", cfgDir)
	cfg := &config.Config{Active: "alpha", Brains: []config.Brain{{Name: "alpha", Path: a.Root}, {Name: "beta", Path: b.Root}}}

	m := newModel(cfg)
	m = send(m, tea.KeyMsg{Type: tea.KeyTab}) // brains view
	m = send(m, tea.KeyMsg{Type: tea.KeyDown})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // activate beta
	if m.cfg.Active != "beta" {
		t.Fatalf("expected active beta, got %q", m.cfg.Active)
	}
}
