package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/mxcd/multiverse/internal/config"
)

// runMulti runs the multi command tree against the config in MULTI_CONFIG_DIR.
func runMulti(t *testing.T, args ...string) error {
	t.Helper()
	return NewApp("test").Run(context.Background(), append([]string{"multi"}, args...))
}

// mkCheckout creates a git checkout with a subdirectory and returns its
// symlink-resolved root (git reports /private/var on macOS, not /var).
func mkCheckout(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := brain.InitGit(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func loadRepos(t *testing.T) map[string]string {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Repos
}

func TestRepoAddResolvesToplevel(t *testing.T) {
	t.Setenv("MULTI_CONFIG_DIR", t.TempDir())
	root := mkCheckout(t)

	// explicit path inside the checkout stores the toplevel
	if err := runMulti(t, "repo", "add", "explicit", filepath.Join(root, "sub")); err != nil {
		t.Fatal(err)
	}
	// no path: the cwd, again resolved to the toplevel
	t.Chdir(filepath.Join(root, "sub", "deeper"))
	if err := runMulti(t, "repo", "add", "from-cwd"); err != nil {
		t.Fatal(err)
	}
	repos := loadRepos(t)
	if repos["explicit"] != root || repos["from-cwd"] != root {
		t.Fatalf("expected both ids at %s, got %+v", root, repos)
	}
}

func TestRepoAddRejects(t *testing.T) {
	t.Setenv("MULTI_CONFIG_DIR", t.TempDir())
	root := mkCheckout(t)

	for _, id := range []string{"Cluster-CSI", "cluster_csi", "csi-", "a--b", "/a", "a/", "a//b", "mbag/Cluster"} {
		if err := runMulti(t, "repo", "add", id, root); err == nil || !strings.Contains(err.Error(), "kebab-case") {
			t.Fatalf("id %q: expected kebab-case error, got %v", id, err)
		}
	}
	plain := t.TempDir()
	if err := runMulti(t, "repo", "add", "plain", plain); err == nil || !strings.Contains(err.Error(), "not a git checkout") {
		t.Fatalf("non-git path: expected rejection, got %v", err)
	}
	if len(loadRepos(t)) != 0 {
		t.Fatalf("rejected adds must not be stored, got %+v", loadRepos(t))
	}
}

func TestRepoAddReplaces(t *testing.T) {
	t.Setenv("MULTI_CONFIG_DIR", t.TempDir())
	first, second := mkCheckout(t), mkCheckout(t)
	if err := runMulti(t, "repo", "add", "cluster-csi", first); err != nil {
		t.Fatal(err)
	}
	if err := runMulti(t, "repo", "add", "cluster-csi", second); err != nil {
		t.Fatal(err)
	}
	if repos := loadRepos(t); repos["cluster-csi"] != second || len(repos) != 1 {
		t.Fatalf("re-add should replace the path, got %+v", repos)
	}
}

func TestRepoPath(t *testing.T) {
	root := mkCheckout(t)
	cfg := &config.Config{Repos: map[string]string{
		"cluster-csi": root,
		"gone":        filepath.Join(root, "no-such-dir"),
	}}

	if p, err := repoPath(cfg, "cluster-csi"); err != nil || p != root {
		t.Fatalf("expected %s, got %q (%v)", root, p, err)
	}
	_, err := repoPath(cfg, "unknown")
	if err == nil || !strings.Contains(err.Error(), "ask the user") || !strings.Contains(err.Error(), "multi repo add unknown <path>") {
		t.Fatalf("unregistered id must tell the agent what to do, got %v", err)
	}
	_, err = repoPath(cfg, "gone")
	if err == nil || !strings.Contains(err.Error(), "no longer exists") || !strings.Contains(err.Error(), "multi repo add gone <path>") {
		t.Fatalf("missing dir must ask for re-registration, got %v", err)
	}
	// an empty registry (no repos key at all) behaves like an unknown id
	if _, err := repoPath(&config.Config{}, "cluster-csi"); err == nil {
		t.Fatal("expected error on an empty registry")
	}
}

func TestRepoRm(t *testing.T) {
	t.Setenv("MULTI_CONFIG_DIR", t.TempDir())
	root := mkCheckout(t)
	if err := runMulti(t, "repo", "add", "cluster-csi", root); err != nil {
		t.Fatal(err)
	}
	if err := runMulti(t, "repo", "rm", "cluster-csi"); err != nil {
		t.Fatal(err)
	}
	if repos := loadRepos(t); len(repos) != 0 {
		t.Fatalf("rm should remove the entry, got %+v", repos)
	}
	if err := runMulti(t, "repo", "rm", "cluster-csi"); err == nil {
		t.Fatal("rm of an unknown id must error")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("rm must leave the checkout alone: %v", err)
	}
}

func TestRepoAddNamespacedID(t *testing.T) {
	t.Setenv("MULTI_CONFIG_DIR", t.TempDir())
	root := mkCheckout(t)

	if err := runMulti(t, "repo", "add", "mbag/cluster-csi", root); err != nil {
		t.Fatal(err)
	}
	if loadRepos(t)["mbag/cluster-csi"] != root {
		t.Fatalf("namespaced id not stored, got %+v", loadRepos(t))
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
