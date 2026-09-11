package brain

import "testing"

func seedExploreNotes(t *testing.T) *Brain {
	t.Helper()
	b := newBrain(t)
	mk := func(title, summary, body string, typ string) {
		t.Helper()
		if _, err := b.Write(WriteParams{Title: title, Dir: "patterns", Type: typ, Summary: summary, Body: body, Tags: []string{"domain"}}); err != nil {
			t.Fatalf("write %s: %v", title, err)
		}
	}
	mk("Websocket Reconnect", "backoff for websocket reconnects", "see [[Heartbeat Ping]] and [[Realtime MOC]]", "pattern")
	mk("Heartbeat Ping", "keepalive pings on idle sockets", "pairs with [[Socket Close Codes]]", "pattern")
	mk("Socket Close Codes", "which close codes mean retry", "two hops out", "reference")
	mk("Realtime MOC", "map of realtime notes", "[[Heartbeat Ping]] [[Unrelated Hub Child]]", "moc")
	mk("Unrelated Hub Child", "only reachable through the moc", "nothing", "reference")
	return b
}

func TestExploreSeedsThenOneHop(t *testing.T) {
	b := seedExploreNotes(t)
	hits, err := b.Explore([]string{"websocket reconnect"}, ExploreOptions{Top: 5, Hops: 1})
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	got := map[string]int{}
	for _, h := range hits {
		got[h.Path] = h.Hop
	}
	if got["patterns/websocket-reconnect.md"] != 0 || len(hits) == 0 || hits[0].Path != "patterns/websocket-reconnect.md" {
		t.Fatalf("seed should rank first at hop 0, got %+v", hits)
	}
	if got["patterns/heartbeat-ping.md"] != 1 || got["patterns/realtime-moc.md"] != 1 {
		t.Fatalf("direct links should be hop 1, got %+v", got)
	}
	if _, ok := got["patterns/socket-close-codes.md"]; ok {
		t.Fatalf("two hops away must not be reached with Hops=1: %+v", got)
	}
	if _, ok := got["patterns/unrelated-hub-child.md"]; ok {
		t.Fatalf("a moc is reached but never expanded: %+v", got)
	}
}

func TestExploreSecondHopAndNoMatch(t *testing.T) {
	b := seedExploreNotes(t)
	hits, err := b.Explore([]string{"websocket reconnect"}, ExploreOptions{Top: 5, Hops: 2})
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	found := false
	for _, h := range hits {
		if h.Path == "patterns/socket-close-codes.md" && h.Hop == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("Hops=2 should reach the second hop, got %+v", hits)
	}
	none, err := b.Explore([]string{"nothing matches this"}, ExploreOptions{Top: 5, Hops: 1})
	if err != nil || len(none) != 0 {
		t.Fatalf("no seeds should yield no hits, got %+v (%v)", none, err)
	}
}
