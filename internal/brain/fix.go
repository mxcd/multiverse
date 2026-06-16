package brain

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// FixOptions tunes a kebab-case migration.
type FixOptions struct {
	// DryRun computes and reports the plan without touching the filesystem.
	DryRun bool
	// KeepDisplay preserves a link's rendered text by adding it as an alias when
	// the target slug differs from the original ([[Foo Bar]] -> [[foo-bar|Foo Bar]]).
	KeepDisplay bool
}

// Rename is one planned file/directory move (vault-relative, slash-separated).
type Rename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// FixReport summarizes what a Fix did (or, in dry-run, would do).
type FixReport struct {
	Renames      []Rename `json:"renames"`
	LinksChanged []string `json:"links_changed"` // final paths of notes whose links were rewritten
	Applied      bool     `json:"applied"`
}

// Changed reports whether the fix found anything to do.
func (r FixReport) Changed() bool { return len(r.Renames) > 0 || len(r.LinksChanged) > 0 }

// Fix brings a brain into kebab-case: every note file and directory is renamed to
// its slug and every [[wikilink]] is rewritten to match, so the link graph stays
// intact. With DryRun set it only computes the plan. It does not commit — the
// caller decides whether to.
func (b *Brain) Fix(opts FixOptions) (FixReport, error) {
	notes, err := b.Notes()
	if err != nil {
		return FixReport{}, err
	}
	rep := FixReport{}

	// 1. Plan the renames. A note's slug path is always fully kebab, so a planned
	// target can never itself be a planned source — no rename chains or swaps.
	plan := map[string]string{} // from -> to
	for _, rel := range notes {
		if to := SlugifyPath(rel); to != rel {
			plan[rel] = to
		}
	}

	// 2. Reject collisions rather than silently clobber: two notes that slug to
	// the same path, or a slug that lands on an existing distinct note. Every
	// collision is collected so a migration can be unblocked in one pass instead
	// of one re-run per clash.
	existing := map[string]bool{}
	for _, rel := range notes {
		existing[rel] = true
	}
	byTarget := map[string][]string{}
	froms := make([]string, 0, len(plan))
	for from := range plan {
		froms = append(froms, from)
	}
	sort.Strings(froms)
	for _, from := range froms {
		byTarget[plan[from]] = append(byTarget[plan[from]], from)
	}
	targets := make([]string, 0, len(byTarget))
	for to := range byTarget {
		targets = append(targets, to)
	}
	sort.Strings(targets)
	var collisions []string
	for _, to := range targets {
		srcs := byTarget[to]
		// A planned target is always fully kebab, so it can never also be a
		// planned source; an existing note at the target is therefore distinct.
		if len(srcs) > 1 || existing[to] {
			group := append([]string{}, srcs...)
			if existing[to] {
				group = append(group, to+" (already exists)")
			}
			collisions = append(collisions, fmt.Sprintf("  %s  <=  %s", to, strings.Join(group, ", ")))
		}
	}
	if len(collisions) > 0 {
		return rep, fmt.Errorf("fix aborted: %d name collision(s) — resolve each by hand (rename, merge, or delete one), then re-run:\n%s",
			len(collisions), strings.Join(collisions, "\n"))
	}

	for from, to := range plan {
		rep.Renames = append(rep.Renames, Rename{From: from, To: to})
	}
	sort.Slice(rep.Renames, func(i, j int) bool { return rep.Renames[i].From < rep.Renames[j].From })

	// 3. Rewrite wikilinks in every note (in place, at its current path). Record
	// the note's *final* path so the caller can stage exactly what changed.
	for _, rel := range notes {
		abs := filepath.Join(b.Root, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			return rep, err
		}
		newText, changed := RewriteLinks(string(data), opts.KeepDisplay)
		if !changed {
			continue
		}
		final := rel
		if to, ok := plan[rel]; ok {
			final = to
		}
		rep.LinksChanged = append(rep.LinksChanged, final)
		if !opts.DryRun {
			if err := os.WriteFile(abs, []byte(newText), 0o644); err != nil {
				return rep, err
			}
		}
	}
	sort.Strings(rep.LinksChanged)

	if opts.DryRun {
		return rep, nil
	}

	// 4. Apply the renames (content was rewritten first, then the file moves).
	git := b.IsGit()
	for _, r := range rep.Renames {
		if err := b.moveNote(r.From, r.To, git); err != nil {
			return rep, fmt.Errorf("rename %q -> %q: %w", r.From, r.To, err)
		}
	}

	// 5. Drop directories emptied by the moves. On a case-sensitive filesystem
	// this removes the old PascalCase dirs (their files moved to new lowercase
	// dirs). On a case-insensitive one the old dir still holds the files (the
	// lowercase path resolved back to it), so it survives to step 6.
	if err := b.pruneEmptyDirs(); err != nil {
		return rep, err
	}

	// 6. Correct directory casing left over on case-insensitive filesystems,
	// where "Projects/" never physically became "projects/" above. Git already
	// records the lowercase path; this only realigns the working tree.
	if err := b.recaseDirs(); err != nil {
		return rep, err
	}

	rep.Applied = true
	return rep, nil
}

