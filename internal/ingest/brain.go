package ingest

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mxcd/multiverse/internal/config"
)

// BrainName is the registry name of the brain this ingester targets. Override
// it with INGESTER_BRAIN — set on the Stop-hook command and any cron entry;
// the spawned dispatcher and steered session inherit it through the env.
var BrainName = cmp.Or(os.Getenv("INGESTER_BRAIN"), "second-brain")

// BrainDir resolves the target brain's on-disk path from the multi registry.
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

// SyncBrain commits/pulls/pushes the target brain via multi — a backstop in
// case the steered agent forgot to sync.
func SyncBrain() error {
	return exec.Command(multiPath(), "--brain", BrainName, "sync").Run()
}

// ReindexBrain refreshes the semantic shadow index (incremental) so notes the
// steered agent just wrote become visible to `multi similar` immediately
// instead of waiting for the next manual reindex.
func ReindexBrain() error {
	return exec.Command(multiPath(), "--brain", BrainName, "reindex").Run()
}
