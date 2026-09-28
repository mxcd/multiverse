package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReposRoundTrip(t *testing.T) {
	t.Setenv("MULTI_CONFIG_DIR", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c.Add(Brain{Name: "alpha", Path: "/vaults/alpha"})
	c.Repos = map[string]string{"cluster-csi": "/src/cluster-csi"}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Repos["cluster-csi"] != "/src/cluster-csi" || len(got.Repos) != 1 {
		t.Fatalf("repos not round-tripped: %+v", got.Repos)
	}
	if got.Find("alpha") == nil {
		t.Fatal("brains lost next to repos")
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
