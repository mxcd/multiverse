package brain

import "strings"

// NearDuplicates returns existing notes whose name+summary token sets overlap
// the candidate title+summary above the given Dice threshold, best-first. It is
// the offline dedup gate `multi write` warns with — one fact per file only
// works when the same fact isn't written twice under two names.
func (b *Brain) NearDuplicates(title, summary string, threshold float64, limit int) ([]NoteInfo, error) {
	cand := tokenSet(title + " " + summary)
	if len(cand) == 0 {
		return nil, nil
	}
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
		base := strings.TrimSuffix(rel[strings.LastIndexByte(rel, '/')+1:], ".md")
		score := dice(cand, tokenSet(strings.ReplaceAll(base, "-", " ")+" "+n.FM.Summary))
		if score >= threshold {
			info := b.info(n)
			info.Score = score
			out = append(out, info)
		}
	}
	SortByScore(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// tokenSet lowercases and splits text into its set of alphanumeric tokens,
// dropping one-character noise.
func tokenSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, t := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !('a' <= r && r <= 'z' || '0' <= r && r <= '9')
	}) {
		if len(t) > 1 {
			set[t] = true
		}
	}
	return set
}

// dice is the Sørensen–Dice coefficient of two token sets: 2·|A∩B| / (|A|+|B|).
func dice(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	for t := range a {
		if b[t] {
			shared++
		}
	}
	return 2 * float64(shared) / float64(len(a)+len(b))
}
