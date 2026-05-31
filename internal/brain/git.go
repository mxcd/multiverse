package brain

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// git runs a git command in the brain root and returns trimmed stdout.
func (b *Brain) git(args ...string) (string, error) {
	return runGit(b.Root, args...)
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return strings.TrimSpace(out.String()), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// IsGit reports whether the brain root is inside a git work tree.
func (b *Brain) IsGit() bool {
	_, err := b.git("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// HasRemote reports whether at least one git remote is configured.
func (b *Brain) HasRemote() bool {
	out, err := b.git("remote")
	return err == nil && strings.TrimSpace(out) != ""
}

// Dirty reports whether the work tree has uncommitted changes.
func (b *Brain) Dirty() (bool, error) {
	out, err := b.git("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Commit stages the given paths (vault-relative) and commits them. Passing no
// paths stages everything.
func (b *Brain) Commit(paths []string, message string) error {
	if len(paths) == 0 {
		if _, err := b.git("add", "-A"); err != nil {
			return err
		}
	} else {
		args := append([]string{"add", "--"}, toFS(paths)...)
		if _, err := b.git(args...); err != nil {
			return err
		}
	}
	dirty, err := b.Dirty()
	if err != nil {
		return err
	}
	if !dirty {
		return nil
	}
	_, err = b.git("commit", "-m", message)
	return err
}

// SyncResult summarizes what a sync did.
type SyncResult struct {
	Committed bool
	Pulled    bool
	Pushed    bool
	Offline   bool
	Note      string
}

// Sync is the git-wrapper transport the agent never has to think about: commit
// any local changes, integrate remote changes (rebase, *.md union-merged), then
// push. Network failures degrade to "offline" rather than erroring — commits
// queue locally and replay on the next reachable sync.
func (b *Brain) Sync(message string) (SyncResult, error) {
	var res SyncResult
	if !b.IsGit() {
		return res, fmt.Errorf("not a git repository: %s", b.Root)
	}

	if dirty, err := b.Dirty(); err != nil {
		return res, err
	} else if dirty {
		if message == "" {
			message = "sync: local changes"
		}
		if err := b.Commit(nil, message); err != nil {
			return res, err
		}
		res.Committed = true
	}

	if !b.HasRemote() {
		res.Note = "no remote configured; commits are local only"
		return res, nil
	}

	if _, err := b.git("pull", "--rebase", "--autostash"); err != nil {
		res.Offline = true
		res.Note = "pull failed (offline?); local commits preserved: " + err.Error()
		return res, nil
	}
	res.Pulled = true

	if _, err := b.git("push"); err != nil {
		res.Offline = true
		res.Note = "push failed (offline?); commits queued locally: " + err.Error()
		return res, nil
	}
	res.Pushed = true
	return res, nil
}

// GitInfo summarizes a brain's git state for dashboards.
type GitInfo struct {
	IsGit       bool
	Branch      string
	Dirty       bool
	HasRemote   bool
	HasUpstream bool
	Ahead       int
	Behind      int
}

// GitInfo collects the brain's current git state. Best-effort: fields stay at
// their zero value when the underlying query is unavailable.
func (b *Brain) GitInfo() GitInfo {
	gi := GitInfo{}
	if !b.IsGit() {
		return gi
	}
	gi.IsGit = true
	if br, err := b.git("rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		gi.Branch = br
	}
	if d, err := b.Dirty(); err == nil {
		gi.Dirty = d
	}
	gi.HasRemote = b.HasRemote()
	if _, err := b.git("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		gi.HasUpstream = true
		if out, err := b.git("rev-list", "--left-right", "--count", "@{u}...HEAD"); err == nil {
			fields := strings.Fields(out)
			if len(fields) == 2 {
				gi.Behind = atoi(fields[0])
				gi.Ahead = atoi(fields[1])
			}
		}
	}
	return gi
}

// NoteCount returns the number of markdown notes in the brain.
func (b *Brain) NoteCount() int {
	notes, err := b.Notes()
	if err != nil {
		return 0
	}
	return len(notes)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// Status returns a short porcelain status plus the ahead/behind summary.
func (b *Brain) Status() (string, error) {
	if !b.IsGit() {
		return "", fmt.Errorf("not a git repository: %s", b.Root)
	}
	out, err := b.git("status", "--short", "--branch")
	if err != nil {
		return "", err
	}
	return out, nil
}

// InitGit initializes a git repository at dir if one is not already present.
func InitGit(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	_, err := runGit(dir, "init")
	return err
}

// Clone clones url into dest and returns the absolute destination path.
func Clone(url, dest string) (string, error) {
	abs, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	if _, err := runGit(".", "clone", url, abs); err != nil {
		return "", err
	}
	return abs, nil
}

func toFS(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.FromSlash(p)
	}
	return out
}
