package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mxcd/multiverse/internal/config"
)

func loadConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestBrainAddWithAliases(t *testing.T) {
	newRegistry(t)
	dt := mkBrain(t, "deep-thought")
	if err := runMulti(t, "brain", "add", dt, "--alias", "dt,thought", "--alias", "brain"); err != nil {
		t.Fatal(err)
	}
	// re-adding the brain merges new aliases
	if err := runMulti(t, "brain", "add", dt, "--alias", "dt", "--alias", "second"); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig(t)
	if b := cfg.Find("dt"); b == nil || b.Name != "deep-thought" || !slices.Equal(b.Aliases, []string{"dt", "thought", "brain", "second"}) || len(cfg.Brains) != 1 {
		t.Fatalf("expected deep-thought with its aliases, got %+v", cfg.Brains)
	}
}

func TestBrainUseAliasStoresName(t *testing.T) {
	newRegistry(t)
	if err := runMulti(t, "brain", "add", mkBrain(t, "alpha")); err != nil {
		t.Fatal(err)
	}
	if err := runMulti(t, "brain", "add", mkBrain(t, "deep-thought"), "--alias", "dt"); err != nil {
		t.Fatal(err)
	}
	if err := runMulti(t, "brain", "use", "dt"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readConfig(t), "active: deep-thought\n") {
		t.Fatalf("brain use by alias must store the name:\n%s", readConfig(t))
	}
	if err := runMulti(t, "brain", "use", "unknown"); err == nil {
		t.Fatal("brain use of an unknown brain must error")
	}
}

func TestBrainAliasUnalias(t *testing.T) {
	newRegistry(t)
	if err := runMulti(t, "brain", "add", mkBrain(t, "deep-thought")); err != nil {
		t.Fatal(err)
	}
	if err := runMulti(t, "brain", "alias", "deep-thought", "dt", "thought"); err != nil {
		t.Fatal(err)
	}
	// the brain is addressable by any of its aliases
	if err := runMulti(t, "brain", "alias", "dt", "brain"); err != nil {
		t.Fatal(err)
	}
	if got := loadConfig(t).Find("deep-thought").Aliases; !slices.Equal(got, []string{"dt", "thought", "brain"}) {
		t.Fatalf("aliases not added, got %v", got)
	}
	if err := runMulti(t, "brain", "unalias", "thought", "brain"); err != nil {
		t.Fatal(err)
	}
	if got := loadConfig(t).Find("deep-thought").Aliases; !slices.Equal(got, []string{"dt"}) {
		t.Fatalf("aliases not removed, got %v", got)
	}

	before := readConfig(t)
	for _, args := range [][]string{
		{"alias", "dt", "DT"},
		{"alias", "dt", "deep/thought"},
		{"alias", "unknown", "x"},
		{"alias", "dt"},
		{"unalias", "dt", "thought"},
		{"unalias", "deep-thought"},
	} {
		if err := runMulti(t, append([]string{"brain"}, args...)...); err == nil {
			t.Fatalf("brain %v: expected an error", args)
		}
	}
	if readConfig(t) != before {
		t.Fatalf("rejected alias changes must not be written:\n%s", readConfig(t))
	}

	if err := runMulti(t, "brain", "unalias", "dt"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(readConfig(t), "aliases") {
		t.Fatalf("a brain without aliases stores no aliases key:\n%s", readConfig(t))
	}
}

func TestBrainNamesAreUnique(t *testing.T) {
	newRegistry(t)
	pdb := mkBrain(t, "pdb-brain")
	if err := runMulti(t, "brain", "add", mkBrain(t, "deep-thought"), "--alias", "dt"); err != nil {
		t.Fatal(err)
	}
	if err := runMulti(t, "brain", "add", pdb); err != nil {
		t.Fatal(err)
	}
	before := readConfig(t)

	// an alias never equals another brain's alias ...
	if err := runMulti(t, "brain", "alias", "pdb-brain", "dt"); err == nil || !strings.Contains(err.Error(), `already belongs to brain "deep-thought"`) {
		t.Fatalf("alias with a taken alias: expected a conflict naming the owner, got %v", err)
	}
	if err := runMulti(t, "brain", "add", pdb, "--alias", "pdb", "--alias", "dt"); err == nil || !strings.Contains(err.Error(), `already belongs to brain "deep-thought"`) {
		t.Fatalf("add with a taken alias: expected a conflict naming the owner, got %v", err)
	}
	// ... nor any brain name, its own included
	for _, name := range []string{"pdb-brain", "deep-thought"} {
		if err := runMulti(t, "brain", "alias", "dt", name); err == nil || !strings.Contains(err.Error(), `is already a brain name`) {
			t.Fatalf("alias equal to brain name %s: expected a conflict, got %v", name, err)
		}
	}
	// a brain name never equals an alias, on every registration path
	if err := runMulti(t, "brain", "add", mkBrain(t, "other"), "--name", "dt"); err == nil || !strings.Contains(err.Error(), `already an alias of brain "deep-thought"`) {
		t.Fatalf("add under an alias: expected a conflict naming the owner, got %v", err)
	}
	fresh := filepath.Join(t.TempDir(), "fresh")
	if err := runMulti(t, "init", fresh, "--name", "dt", "--no-git"); err == nil || !strings.Contains(err.Error(), `already an alias of brain "deep-thought"`) {
		t.Fatalf("init under an alias: expected a conflict naming the owner, got %v", err)
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Fatalf("a rejected init must not scaffold a brain: %v", err)
	}
	if err := runMulti(t, "clone", "file:///no/such/repo.git", fresh, "--name", "dt"); err == nil || !strings.Contains(err.Error(), `already an alias of brain "deep-thought"`) {
		t.Fatalf("clone under an alias: expected a conflict before cloning, got %v", err)
	}
	if readConfig(t) != before {
		t.Fatalf("rejected writes must not be stored:\n%s", readConfig(t))
	}
}

func TestBrainListShowsAliases(t *testing.T) {
	newRegistry(t)
	if err := runMulti(t, "brain", "add", mkBrain(t, "a")); err != nil {
		t.Fatal(err)
	}
	if err := runMulti(t, "brain", "add", mkBrain(t, "deep-thought"), "--alias", "dt,thought"); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig(t)
	out := captureStdout(t, func() {
		if err := runMulti(t, "brain", "list"); err != nil {
			t.Fatal(err)
		}
	})
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "* a ") || !strings.HasPrefix(lines[1], "  deep-thought ") {
		t.Fatalf("expected one line per brain, the active one marked, got:\n%s", out)
	}
	if strings.Index(lines[0], "- ") != strings.Index(lines[1], "dt,thought") || strings.Index(lines[0], cfg.Brains[0].Path) != strings.Index(lines[1], cfg.Brains[1].Path) {
		t.Fatalf("columns not aligned:\n%s", out)
	}
}
