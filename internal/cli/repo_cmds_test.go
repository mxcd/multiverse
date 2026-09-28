package cli

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/mxcd/multiverse/internal/config"
)

// testPlatforms is the platforms map every registry test starts from.
var testPlatforms = map[string]string{
	"mercedes-benz.ghe.com": "mbag",
	"gitlab.wilde-it.com":   "wit",
	"git.fsintra.net":       "fsus-gitlab",
	"github.com":            "github",
}

// runMulti runs the multi command tree against the config in MULTI_CONFIG_DIR.
func runMulti(t *testing.T, args ...string) error {
	t.Helper()
	return NewApp("test").Run(context.Background(), append([]string{"multi"}, args...))
}

// newRegistry points MULTI_CONFIG_DIR at a fresh config holding testPlatforms.
func newRegistry(t *testing.T) {
	t.Helper()
	t.Setenv("MULTI_CONFIG_DIR", t.TempDir())
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Platforms = testPlatforms
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
}

// mkCheckout creates a git checkout with a subdirectory and the given origin
// remote (none when empty) and returns its symlink-resolved root (git reports
// /private/var on macOS, not /var).
func mkCheckout(t *testing.T, origin string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := brain.InitGit(dir); err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", origin).CombinedOutput(); err != nil {
			t.Fatalf("git remote add: %v: %s", err, out)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func loadRepos(t *testing.T) map[string]config.Repo {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Repos
}

// captureStdout returns what fn prints to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestRepoID(t *testing.T) {
	for origin, want := range map[string]string{
		"git@github.com:mxcd/multiverse.git":                             "github/mxcd/multiverse",
		"github.com:mxcd/multiverse":                                     "github/mxcd/multiverse",
		"https://github.com/fs-us/iac.git":                               "github/fs-us/iac",
		"https://token@github.com/fs-us/iac":                             "github/fs-us/iac",
		"https://oauth2:glpat-secret@git.fsintra.net/apps/intranet.git":  "fsus-gitlab/apps/intranet",
		"ssh://git@gitlab.wilde-it.com:2222/internal/devops/cluster.git": "wit/internal/devops/cluster",
		"ssh://gitlab.wilde-it.com/internal/devops/cluster/":             "wit/internal/devops/cluster",
		"git@mercedes-benz.ghe.com:wilde-it/cluster-csi.git":             "mbag/wilde-it/cluster-csi",
		"https://mercedes-benz.ghe.com/wilde-it/cluster-csi.git/":        "mbag/wilde-it/cluster-csi",
		"git@git.fsintra.net:tm/control_unit.git":                        "fsus-gitlab/tm/control_unit",
		"https://GitHub.com/OneDevice/od.git":                            "github/onedevice/od",
		"git@gitlab.example.com:a/b.git":                                 "gitlab.example.com/a/b",
		"http://user:pw@GitLab.Example.com:8080/Group/Sub/Repo":          "gitlab.example.com/group/sub/repo",
	} {
		got, err := repoID(origin, testPlatforms)
		if err != nil || got != want {
			t.Errorf("repoID(%q) = %q, %v; want %q", origin, got, err, want)
		}
	}
	for _, origin := range []string{"/srv/git/repo.git", "../repo", "file:///srv/git/repo.git", "https://github.com/", "git@github.com:", "git@github.com:.git"} {
		if id, err := repoID(origin, testPlatforms); err == nil {
			t.Errorf("repoID(%q) = %q, want an error", origin, id)
		}
	}
	if _, err := repoID("https://user:glpat-secret@github.com/", nil); err == nil || strings.Contains(err.Error(), "glpat-secret") {
		t.Errorf("errors must not leak credentials, got %v", err)
	}
}

func TestRepoAddDerivesID(t *testing.T) {
	newRegistry(t)
	root := mkCheckout(t, "git@github.com:mxcd/multiverse.git")

	// explicit path inside the checkout stores the toplevel
	if err := runMulti(t, "repo", "add", filepath.Join(root, "sub")); err != nil {
		t.Fatal(err)
	}
	// no path: the cwd, again resolved to the toplevel
	t.Chdir(filepath.Join(root, "sub", "deeper"))
	if err := runMulti(t, "repo", "add"); err != nil {
		t.Fatal(err)
	}
	repos := loadRepos(t)
	if repos["github/mxcd/multiverse"].Path != root || len(repos) != 1 {
		t.Fatalf("expected github/mxcd/multiverse at %s, got %+v", root, repos)
	}
}

