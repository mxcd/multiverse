package cli

import (
	"testing"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/mxcd/multiverse/internal/config"
)

func mkBrain(t *testing.T, name string) string {
	t.Helper()
	b, err := brain.Init(t.TempDir(), brain.Settings{Name: name, Split: []string{"domain"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	return b.Root
}

func TestBuildScopeSourcesAndTargets(t *testing.T) {
	a := mkBrain(t, "alpha")
	b := mkBrain(t, "beta")
	cfg := &config.Config{}

	// sources = both, targets = alpha only (read both, write one)
	sc, err := buildScope(cfg, &config.Binding{Sources: []string{a, b}, Targets: []string{a}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Sources) != 2 || !sc.multiSource() {
		t.Fatalf("expected 2 sources, got %d", len(sc.Sources))
	}
	target, err := sc.writeTarget()
	if err != nil {
		t.Fatal(err)
	}
	if target.Root != a {
		t.Fatalf("write target should be alpha, got %s", target.Name)
	}
}

func TestTargetsDefaultToSources(t *testing.T) {
	a := mkBrain(t, "alpha")
	sc, err := buildScope(&config.Config{}, &config.Binding{Sources: []string{a}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Targets) != 1 || sc.Targets[0].Root != a {
		t.Fatalf("targets should default to sources, got %+v", sc.Targets)
	}
}

func TestReadOnlyBindingHasNoTargets(t *testing.T) {
	a := mkBrain(t, "alpha")
	// ReadOnly: sources only, no write target.
	sc, err := buildScope(&config.Config{}, &config.Binding{Sources: []string{a}, ReadOnly: true}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sc.Sources))
	}
	if len(sc.Targets) != 0 {
		t.Fatalf("read-only scope must have no targets, got %d", len(sc.Targets))
	}
	if _, err := sc.writeTarget(); err == nil {
		t.Fatal("writeTarget must error under a read-only scope")
	}
}

func TestResolveNoteAcrossBrains(t *testing.T) {
	a := mkBrain(t, "alpha")
	b := mkBrain(t, "beta")
	sc, err := buildScope(&config.Config{}, &config.Binding{Sources: []string{a, b}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	// write a uniquely-named note into beta and resolve it from scope
	if _, err := sc.Targets[1].Write(brain.WriteParams{Title: "OnlyInBeta", Dir: "domain", Summary: "s", Tags: []string{"domain"}, Source: "x", Freshness: "y"}); err != nil {
		t.Fatal(err)
	}
	sb, rel, err := sc.resolveNote("OnlyInBeta")
	if err != nil {
		t.Fatal(err)
	}
	if sb.Name != "beta" || rel != "domain/OnlyInBeta.md" {
		t.Fatalf("resolved to wrong brain/path: %s %s", sb.Name, rel)
	}

	// a name present in both brains is ambiguous
	for _, sbb := range sc.Sources {
		if _, err := sbb.Write(brain.WriteParams{Title: "Shared", Dir: "domain", Summary: "s", Tags: []string{"domain"}, Source: "x", Freshness: "y"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := sc.resolveNote("Shared"); err == nil {
		t.Fatal("expected cross-brain ambiguity error")
	}
	// brain:note qualifier disambiguates
	if sb, _, err := sc.resolveNote("alpha:Shared"); err != nil || sb.Name != "alpha" {
		t.Fatalf("qualified resolve failed: %v", err)
	}
}
