package cli

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

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
				Usage:     "register a git checkout under the id derived from its origin remote (path defaults to the cwd)",
				ArgsUsage: "[path]",
				Action: func(_ context.Context, cmd *cli.Command) error {
					path := cmd.Args().First()
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
					origin, err := brain.GitOrigin(top)
					if err != nil {
						return fmt.Errorf("%s has no origin remote - the repo id is derived from it", top)
					}
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					id, err := repoID(origin, cfg.Platforms)
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
						fmt.Println("no repos registered - run `multi repo add [path]`")
						return nil
					}
					tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					for _, id := range slices.Sorted(maps.Keys(cfg.Repos)) {
						p := cfg.Repos[id].Path
						origin := "(missing)"
						if isDir(p) {
							origin = "(no origin)"
							if url, err := brain.GitOrigin(p); err == nil {
								origin = redactURL(url)
							}
						}
						fmt.Fprintf(tw, "%s\t%s\t%s\n", id, p, origin)
					}
					return tw.Flush()
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
	if !ok {
		return "", fmt.Errorf("repo %q is not registered on this machine - ask the user for the local checkout path, then run: multi repo add <path>", id)
	}
	if !isDir(r.Path) {
		return "", fmt.Errorf("repo %q is registered at %s, but that directory no longer exists - ask the user for the current checkout path, then re-register it: multi repo add <path>", id, r.Path)
	}
	return r.Path, nil
}

// repoID derives a repo id from an origin remote: the platform prefix of its
// host (the host itself when platforms has no entry) plus the repo path,
// lowercased so every clone of a repo gets the same id. Accepts
// scheme://[userinfo@]host[:port]/path and scp-style [user@]host:path.
func repoID(origin string, platforms map[string]string) (string, error) {
	var host, path string
	if strings.Contains(origin, "://") {
		if u, err := url.Parse(origin); err == nil {
			host, path = u.Hostname(), u.Path
		}
	} else if h, p, ok := strings.Cut(origin, ":"); ok && !strings.Contains(h, "/") {
		host, path = h[strings.LastIndex(h, "@")+1:], p
	}
	host = strings.ToLower(host)
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if host == "" || path == "" {
		return "", fmt.Errorf("cannot derive a repo id from origin %s: expected host and path, e.g. git@github.com:group/repo.git", redactURL(origin))
	}
	prefix := host
	if p := platforms[host]; p != "" {
		prefix = p
	}
	return strings.ToLower(prefix + "/" + path), nil
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