func TestRepoAddRejects(t *testing.T) {
	newRegistry(t)

	if err := runMulti(t, "repo", "add", mkCheckout(t, "")); err == nil || !strings.Contains(err.Error(), "no origin remote") {
		t.Fatalf("checkout without origin: expected rejection, got %v", err)
	}
	if err := runMulti(t, "repo", "add", mkCheckout(t, "/srv/git/repo.git")); err == nil || !strings.Contains(err.Error(), "cannot derive a repo id") {
		t.Fatalf("local-path origin: expected rejection, got %v", err)
	}
	if err := runMulti(t, "repo", "add", t.TempDir()); err == nil || !strings.Contains(err.Error(), "not a git checkout") {
		t.Fatalf("non-git path: expected rejection, got %v", err)
	}
	if len(loadRepos(t)) != 0 {
		t.Fatalf("rejected adds must not be stored, got %+v", loadRepos(t))
	}
}

func TestRepoAddReplaces(t *testing.T) {
	newRegistry(t)
	// two clones of one repo through different remote forms share the id
	first := mkCheckout(t, "git@mercedes-benz.ghe.com:wilde-it/cluster-csi.git")
	second := mkCheckout(t, "https://Mercedes-Benz.ghe.com/Wilde-IT/Cluster-CSI")
	if err := runMulti(t, "repo", "add", first); err != nil {
		t.Fatal(err)
	}
	if err := runMulti(t, "repo", "add", second); err != nil {
		t.Fatal(err)
	}
	if repos := loadRepos(t); repos["mbag/wilde-it/cluster-csi"].Path != second || len(repos) != 1 {
		t.Fatalf("re-add should replace the path, got %+v", repos)
	}
}

func TestRepoPath(t *testing.T) {
	root := mkCheckout(t, "")
	cfg := &config.Config{Repos: map[string]config.Repo{
		"mbag/wilde-it/cluster-csi": {Path: root},
		"gone":                      {Path: filepath.Join(root, "no-such-dir")},
	}}

	if p, err := repoPath(cfg, "mbag/wilde-it/cluster-csi"); err != nil || p != root {
		t.Fatalf("expected %s, got %q (%v)", root, p, err)
	}
	_, err := repoPath(cfg, "unknown")
	if err == nil || !strings.Contains(err.Error(), "ask the user") || !strings.Contains(err.Error(), "multi repo add <path>") {
		t.Fatalf("unregistered id must tell the agent what to do, got %v", err)
	}
	_, err = repoPath(cfg, "gone")
	if err == nil || !strings.Contains(err.Error(), "no longer exists") || !strings.Contains(err.Error(), "multi repo add <path>") {
		t.Fatalf("missing dir must ask for re-registration, got %v", err)
	}
	// an empty registry (no repos key at all) behaves like an unknown id
	if _, err := repoPath(&config.Config{}, "mbag/wilde-it/cluster-csi"); err == nil {
		t.Fatal("expected error on an empty registry")
	}
}

func TestRepoRm(t *testing.T) {
	newRegistry(t)
	root := mkCheckout(t, "git@github.com:fs-us/iac.git")
	if err := runMulti(t, "repo", "add", root); err != nil {
		t.Fatal(err)
	}
	if err := runMulti(t, "repo", "rm", "github/fs-us/iac"); err != nil {
		t.Fatal(err)
	}
	if repos := loadRepos(t); len(repos) != 0 {
		t.Fatalf("rm should remove the entry, got %+v", repos)
	}
	if err := runMulti(t, "repo", "rm", "github/fs-us/iac"); err == nil {
		t.Fatal("rm of an unknown id must error")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("rm must leave the checkout alone: %v", err)
	}
}

func TestRepoListAlignsColumns(t *testing.T) {
	newRegistry(t)
	short := mkCheckout(t, "git@github.com:fs-us/iac.git")
	long := mkCheckout(t, "https://token@gitlab.example.com/a/very/deeply/nested/group/repo.git")
	for _, p := range []string{short, long} {
		if err := runMulti(t, "repo", "add", p); err != nil {
			t.Fatal(err)
		}
	}
	out := captureStdout(t, func() {
		if err := runMulti(t, "repo", "list"); err != nil {
			t.Fatal(err)
		}
	})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "github/fs-us/iac ") || !strings.HasPrefix(lines[1], "gitlab.example.com/a/very/deeply/nested/group/repo ") {
		t.Fatalf("expected one line per repo sorted by id, got:\n%s", out)
	}
	if strings.Index(lines[0], short) != strings.Index(lines[1], long) {
		t.Fatalf("path column not aligned to the longest id:\n%s", out)
	}
	if strings.Contains(out, "token@") {
		t.Fatalf("list must strip credentials from origins:\n%s", out)
	}
}

func TestRedactURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://user:glpat-secret@gitlab.example.com/a/b": "https://gitlab.example.com/a/b",
		"https://token@github.com/a/b":                     "https://github.com/a/b",
		"ssh://git@gitlab.example.com/a/b":                 "ssh://git@gitlab.example.com/a/b",
		"git@github.com:a/b":                               "git@github.com:a/b",
	} {
		if got := redactURL(in); got != want {
			t.Errorf("redactURL(%q) = %q, want %q", in, got, want)
		}
	}
}
