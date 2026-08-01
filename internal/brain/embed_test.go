package brain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEmbedServer returns axis-aligned vectors keyed by marker words so cosine
// ranking is fully predictable, and counts embedded texts.
func fakeEmbedServer(t *testing.T, embedded *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var req struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		*embedded += len(req.Input)
		type datum struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		var data []datum
		for i, text := range req.Input {
			v := []float32{0, 0, 1}
			switch {
			case strings.Contains(text, "alpha"):
				v = []float32{1, 0, 0}
			case strings.Contains(text, "beta"):
				v = []float32{0, 1, 0}
			}
			data = append(data, datum{Index: i, Embedding: v})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
}

func embedBrain(t *testing.T, url string) *Brain {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	b := newBrain(t)
	b.Settings.Embeddings = &EmbedSettings{BaseURL: url + "/v1", Model: "test-model", BatchSize: 2}
	mk := func(title, summary string) {
		t.Helper()
		if _, err := b.Write(WriteParams{Title: title, Dir: "notes", Summary: summary, Tags: []string{"domain"}}); err != nil {
			t.Fatal(err)
		}
	}
	mk("Alpha Topic", "all about alpha things")
	mk("Beta Topic", "all about beta things")
	mk("Gamma Topic", "something else entirely")
	return b
}

func TestReindexAndSimilar(t *testing.T) {
	var embedded int
	srv := fakeEmbedServer(t, &embedded)
	defer srv.Close()
	b := embedBrain(t, srv.URL)

	notes, err := b.Notes()
	if err != nil {
		t.Fatal(err)
	}
	total := len(notes) // includes the scaffold notes Init creates

	stats, err := b.Reindex(false, nil)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if stats.Embedded != total || stats.Kept != 0 {
		t.Fatalf("first reindex should embed all %d notes: %+v", total, stats)
	}

	// Second run: nothing changed, nothing re-embedded.
	before := embedded
	stats, err = b.Reindex(false, nil)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if stats.Embedded != 0 || stats.Kept != total || embedded != before {
		t.Fatalf("incremental reindex should embed nothing: %+v (requests %d→%d)", stats, before, embedded)
	}

	hits, err := b.Similar("alpha", 2)
	if err != nil {
		t.Fatalf("similar: %v", err)
	}
	if len(hits) != 2 || hits[0].Path != "notes/alpha-topic.md" {
		t.Fatalf("expected the alpha note first, got %+v", hits)
	}
	if hits[0].Score < 0.99 {
		t.Fatalf("exact-axis match should score ~1, got %f", hits[0].Score)
	}
	if hits[0].Summary == "" {
		t.Fatalf("similar hits should carry front matter, got %+v", hits[0])
	}

	// Neighbor lookup excludes the note itself.
	neighbors, err := b.SimilarNote("notes/alpha-topic.md", 2)
	if err != nil {
		t.Fatalf("similar note: %v", err)
	}
	for _, n := range neighbors {
		if n.Path == "notes/alpha-topic.md" {
			t.Fatalf("neighbors must exclude the note itself: %+v", neighbors)
		}
	}

	// Editing a note re-embeds exactly that note; deleting one drops its entry.
	if err := b.Append("notes/alpha-topic.md", "now with more detail"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(b.Root, "notes/gamma-topic.md")); err != nil {
		t.Fatal(err)
	}
	stats, err = b.Reindex(false, nil)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if stats.Embedded != 1 || stats.Kept != total-2 || stats.Removed != 1 {
		t.Fatalf("expected embed 1 / keep %d / remove 1, got %+v", total-2, stats)
	}
}

func TestSimilarWithoutConfigOrIndex(t *testing.T) {
	b := newBrain(t)
	if _, err := b.Similar("anything", 5); err != ErrNoEmbeddings {
		t.Fatalf("expected ErrNoEmbeddings, got %v", err)
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	srv := fakeEmbedServer(t, new(int))
	defer srv.Close()
	b.Settings.Embeddings = &EmbedSettings{BaseURL: srv.URL + "/v1", Model: "test-model"}
	if _, err := b.Similar("anything", 5); err != ErrNoIndex {
		t.Fatalf("expected ErrNoIndex, got %v", err)
	}
}
