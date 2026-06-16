package brain

import (
	"regexp"
	"strings"
)

var wikilinkRe = regexp.MustCompile(`\[\[([^\]\[]+)\]\]`)

// extractLinks returns the target note names referenced by [[wikilinks]] in text,
// stripping any |alias or #heading suffix.
func extractLinks(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range wikilinkRe.FindAllStringSubmatch(text, -1) {
		t := m[1]
		if i := strings.IndexAny(t, "|#"); i >= 0 {
			t = t[:i]
		}
		t = strings.TrimSpace(t)
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// Links returns the outgoing wikilink targets of a note (by name).
func (b *Brain) Links(rel string) ([]string, error) {
	n, err := b.Load(rel)
	if err != nil {
		return nil, err
	}
	return extractLinks(n.Body), nil
}

// Backlinks returns the notes that link to the given note. Targets are compared
// on their slug key, so a link survives whatever case/spacing it was written in.
func (b *Brain) Backlinks(rel string) ([]string, error) {
	target := slugKey(rel)
	notes, err := b.Notes()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, other := range notes {
		if other == rel {
			continue
		}
		n, err := b.Load(other)
		if err != nil {
			return nil, err
		}
		for _, link := range extractLinks(n.Body) {
			if slugKey(link) == target {
				out = append(out, other)
				break
			}
		}
	}
	return out, nil
}

// Orphans returns content notes that no other note links to — the link graph is
// the index, so an unlinked content note is a navigation dead spot.
func (b *Brain) Orphans() ([]string, error) {
	notes, err := b.Notes()
	if err != nil {
		return nil, err
	}
	linked := map[string]bool{}
	for _, rel := range notes {
		n, err := b.Load(rel)
		if err != nil {
			return nil, err
		}
		for _, link := range extractLinks(n.Body) {
			linked[slugKey(link)] = true
		}
	}
	var out []string
	for _, rel := range notes {
		n, err := b.Load(rel)
		if err != nil {
			return nil, err
		}
		if !b.IsContent(n) {
			continue
		}
		if !linked[slugKey(rel)] {
			out = append(out, rel)
		}
	}
	return out, nil
}
