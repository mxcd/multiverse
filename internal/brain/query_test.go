package brain

import "testing"

func seedSearchNotes(t *testing.T) *Brain {
	t.Helper()
	b := newBrain(t)
	mk := func(title, dir, summary, body string, tags ...string) {
		t.Helper()
		if _, err := b.Write(WriteParams{Title: title, Dir: dir, Summary: summary, Body: body, Tags: append(tags, "domain")}); err != nil {
			t.Fatalf("write %s: %v", title, err)
		}
	}
	mk("Websocket Reconnect Pattern", "patterns", "exponential backoff for websocket reconnects", "details")
	mk("Vue Composables", "patterns", "extracting shared vue logic", "uses a websocket internally", "vue")
	mk("Unrelated", "references", "nothing to see", "still nothing")
	return b
}

func TestSearchRequiresAllTerms(t *testing.T) {
	b := seedSearchNotes(t)
	hits, err := b.Search("websocket reconnect", false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Path != "patterns/websocket-reconnect-pattern.md" {
		t.Fatalf("expected the reconnect note only, got %+v", hits)
	}
}

func TestSearchRanksPathHitsAboveBodyHits(t *testing.T) {
	b := seedSearchNotes(t)
	hits, err := b.Search("websocket", true)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %+v", hits)
	}
	if hits[0].Path != "patterns/websocket-reconnect-pattern.md" {
		t.Fatalf("path+summary hit should outrank body hit, got %+v", hits)
	}
	if hits[0].Score <= hits[1].Score {
		t.Fatalf("scores not descending: %+v", hits)
	}
}

func TestSearchBodyOptIn(t *testing.T) {
	b := seedSearchNotes(t)
	hits, err := b.Search("internally", false)
	if err != nil || len(hits) != 0 {
		t.Fatalf("body should not match without --body, got %+v (%v)", hits, err)
	}
	hits, err = b.Search("internally", true)
	if err != nil || len(hits) != 1 {
		t.Fatalf("body should match with --body, got %+v (%v)", hits, err)
	}
}
