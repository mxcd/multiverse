package brain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrSummaryRequired enforces the brain's first standing rule: no note without a
// one-line summary. This is the guarantee that makes the brain navigable.
var ErrSummaryRequired = errors.New("a note must have a one-line summary (the gate readers read first)")

// WriteParams describes a note to create.
type WriteParams struct {
	Path    string // explicit vault-relative path; takes precedence over Dir/Title
	Dir     string // directory for the note when Path is empty
	Title   string // note title; also the filename when Path is empty
	Type    string
	Status  string
	Summary string
	Tags    []string
	Source    string
	Retrieved string
	Freshness string
	Body      string
	Force     bool // overwrite an existing note
}

// Write creates a note from params, enforcing the summary rule and auto-filling
// created/retrieved dates. It returns the note's vault-relative path.
func (b *Brain) Write(p WriteParams) (string, error) {
	if strings.TrimSpace(p.Summary) == "" {
		return "", ErrSummaryRequired
	}
	rel := notePath(p)
	if rel == "" {
		return "", errors.New("a note needs a --path or a --title")
	}
	abs := filepath.Join(b.Root, filepath.FromSlash(rel))
	if !p.Force {
		if _, err := os.Stat(abs); err == nil {
			return "", fmt.Errorf("note already exists: %s (use --force to overwrite, or `multi append`)", rel)
		}
	}

	today := time.Now().Format("2006-01-02")
	fm := FrontMatter{
		Type:      orDefault(p.Type, "reference"),
		Status:    orDefault(p.Status, "active"),
		Tags:      p.Tags,
		Created:   today,
		Summary:   strings.TrimSpace(p.Summary),
		Source:    p.Source,
		Retrieved: orDefault(p.Retrieved, today),
		Freshness: p.Freshness,
	}

	title := p.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(rel), ".md")
	}
	body := strings.TrimLeft(p.Body, "\n")
	var sb strings.Builder
	sb.WriteString(fm.Render())
	sb.WriteString("\n# ")
	sb.WriteString(title)
	sb.WriteString("\n")
	if body != "" {
		sb.WriteString("\n")
		sb.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			sb.WriteString("\n")
		}
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}
	return rel, nil
}

// Append adds content to the body of an existing note, leaving its front matter
// untouched.
func (b *Brain) Append(rel, content string) error {
	abs := filepath.Join(b.Root, filepath.FromSlash(rel))
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	buf := string(data)
	if !strings.HasSuffix(buf, "\n") {
		buf += "\n"
	}
	if !strings.HasSuffix(buf, "\n\n") {
		buf += "\n"
	}
	buf += strings.TrimRight(content, "\n") + "\n"
	return os.WriteFile(abs, []byte(buf), 0o644)
}

func notePath(p WriteParams) string {
	if p.Path != "" {
		rel := filepath.ToSlash(p.Path)
		if !strings.HasSuffix(rel, ".md") {
			rel += ".md"
		}
		return rel
	}
	if p.Title == "" {
		return ""
	}
	name := p.Title + ".md"
	if p.Dir != "" {
		return filepath.ToSlash(filepath.Join(p.Dir, name))
	}
	return name
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
