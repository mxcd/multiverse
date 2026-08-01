package brain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func taxBrain(t *testing.T) *Brain {
	t.Helper()
	b, err := Init(t.TempDir(), Settings{
		Name:     "tax",
		Taxonomy: Taxonomy{Types: []string{"reference", "pattern"}, Statuses: []string{"active", "draft"}},
	}, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	return b
}

func TestWriteRejectsUnknownType(t *testing.T) {
	b := taxBrain(t)
	_, err := b.Write(WriteParams{Title: "X", Dir: "d", Summary: "s", Type: "bogus"})
	if err == nil || !strings.Contains(err.Error(), "taxonomy") {
		t.Fatalf("expected taxonomy rejection, got %v", err)
	}
	if _, err := b.Write(WriteParams{Title: "X", Dir: "d", Summary: "s", Type: "pattern", Status: "draft"}); err != nil {
		t.Fatalf("allowed values should pass: %v", err)
	}
}

func TestLintFlagsTaxonomyDrift(t *testing.T) {
	b := taxBrain(t)
	bad := "---\ntype: gotcha\nstatus: someday\nsummary: s\n---\n# Bad\n"
	if err := os.MkdirAll(filepath.Join(b.Root, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.Root, "d", "bad.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := b.Lint(LintOptions{Taxonomy: true})
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	if len(rep.Findings) != 2 {
		t.Fatalf("expected type+status findings, got %+v", rep.Findings)
	}
}

func TestOpenTaxonomyAcceptsAnything(t *testing.T) {
	b := newBrain(t) // no taxonomy configured
	if _, err := b.Write(WriteParams{Title: "X", Dir: "d", Summary: "s", Type: "whatever", Tags: []string{"domain"}}); err != nil {
		t.Fatalf("open taxonomy should accept any type: %v", err)
	}
}