// moveNote renames a note, preferring `git mv` so history follows the file.
// Case-only renames (README.md -> readme.md) go through a temp name because a
// case-insensitive filesystem treats source and destination as the same path.
func (b *Brain) moveNote(from, to string, git bool) error {
	fromAbs := filepath.Join(b.Root, filepath.FromSlash(from))
	toAbs := filepath.Join(b.Root, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(toAbs), 0o755); err != nil {
		return err
	}
	caseOnly := from != to && strings.EqualFold(from, to)

	if git {
		var err error
		if caseOnly {
			tmp := filepath.FromSlash(to) + ".multifix-tmp"
			if _, err = b.git("mv", "-f", filepath.FromSlash(from), tmp); err == nil {
				_, err = b.git("mv", "-f", tmp, filepath.FromSlash(to))
			}
		} else {
			_, err = b.git("mv", "-f", filepath.FromSlash(from), filepath.FromSlash(to))
		}
		if err == nil {
			return nil
		}
		// Untracked or otherwise un-mv-able: fall back to a plain rename; the
		// caller's commit (git add -A) will pick it up.
	}
	return renameCaseAware(fromAbs, toAbs, caseOnly)
}

func renameCaseAware(fromAbs, toAbs string, caseOnly bool) error {
	if caseOnly {
		tmp := toAbs + ".multifix-tmp"
		if err := os.Rename(fromAbs, tmp); err != nil {
			return err
		}
		return os.Rename(tmp, toAbs)
	}
	return os.Rename(fromAbs, toAbs)
}

// pruneEmptyDirs removes empty directories under the brain root, deepest first,
// skipping dot-directories (.git, .multi, .obsidian).
func (b *Brain) pruneEmptyDirs() error {
	var dirs []string
	err := filepath.WalkDir(b.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != b.Root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			if p != b.Root {
				dirs = append(dirs, p)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil || len(entries) > 0 {
			continue
		}
		_ = os.Remove(d)
	}
	return nil
}

// recaseDirs realigns directory casing on case-insensitive filesystems, where a
// move into "projects/" lands in an existing "Projects/" and leaves the on-disk
// name untouched. Only pure case-only differences are corrected (other renames
// already created fresh lowercase directories), so there is never a merge to do.
// Each rename goes through a temp name because source and destination resolve to
// the same path. Deepest directories first, re-walking after each change.
func (b *Brain) recaseDirs() error {
	for {
		target, want := "", ""
		err := filepath.WalkDir(b.Root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			if p != b.Root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			if p == b.Root {
				return nil
			}
			s := slugOrUntitled(d.Name())
			if s != d.Name() && strings.EqualFold(s, d.Name()) && len(p) > len(target) {
				target, want = p, s
			}
			return nil
		})
		if err != nil {
			return err
		}
		if target == "" {
			return nil
		}
		newPath := filepath.Join(filepath.Dir(target), want)
		tmp := target + "~multifix"
		if err := os.Rename(target, tmp); err != nil {
			return err
		}
		if err := os.Rename(tmp, newPath); err != nil {
			return err
		}
	}
}

// wikilinkFullRe captures a whole [[wikilink]] (and any leading ! embed marker)
// so it can be rewritten while preserving the surrounding text.
var wikilinkFullRe = regexp.MustCompile(`(!?)\[\[([^\]\n]+?)\]\]`)

// assetExts are non-note extensions a wikilink may target (images, binaries).
// Such links point at files Fix does not rename, so their targets are left alone.
var assetExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".webp": true, ".bmp": true, ".ico": true, ".pdf": true, ".mp4": true,
	".mov": true, ".webm": true, ".mp3": true, ".wav": true, ".m4a": true,
	".zip": true, ".tar": true, ".gz": true, ".7z": true, ".rar": true,
	".xlsx": true, ".docx": true, ".pptx": true, ".csv": true, ".json": true,
	".excalidraw": true, ".canvas": true,
}

// RewriteLinks rewrites every note-targeting [[wikilink]] in text to its
// kebab-case form, preserving #headings, |aliases, and ! embed markers, and
// leaving links to non-note assets untouched. It reports whether anything
// changed. When keepDisplay is set, an alias-less link whose target slug differs
// gains the original text as an alias so its rendered form is preserved.
func RewriteLinks(text string, keepDisplay bool) (string, bool) {
	changed := false
	out := wikilinkFullRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := wikilinkFullRe.FindStringSubmatch(m)
		bang, inner := sub[1], sub[2]
		rewritten := bang + "[[" + rewriteWikilinkBody(inner, keepDisplay) + "]]"
		if rewritten != m {
			changed = true
		}
		return rewritten
	})
	return out, changed
}

// rewriteWikilinkBody rewrites the inside of one wikilink ("target#heading|alias").
func rewriteWikilinkBody(inner string, keepDisplay bool) string {
	target, alias := inner, ""
	if i := strings.Index(inner, "|"); i >= 0 {
		target, alias = inner[:i], inner[i+1:]
	}
	head := ""
	if i := strings.Index(target, "#"); i >= 0 {
		target, head = target[:i], target[i:]
	}
	display := strings.TrimSpace(target)
	if display == "" {
		return inner // same-file heading link like [[#Section]]
	}
	if assetExts[strings.ToLower(filepath.Ext(display))] {
		return inner // points at a file Fix does not rename
	}

	newTarget := slugLinkTarget(display)

	var b strings.Builder
	b.WriteString(newTarget)
	b.WriteString(head)
	switch {
	case alias != "":
		b.WriteString("|")
		b.WriteString(alias)
	case keepDisplay && newTarget != display:
		b.WriteString("|")
		b.WriteString(display)
	}
	return b.String()
}

// slugLinkTarget kebab-cases a wikilink's target path, segment by segment,
// without appending an extension (links are extensionless by convention).
func slugLinkTarget(t string) string {
	t = strings.TrimSuffix(filepath.ToSlash(t), ".md")
	var out []string
	for _, seg := range strings.Split(t, "/") {
		if strings.TrimSpace(seg) == "" {
			continue
		}
		out = append(out, slugOrUntitled(seg))
	}
	return strings.Join(out, "/")
}
