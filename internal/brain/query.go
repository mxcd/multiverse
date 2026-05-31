package brain

import "strings"

// NoteInfo is the lightweight, front-matter-only view of a note used by index,
// search and find — so callers (and agents) judge relevance without reading bodies.
// Brain is the owning brain's display name; it is stamped by the scope layer when
// results span multiple brains and is empty for single-brain queries.
type NoteInfo struct {
	Brain   string   `json:"brain,omitempty"`
	Path    string   `json:"path"`
	Type    string   `json:"type,omitempty"`
	Status  string   `json:"status,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Summary string   `json:"summary"`
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

// Search returns notes whose path, summary or tags match the query
// (case-insensitive). When body is true, note bodies are matched too.
func (b *Brain) Search(query string, body bool) ([]NoteInfo, error) {
	q := strings.ToLower(strings.TrimSpace(query))
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
		hay := strings.ToLower(rel + " " + n.FM.Summary + " " + strings.Join(n.FM.Tags, " "))
		if body {
			hay += " " + strings.ToLower(n.Body)
		}
		if strings.Contains(hay, q) {
			out = append(out, b.info(n))
		}
	}
	return out, nil
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
