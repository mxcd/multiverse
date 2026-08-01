package brain

import (
	"sort"
	"strings"
)

// NoteInfo is the lightweight, front-matter-only view of a note used by index,
// search and find — so callers (and agents) judge relevance without reading bodies.
// Brain is the owning brain's display name; it is stamped by the scope layer when
// results span multiple brains and is empty for single-brain queries.
// Score is set by ranked operations (search, similar) and zero elsewhere.
type NoteInfo struct {
	Brain   string   `json:"brain,omitempty"`
	Path    string   `json:"path"`
	Type    string   `json:"type,omitempty"`
	Status  string   `json:"status,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Summary string   `json:"summary"`
	Score   float64  `json:"score,omitempty"`
}

func (b *Brain) info(n *Note) NoteInfo {
	return NoteInfo{
		Path:    n.Rel,
		Type:    n.FM.Type,
		Status:  n.FM.Status,
		Tags:    n.FM.Tags,
		Summary: n.FM.Summary,
	}
}

// Index returns front-matter info for every note — the summary-first index.
func (b *Brain) Index() ([]NoteInfo, error) {
	notes, err := b.Notes()
	if err != nil {
		return nil, err
	}
	out := make([]NoteInfo, 0, len(notes))
	for _, rel := range notes {
		n, err := b.Load(rel)
		if err != nil {
			return nil, err
		}
		out = append(out, b.info(n))
	}
	return out, nil
}

// Search returns notes matching every whitespace-separated term of the query
// (case-insensitive), ranked best-first. A term may hit any field; fields are
// weighted path > tags > summary > body so filename and tag hits outrank body
// mentions. When body is false, note bodies are not searched.
func (b *Brain) Search(query string, body bool) ([]NoteInfo, error) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil, nil
	}
	phrase := strings.Join(terms, " ")
	notes, err := b.Notes()
	if err != nil {
		return nil, err
	}
	var out []NoteInfo
	for _, rel := range notes {
		n, err := b.Load(rel)
		if err != nil {
			return nil, err
		}
		fields := []struct {
			text   string
			weight float64
		}{
			{strings.ToLower(rel), 4},
			{strings.ToLower(strings.Join(n.FM.Tags, " ")), 3},
			{strings.ToLower(n.FM.Summary), 2},
		}
		if body {
			fields = append(fields, struct {
				text   string
				weight float64
			}{strings.ToLower(n.Body), 1})
		}
		score := 0.0
		matched := true
		for _, term := range terms {
			ts := 0.0
			for _, f := range fields {
				if strings.Contains(f.text, term) {
					ts += f.weight
				}
			}
			if ts == 0 {
				matched = false
				break
			}
			score += ts
		}
		if !matched {
			continue
		}
		// Whole-phrase hits in path or summary outrank scattered term hits.
		if len(terms) > 1 {
			for _, f := range fields[:3] {
				if strings.Contains(f.text, phrase) {
					score += f.weight * 2
				}
			}
		}
		info := b.info(n)
		info.Score = score
		out = append(out, info)
	}
	SortByScore(out)
	return out, nil
}

// SortByScore orders notes best-first (score descending, path ascending as the
// tie-break) — the order agents should read results in.
func SortByScore(notes []NoteInfo) {
	sort.SliceStable(notes, func(i, j int) bool {
		if notes[i].Score != notes[j].Score {
			return notes[i].Score > notes[j].Score
		}
		return notes[i].Path < notes[j].Path
	})
}

// FindFilter constrains a structured find by front-matter fields. Empty fields
// are ignored; Tags must all be present.
type FindFilter struct {
	Type   string
	Status string
	Tags   []string
}

// Find returns notes matching all set constraints in the filter.
func (b *Brain) Find(f FindFilter) ([]NoteInfo, error) {
	notes, err := b.Notes()
	if err != nil {
		return nil, err
	}
	var out []NoteInfo
	for _, rel := range notes {
		n, err := b.Load(rel)
		if err != nil {
			return nil, err
		}
		if f.Type != "" && !strings.EqualFold(n.FM.Type, f.Type) {
			continue
		}
		if f.Status != "" && !strings.EqualFold(n.FM.Status, f.Status) {
			continue
		}
		if !hasAllTags(n.FM.Tags, f.Tags) {
			continue
		}
		out = append(out, b.info(n))
	}
	return out, nil
}

func hasAllTags(have, want []string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if strings.EqualFold(h, w) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
