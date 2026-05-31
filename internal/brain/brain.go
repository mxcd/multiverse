// Package brain models a second brain: a git repository of markdown notes that
// follow the summary-first, freshness-tracked conventions. It is the storage
// layer the agent talks to — read, write, search — while git transport stays
// hidden behind it.
package brain

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Settings is the per-brain configuration stored at <root>/.multi/brain.yaml.
type Settings struct {
	// Name is the human label for the brain.
	Name string `yaml:"name"`
	// Split lists the mutually-exclusive "half" tags every content note must
	// carry exactly one of (e.g. [domain, operations]). Empty disables the rule.
	Split []string `yaml:"split,omitempty"`
}

// Brain is an opened brain rooted at a directory on disk.
type Brain struct {
	Root     string
	Settings Settings
}

const settingsRel = ".multi/brain.yaml"

// Open loads the brain rooted at root. A missing settings file is not an error —
// the brain simply has no enforced split.
func Open(root string) (*Brain, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	b := &Brain{Root: abs}
	if data, err := os.ReadFile(filepath.Join(abs, settingsRel)); err == nil {
		_ = yaml.Unmarshal(data, &b.Settings)
	}
	return b, nil
}

// DisplayName is the brain's human label: its configured name, else the
// directory base name.
func (b *Brain) DisplayName() string {
	if b.Settings.Name != "" {
		return b.Settings.Name
	}
	return filepath.Base(b.Root)
}

// SaveSettings writes the per-brain settings file.
func (b *Brain) SaveSettings() error {
	dir := filepath.Join(b.Root, ".multi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(b.Settings)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.Root, settingsRel), data, 0o644)
}

// Notes returns every markdown note path (vault-relative, sorted), skipping
// dot-directories such as .git, .obsidian and .multi.
func (b *Brain) Notes() ([]string, error) {
	var out []string
	err := filepath.WalkDir(b.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != b.Root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") {
			rel, relErr := filepath.Rel(b.Root, p)
			if relErr != nil {
				return relErr
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// Resolve turns a wikilink-style query (bare name, name.md, or exact relative
// path) into a single vault-relative note path.
func (b *Brain) Resolve(q string) (string, error) {
	q = filepath.ToSlash(strings.TrimSpace(q))
	if strings.HasSuffix(q, ".md") && fileExists(filepath.Join(b.Root, q)) {
		return q, nil
	}
	if fileExists(filepath.Join(b.Root, q+".md")) {
		return q + ".md", nil
	}
	notes, err := b.Notes()
	if err != nil {
		return "", err
	}
	want := strings.TrimSuffix(q, ".md")
	var matches []string
	for _, rel := range notes {
		if strings.TrimSuffix(filepath.Base(rel), ".md") == want {
			matches = append(matches, rel)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", &NotFoundError{Query: q}
	default:
		return "", &AmbiguousError{Query: q, Matches: matches}
	}
}

// IsContent reports whether a note is a "content" note subject to the split and
// freshness rules — i.e. not a root-level navigation note, not a template, and
// not type: meta.
func (b *Brain) IsContent(n *Note) bool {
	if !strings.Contains(n.Rel, "/") {
		return false
	}
	if strings.HasPrefix(n.Rel, "Templates/") {
		return false
	}
	return n.FM.Type != "meta"
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
