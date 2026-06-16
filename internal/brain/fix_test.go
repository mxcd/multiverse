package brain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteLinks(t *testing.T) {
	cases := []struct {
		in, want string
		keep     bool
	}{
		{"see [[Formula Student]] now", "see [[formula-student]] now", false},
		{"see [[Formula Student]]", "see [[formula-student|Formula Student]]", true},
		{"[[Foo Bar|the alias]]", "[[foo-bar|the alias]]", false},
		{"[[Foo Bar#Some Heading]]", "[[foo-bar#Some Heading]]", false},
		{"[[Foo Bar#Some Heading|alias]]", "[[foo-bar#Some Heading|alias]]", false},
		{"embed ![[Diagram.png]]", "embed ![[Diagram.png]]", false}, // asset untouched
		{"![[Some Note]]", "![[some-note]]", false},                 // note embed slugged
		{"[[already-kebab]]", "[[already-kebab]]", false},           // already correct
		{"[[Projects/Formula Student]]", "[[projects/formula-student]]", false},
		{"[[#Local Heading]]", "[[#Local Heading]]", false}, // same-file anchor
		{"text without links", "text without links", false},
	}
	for _, c := range cases {
		got, _ := RewriteLinks(c.in, c.keep)
		if got != c.want {
			t.Errorf("RewriteLinks(%q, keep=%v) = %q, want %q", c.in, c.keep, got, c.want)
		}
	}
}

func writeRaw(t *testing.T, root, rel, body string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntype: reference\nstatus: active\nsummary: s\n---\n\n# Title\n\n" + body + "\n"
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasNote(notes []string, rel string) bool {
	for _, n := range notes {
		if n == rel {
			return true
		}
	}
	return false
}

func TestFix(t *testing.T) {
	b := newBrain(t)
	writeRaw(t, b.Root, "Projects/Formula Student.md", "See [[Old Note]] and [[Formula Student]].")
	writeRaw(t, b.Root, "Projects/Old Note.md", "Back to [[Formula Student]].")

	rep, err := b.Fix(FixOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Applied {
		t.Fatal("expected Applied")
	}

	notes, err := b.Notes()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"projects/formula-student.md", "projects/old-note.md"} {
		if !hasNote(notes, want) {
			t.Errorf("missing renamed note %q in %v", want, notes)
		}
	}
	// Nothing should be left non-kebab.
	for _, n := range notes {
		if seg, ok := IsKebabPath(n); !ok {
			t.Errorf("note %q still has non-kebab segment %q", n, seg)
		}
	}

	// Links inside the renamed note were rewritten to the slug form.
	fs, err := b.Load("projects/formula-student.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fs.Body, "[[old-note]]") || !strings.Contains(fs.Body, "[[formula-student]]") {
		t.Errorf("links not rewritten: %q", fs.Body)
	}

	// The graph still resolves, including by the old human name.
	if rel, err := b.Resolve("Formula Student"); err != nil || rel != "projects/formula-student.md" {
		t.Errorf("legacy-name resolve failed: %q %v", rel, err)
	}
	back, err := b.Backlinks("projects/formula-student.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back[0] != "projects/old-note.md" {
		t.Errorf("unexpected backlinks: %v", back)
	}
}

func TestFixDryRun(t *testing.T) {
	b := newBrain(t)
	writeRaw(t, b.Root, "Projects/Foo Bar.md", "x")

	rep, err := b.Fix(FixOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Applied {
		t.Error("dry-run must not apply")
	}
	if len(rep.Renames) != 1 || rep.Renames[0].To != "projects/foo-bar.md" {
		t.Errorf("unexpected plan: %+v", rep.Renames)
	}
	if _, err := os.Stat(filepath.Join(b.Root, "Projects", "Foo Bar.md")); err != nil {
		t.Errorf("dry-run touched disk: %v", err)
	}
}

func TestFixCollisionAborts(t *testing.T) {
	b := newBrain(t)
	writeRaw(t, b.Root, "Foo Bar.md", "x") // slugs to foo-bar.md
	writeRaw(t, b.Root, "foo-bar.md", "y") // already exists at the target
	if _, err := b.Fix(FixOptions{DryRun: true}); err == nil {
		t.Fatal("expected a collision error")
	}
}

func TestFixGitPreservesAndRecases(t *testing.T) {
	// Make commits hermetic: CI runners often have no git author identity, and
	// Init(withGit) commits the scaffold. Env identity needs no config and takes
	// precedence, so the test never depends on the runner's ambient git setup.
	t.Setenv("GIT_AUTHOR_NAME", "multi test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "multi test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	b, err := Init(t.TempDir(), Settings{Name: "g", Split: []string{"domain"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !b.IsGit() {
		t.Skip("git not available")
	}
	writeRaw(t, b.Root, "Projects/Formula Student.md", "See [[Inbox]].")
	writeRaw(t, b.Root, "Inbox.md", "empty") // case-only rename: Inbox.md -> inbox.md
	if err := b.Commit(nil, "add legacy notes"); err != nil {
		t.Fatal(err)
	}

	rep, err := b.Fix(FixOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Applied {
		t.Fatal("expected Applied")
	}

	notes, err := b.Notes()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"projects/formula-student.md", "inbox.md"} {
		if !hasNote(notes, want) {
			t.Errorf("missing %q after git fix in %v", want, notes)
		}
	}
	for _, n := range notes {
		if seg, ok := IsKebabPath(n); !ok {
			t.Errorf("note %q still non-kebab (%q) after git fix", n, seg)
		}
	}
	// The renames are staged in git so a commit captures them.
	if err := b.Commit(nil, "fix: kebab"); err != nil {
		t.Fatalf("commit after fix failed: %v", err)
	}
	if dirty, _ := b.Dirty(); dirty {
		t.Error("work tree should be clean after committing the fix")
	}
}
