package brain

import "testing"

func TestWakeup(t *testing.T) {
	b := newBrain(t)
	if _, err := b.Write(WriteParams{Title: "Identity", Dir: "meta", Summary: "who we are", Body: "Work like a good engineer.", Tags: []string{"domain"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write(WriteParams{Title: "Pinned Fact", Dir: "facts", Summary: "the one thing to remember", Pinned: true, Tags: []string{"domain"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write(WriteParams{Title: "Noise", Dir: "facts", Summary: "not pinned", Tags: []string{"domain"}}); err != nil {
		t.Fatal(err)
	}
	b.Settings.Wakeup = []string{"Identity"}

	sections, facts, err := b.Wakeup()
	if err != nil {
		t.Fatalf("wakeup: %v", err)
	}
	if len(sections) != 1 || sections[0].Path != "meta/identity.md" {
		t.Fatalf("expected the identity section, got %+v", sections)
	}
	if len(facts) != 1 || facts[0].Path != "facts/pinned-fact.md" {
		t.Fatalf("expected exactly the pinned fact, got %+v", facts)
	}
}
