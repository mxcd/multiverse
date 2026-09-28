package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestReposRoundTrip(t *testing.T) {
	t.Setenv("MULTI_CONFIG_DIR", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c.Add(Brain{Name: "alpha", Path: "/vaults/alpha"})
	c.Platforms = map[string]string{"github.com": "github"}
	c.Repos = map[string]Repo{
		"github/mxcd/multiverse": {Path: "/src/multiverse", Aliases: []string{"multi"}},
		"github/fs-us/iac":       {Path: "/src/iac"},
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if r := got.Repos["github/mxcd/multiverse"]; r.Path != "/src/multiverse" || !slices.Equal(r.Aliases, []string{"multi"}) || len(got.Repos) != 2 {
		t.Fatalf("repos not round-tripped: %+v", got.Repos)
	}
	if got.Platforms["github.com"] != "github" {
		t.Fatalf("platforms not round-tripped: %+v", got.Platforms)
	}
	if got.Find("alpha") == nil {
		t.Fatal("brains lost next to repos")
	}
}

func TestLegacyScalarReposLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MULTI_CONFIG_DIR", dir)
	// the v1.6 form: repo id -> plain path string
	legacy := "repos:\n    cluster-csi: /src/cluster-csi\n    mbag/cluster-iam: /src/cluster-iam\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Repos["cluster-csi"].Path != "/src/cluster-csi" || c.Repos["mbag/cluster-iam"].Path != "/src/cluster-iam" {
		t.Fatalf("legacy repos not loaded: %+v", c.Repos)
	}
}

func TestConfigWithoutReposSavesUnchanged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MULTI_CONFIG_DIR", dir)
	// exactly what yaml.v3 writes for a pre-repos config
	orig := "active: alpha\nbrains:\n    - name: alpha\n      path: /vaults/alpha\n"
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != orig {
		t.Fatalf("config changed on save:\n%s", data)
	}
}
