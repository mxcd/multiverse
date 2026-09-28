package cli

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/mxcd/multiverse/internal/config"
	"github.com/urfave/cli/v3"
)

// repoCmd manages the per-machine registry of repository checkouts: a shared
// repo refers to another by id, and each machine resolves that id to its own
// checkout. Read-only towards the repos (rev-parse, remote get-url); they never
// join a scope, so sync/status/lint/reindex cannot touch them.
func repoCmd() *cli.Command {
	return &cli.Command{
		Name:  "repo",
		Usage: "resolve repository ids to checkouts on this machine",
		Commands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "register a git checkout under an id (path defaults to the cwd)",
				ArgsUsage: "<id> [path]",
				Action: func(_ context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return errors.New("usage: multi repo add <id> [path]")
					}
					if !validRepoID(id) {
						return fmt.Errorf("invalid repo id %q: use lowercase kebab-case segments, optionally namespaced with /, e.g. mbag/cluster-csi", id)
					}
					path := cmd.Args().Get(1)
					if path == "" {
						path = "."
					}
					abs, err := filepath.Abs(path)
					if err != nil {
						return err
					}
					top, err := brain.GitToplevel(abs)
					if err != nil {
						return fmt.Errorf("not a git checkout: %s", abs)
					}
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					if cfg.Repos == nil {
						cfg.Repos = map[string]config.Repo{}
					}
					r, known := cfg.Repos[id]
					old := r.Path
					r.Path = top
					cfg.Repos[id] = r
					if err := cfg.Save(); err != nil {
						return err
					}
					if known && old != top {
						fmt.Printf("replaced repo %q: %s -> %s\n", id, old, top)
					} else {
						fmt.Printf("registered repo %q at %s\n", id, top)
					}
					return nil
				},
			},
			{
				Name:      "path",
				Usage:     "print the checkout path of a repo id (scriptable)",
				ArgsUsage: "<id>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return errors.New("usage: multi repo path <id>")
					}
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					p, err := repoPath(cfg, id)
					if err != nil {
						return err
					}
					fmt.Println(p)
					return nil
				},
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "list registered repos with their origin remote",
				Action: func(_ context.Context, _ *cli.Command) error {
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					if len(cfg.Repos) == 0 {
						fmt.Println("no repos registered - run `multi repo add <id> [path]`")
						return nil
					}
					for _, id := range slices.Sorted(maps.Keys(cfg.Repos)) {
						p := cfg.Repos[id].Path
						origin := "(missing)"
						if isDir(p) {
							origin = "(no origin)"
							if url, err := brain.GitOrigin(p); err == nil {
								origin = redactURL(url)
							}
						}
						fmt.Printf("%-20s %s  %s\n", id, p, origin)
					}
					return nil
				},
			},
			{
				Name:      "rm",
				Usage:     "unregister a repo id (the checkout itself is left alone)",
				ArgsUsage: "<id>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					if _, ok := cfg.Repos[id]; !ok {
						return fmt.Errorf("unknown repo %q", id)
					}
					delete(cfg.Repos, id)
					if err := cfg.Save(); err != nil {
						return err
					}
					fmt.Printf("removed repo %q\n", id)
					return nil
				},
			},
		},
	}
}

// repoPath resolves a repo id to its registered checkout. Its errors tell an
// agent what to do instead of guessing a path.
func repoPath(cfg *config.Config, id string) (string, error) {
	r, ok := cfg.Repos[id]
	p := r.Path
	if !ok {
		return "", fmt.Errorf("repo %q is not registered on this machine - ask the user for the local checkout path, then run: multi repo add %s <path>", id, id)
	}
	if !isDir(p) {
		return "", fmt.Errorf("repo %q is registered at %s, but that directory no longer exists - ask the user for the current checkout path, then re-register it: multi repo add %s <path>", id, p, id)
	}
	return p, nil
}

// validRepoID accepts kebab-case segments joined by "/", so ids can carry a
// namespace (mbag/cluster-csi) without allowing empty or odd segments.
func validRepoID(id string) bool {
	for _, seg := range strings.Split(id, "/") {
		if !brain.IsKebab(seg) {
			return false
		}
	}
	return true
}

// redactURL drops the userinfo of an http(s) remote: older checkouts carry
// access tokens there, and `repo list` output lands in agent context.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return raw
	}
	u.User = nil
	return u.String()
}
