package brain

import (
	"fmt"
	"strings"
)

// LintFinding is one problem found by a lint pass.
type LintFinding struct {
	Path    string `json:"path"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// LintReport is the result of running one or more lint rules.
type LintReport struct {
	Checked  int           `json:"checked"`
	Findings []LintFinding `json:"findings"`
}

// OK reports whether the lint passed (no findings).
func (r LintReport) OK() bool { return len(r.Findings) == 0 }

// LintOptions selects which standing rules to enforce.
type LintOptions struct {
	Summary bool // every note carries a one-line summary
	Tags    bool // every content note carries exactly one split tag
	Fresh   bool // every content note records source + retrieved + freshness
}

// AllRules enables every rule.
func AllRules() LintOptions { return LintOptions{Summary: true, Tags: true, Fresh: true} }

// Lint runs the selected standing-rule checks across the brain.
func (b *Brain) Lint(opts LintOptions) (LintReport, error) {
	notes, err := b.Notes()
	if err != nil {
		return LintReport{}, err
	}
	rep := LintReport{Checked: len(notes)}
	for _, rel := range notes {
		n, err := b.Load(rel)
		if err != nil {
			return rep, err
		}
		content := b.IsContent(n)

		if opts.Summary {
			if !n.HasFM {
				rep.add(rel, "summary", "no front matter")
			} else if strings.TrimSpace(n.FM.Summary) == "" {
				rep.add(rel, "summary", "missing summary")
			}
		}

		if opts.Tags && content && len(b.Settings.Split) > 0 {
			if !hasOneSplitTag(n.FM.Tags, b.Settings.Split) {
				rep.add(rel, "tags", fmt.Sprintf("missing a split tag (%s)", strings.Join(b.Settings.Split, "|")))
			}
		}

		if opts.Fresh && content {
			var miss []string
			if n.FM.Source == "" {
				miss = append(miss, "source")
			}
			if n.FM.Retrieved == "" {
				miss = append(miss, "retrieved")
			}
			if n.FM.Freshness == "" {
				miss = append(miss, "freshness")
			}
			if len(miss) > 0 {
				rep.add(rel, "fresh", "missing "+strings.Join(miss, ", "))
			}
		}
	}
	return rep, nil
}

func (r *LintReport) add(path, rule, msg string) {
	r.Findings = append(r.Findings, LintFinding{Path: path, Rule: rule, Message: msg})
}

func hasOneSplitTag(tags, split []string) bool {
	for _, t := range tags {
		for _, s := range split {
			if strings.EqualFold(t, s) {
				return true
			}
		}
	}
	return false
}
