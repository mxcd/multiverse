package tui

import (
	"slices"
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

// typeLine replaces the input's text and submits it.
func typeLine(m Model, s string) Model {
	m.input.SetValue(s)
	return send(m, tea.KeyMsg{Type: tea.KeyEnter})
}

func TestRegistryKeepsAliases(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("MULTI_CONFIG_DIR", t.TempDir())
	a, _ := brain.Init(t.TempDir(), brain.Settings{Name: "alpha"}, false)
	b, _ := brain.Init(t.TempDir(), brain.Settings{Name: "beta"}, false)
	cfg, err := config.Load() // a loaded config saves, so rows are rebuilt after each change
	if err != nil {
		t.Fatal(err)
	}
	cfg.Active = "alpha"
	cfg.Brains = []config.Brain{
		{Name: "alpha", Path: a.Root, Aliases: []string{"a", "first"}},
		{Name: "beta", Path: b.Root, Aliases: []string{"b"}},
	}
	m := newModel(cfg)
	m = send(m, tea.KeyMsg{Type: tea.KeyTab}) // brains view, cursor on alpha

	// renaming to another brain's alias or name is rejected
	for _, name := range []string{"b", "beta"} {
		m = typeLine(send(m, rune1('e')), name)
		if m.cfg.Find("alpha") == nil || m.cfg.Find(name).Name != "beta" {
			t.Fatalf("rename to %q must be rejected: %+v", name, m.cfg.Brains)
		}
	}
	// a rename keeps the aliases; renaming to one of them promotes it
	m = typeLine(send(m, rune1('e')), "first")
	if e := m.cfg.Find("a"); e == nil || e.Name != "first" || len(e.Aliases) != 1 || m.cfg.Active != "first" {
		t.Fatalf("rename should keep the other aliases and follow active: %+v", m.cfg.Brains)
	}
	// adding a brain under an alias is rejected
	m = typeLine(send(m, rune1('a')), "b")
	if m.mode != modeNormal || m.pendName != "" {
		t.Fatalf("add under an alias must be rejected, got mode %d pending %q", m.mode, m.pendName)
	}
	// deleting a brain frees its aliases
	m = send(m, tea.KeyMsg{Type: tea.KeyDown})
	m = send(send(m, rune1('d')), rune1('y'))
	if m.cfg.Find("beta") != nil || m.cfg.Find("b") != nil {
		t.Fatalf("delete should drop the brain and its aliases: %+v", m.cfg.Brains)
	}
	if saved, err := config.Load(); err != nil || len(saved.Brains) != 1 || !slices.Equal(saved.Brains[0].Aliases, []string{"a"}) {
		t.Fatalf("registry not saved as expected: %+v (%v)", saved, err)
	}
}
