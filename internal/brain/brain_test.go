package brain

import (
	"errors"
	"strings"
	"testing"
)

func newBrain(t *testing.T) *Brain {
	t.Helper()
	b, err := Init(t.TempDir(), Settings{Name: "test", Split: []string{"domain", "operations"}}, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	return b
}

func TestWriteRequiresSummary(t *testing.T) {
	b := newBrain(t)
	if _, err := b.Write(WriteParams{Title: "No Summary", Tags: []string{"domain"}}); !errors.Is(err, ErrSummaryRequired) {
		t.Fatalf("expected ErrSummaryRequired, got %v", err)
	}
}

func TestWriteThenLoadRoundTrip(t *testing.T) {
	b := newBrain(t)
	rel, err := b.Write(WriteParams{
		Title:     "Mediation Basics",
		Dir:       "domain",
		Summary:   "what mediation is: with a colon",
		Tags:      []string{"domain", "mediation"},
		Source:    "manual",
		Freshness: "current",
		Body:      "See [[The QVM Standard]].",
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if rel != "domain/Mediation Basics.md" {
		t.Fatalf("unexpected path %q", rel)
	}
	n, err := b.Load(rel)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if n.FM.Summary != "what mediation is: with a colon" {
		t.Errorf("summary round-trip failed: %q", n.FM.Summary)
	}
	if n.FM.Created == "" || n.FM.Retrieved == "" {
		t.Errorf("dates not auto-filled: %+v", n.FM)
	}
	if got := strings.Join(n.FM.Tags, ","); got != "domain,mediation" {
		t.Errorf("tags round-trip failed: %q", got)
	}
}

func TestResolveAmbiguous(t *testing.T) {
	b := newBrain(t)
	mk := func(dir string) {
		if _, err := b.Write(WriteParams{Title: "Dup", Dir: dir, Summary: "s", Tags: []string{"domain"}}); err != nil {
			t.Fatal(err)
		}
	}
	mk("domain")
	mk("operations")
	if _, err := b.Resolve("Dup"); err == nil {
		t.Fatal("expected ambiguous error")
	} else {
		var amb *AmbiguousError
		if !errors.As(err, &amb) {
			t.Fatalf("expected AmbiguousError, got %v", err)
		}
	}
	if rel, err := b.Resolve("domain/Dup.md"); err != nil || rel != "domain/Dup.md" {
		t.Fatalf("exact path should resolve: %q %v", rel, err)
	}
}

func TestLintRules(t *testing.T) {
	b := newBrain(t)
	// content note missing provenance: passes summary, fails fresh.
	if _, err := b.Write(WriteParams{Title: "Naked", Dir: "domain", Summary: "no provenance", Tags: []string{"domain"}}); err != nil {
		t.Fatal(err)
	}
	// content note missing a split tag.
	if _, err := b.Write(WriteParams{Title: "Untagged", Dir: "domain", Summary: "no split tag", Source: "x", Freshness: "y"}); err != nil {
		t.Fatal(err)
	}
	rep, err := b.Lint(AllRules())
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]int{}
	for _, f := range rep.Findings {
		rules[f.Rule]++
	}
	if rules["fresh"] == 0 {
		t.Error("expected a freshness finding for Naked")
	}
	if rules["tags"] == 0 {
		t.Error("expected a split-tag finding for Untagged")
	}
	if rules["summary"] != 0 {
		t.Errorf("did not expect summary findings, got %d", rules["summary"])
	}
}

func TestBacklinks(t *testing.T) {
	b := newBrain(t)
	if _, err := b.Write(WriteParams{Title: "Source", Dir: "domain", Summary: "s", Tags: []string{"domain"}, Source: "x", Freshness: "y", Body: "links to [[Target]]"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write(WriteParams{Title: "Target", Dir: "domain", Summary: "t", Tags: []string{"domain"}, Source: "x", Freshness: "y"}); err != nil {
		t.Fatal(err)
	}
	back, err := b.Backlinks("domain/Target.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back[0] != "domain/Source.md" {
		t.Fatalf("unexpected backlinks: %v", back)
	}
}
