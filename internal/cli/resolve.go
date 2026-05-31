package cli

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
)

// brainFlag selects a single brain by registry name or path, overriding any
// per-directory scope. Defined on the root command, it is persistent by default
// in cli/v3 (propagates to every subcommand), so --brain works before or after
// the subcommand name.
func brainFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    "brain",
		Aliases: []string{"b"},
		Usage:   "operate on a single brain (registry name or path), overriding the directory scope",
	}
}

// withBrain returns a command's flags unchanged — the --brain flag is inherited
// from the persistent root flag, so commands no longer declare it themselves.
func withBrain(flags ...cli.Flag) []cli.Flag {
	return flags
}

// findBrainRoot walks up from the cwd looking for a brain marker (.multi/brain.yaml).
func findBrainRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if isFile(filepath.Join(dir, ".multi", "brain.yaml")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func isDir(p string) bool  { info, err := os.Stat(p); return err == nil && info.IsDir() }
func isFile(p string) bool { info, err := os.Stat(p); return err == nil && !info.IsDir() }
