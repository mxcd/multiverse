package brain

import "testing"

func TestNearDuplicatesFindsRewordedNote(t *testing.T) {
	b := newBrain(t)
	if _, err := b.Write(WriteParams{
		Title:   "Go Basicauth Session Handling",
		Dir:     "patterns",
		Summary: "session cookie handling in go-basicauth middleware",
		Tags:    []string{"domain"},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	dups, err := b.NearDuplicates("Go Basicauth Session Cookies", "handling session cookies in go-basicauth", 0.6, 3)
	if err != nil {
		t.Fatalf("near duplicates: %v", err)
	}
	if len(dups) != 1 || dups[0].Path != "patterns/go-basicauth-session-handling.md" {
		t.Fatalf("expected the existing note as a duplicate, got %+v", dups)
	}

	dups, err = b.NearDuplicates("Vue Transition Hooks", "animating list entries in vue", 0.6, 3)
	if err != nil {
		t.Fatalf("near duplicates: %v", err)
	}
	if len(dups) != 0 {
		t.Fatalf("unrelated note should not match, got %+v", dups)
	}
}
