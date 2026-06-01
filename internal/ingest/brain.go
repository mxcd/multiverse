package ingest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mxcd/multiverse/internal/config"
)

// BrainName is the registry name of the brain this ingester targets.
const BrainName = "deep-thought"

// BrainDir resolves the deep-thought brain's on-disk path from the multi registry.
func BrainDir() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	b := cfg.Find(BrainName)
	if b == nil {
		return "", fmt.Errorf("brain %q is not registered with multi", BrainName)
	}
	return b.Path, nil
}

// multiPath locates the multi binary.
func multiPath() string {
	if p, err := exec.LookPath("multi"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "go", "bin", "multi")
}

// SyncBrain commits/pulls/pushes the deep-thought brain via multi — a backstop in
// case the steered agent forgot to sync.
func SyncBrain() error {
	return exec.Command(multiPath(), "--brain", BrainName, "sync").Run()
}
