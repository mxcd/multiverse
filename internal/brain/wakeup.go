package brain

// WakeupSection is one L0 identity note printed in full at session start.
type WakeupSection struct {
	Path string `json:"path"`
	Body string `json:"body"`
}

// Wakeup assembles the session bootstrap: the settings-listed identity notes
// printed in full (L0) and every `pinned: true` note as a one-line fact (L1).
// Deterministic and cheap — the whole point is replacing "hunt around at
// session start" with one bounded read.
func (b *Brain) Wakeup() ([]WakeupSection, []NoteInfo, error) {
	var sections []WakeupSection
	for _, ref := range b.Settings.Wakeup {
		rel, err := b.Resolve(ref)
		if err != nil {
			return nil, nil, err
		}
		n, err := b.Load(rel)
		if err != nil {
			return nil, nil, err
		}
		sections = append(sections, WakeupSection{Path: rel, Body: n.Body})
	}
	notes, err := b.Notes()
	if err != nil {
		return nil, nil, err
	}
	var facts []NoteInfo
	for _, rel := range notes {
		n, err := b.Load(rel)
		if err != nil {
			return nil, nil, err
		}
		if n.FM.Pinned {
			facts = append(facts, b.info(n))
		}
	}
	return sections, facts, nil
}
