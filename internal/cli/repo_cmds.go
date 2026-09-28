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
// repo refers to another by id or alias, and each machine resolves it to its
// own checkout. Read-only towards the repos (rev-parse, remote get-url); they never
// join a scope, so sync/status/lint/reindex cannot touch them.
func repoCmd() *cli.Command {
	return &cli.Command{
		Name:  "repo",
		Usage: "resolve repository ids and aliases to checkouts on this machine",
		Commands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "register a git checkout under the id derived from its origin remote (path defaults to the cwd)",
				ArgsUsage: "[path]",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{Name: "alias", Usage: "short kebab-case name for the repo (repeatable)"},
				},
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
					if owner, ok := aliasOwner(cfg, id); ok {
						return fmt.Errorf("repo id %q is already an alias of repo %q - remove it first: multi repo unalias %s", id, owner, id)
					}
					if cfg.Repos == nil {
						cfg.Repos = map[string]config.Repo{}
					}
					r, known := cfg.Repos[id]
					old := r.Path
					r.Path = top
					cfg.Repos[id] = r
					if err := addAliases(cfg, id, cmd.StringSlice("alias")); err != nil {
						return err
					}
					if err := cfg.Save(); err != nil {
						return err
					}
					msg := fmt.Sprintf("registered repo %q at %s", id, top)
					if known && old != top {
						msg = fmt.Sprintf("replaced repo %q: %s -> %s", id, old, top)
					}
					if aliases := cfg.Repos[id].Aliases; len(aliases) > 0 {
						msg += " (aliases: " + strings.Join(aliases, ", ") + ")"
					}
					fmt.Println(msg)
					return nil
				},
			},
			{
				Name:      "path",
				Usage:     "print the checkout path of a repo id or alias (scriptable)",
				ArgsUsage: "<id|alias>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					name := cmd.Args().First()
					if name == "" {
						return errors.New("usage: multi repo path <id|alias>")
					}
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					p, err := repoPath(cfg, name)
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
				Usage:   "list registered repos with their aliases and origin remote",
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
						r := cfg.Repos[id]
						aliases := strings.Join(r.Aliases, ",")
						if aliases == "" {
							aliases = "-"
						}
						origin := "(missing)"
						if isDir(r.Path) {
							origin = "(no origin)"
							if url, err := brain.GitOrigin(r.Path); err == nil {
								origin = redactURL(url)
							}
						}
						fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", id, aliases, r.Path, origin)
					}
					return tw.Flush()
				},
			},
			{
				Name:      "alias",
				Usage:     "add aliases to a repo",
				ArgsUsage: "<id|alias> <alias>...",
				Action: func(_ context.Context, cmd *cli.Command) error {
					args := cmd.Args().Slice()
					if len(args) < 2 {
						return errors.New("usage: multi repo alias <id|alias> <alias>...")
					}
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					id, ok := lookupRepo(cfg, args[0])
					if !ok {
						return fmt.Errorf("unknown repo %q", args[0])
					}
					if err := addAliases(cfg, id, args[1:]); err != nil {
						return err
					}
					if err := cfg.Save(); err != nil {
						return err
					}
					fmt.Printf("repo %q aliases: %s\n", id, strings.Join(cfg.Repos[id].Aliases, ", "))
					return nil
				},
			},
			{
				Name:      "unalias",
				Usage:     "remove aliases from their repo",
				ArgsUsage: "<alias>...",
				Action: func(_ context.Context, cmd *cli.Command) error {
					aliases := cmd.Args().Slice()
					if len(aliases) == 0 {
						return errors.New("usage: multi repo unalias <alias>...")
					}
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					for _, a := range aliases {
						id, ok := aliasOwner(cfg, a)
						if !ok {
							return fmt.Errorf("unknown alias %q", a)
						}
						r := cfg.Repos[id]
						r.Aliases = slices.DeleteFunc(r.Aliases, func(s string) bool { return s == a })
						cfg.Repos[id] = r
					}
					if err := cfg.Save(); err != nil {
						return err
					}
					fmt.Printf("removed alias %s\n", strings.Join(aliases, ", "))
					return nil
				},
			},
			{
				Name:      "rm",
				Usage:     "unregister a repo and its aliases (the checkout itself is left alone)",
				ArgsUsage: "<id|alias>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					name := cmd.Args().First()
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					id, ok := lookupRepo(cfg, name)
					if !ok {
						return fmt.Errorf("unknown repo %q", name)
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

// repoPath resolves a repo id or alias to its registered checkout. Its errors
// tell an agent what to do instead of guessing a path.
func repoPath(cfg *config.Config, name string) (string, error) {
	id, ok := lookupRepo(cfg, name)
	if !ok {
		return "", fmt.Errorf("repo %q is not registered on this machine - ask the user for the local checkout path, then run: multi repo add <path> (optionally --alias <name>)", name)
	}
	p := cfg.Repos[id].Path
	if !isDir(p) {
		return "", fmt.Errorf("repo %q is registered at %s, but that directory no longer exists - ask the user for the current checkout path, then re-register it: multi repo add <path>", name, p)
	}
	return p, nil
}

// lookupRepo resolves an id or an alias to the registered repo id.
func lookupRepo(cfg *config.Config, name string) (string, bool) {
	if _, ok := cfg.Repos[name]; ok {
		return name, true
	}
	return aliasOwner(cfg, name)
}

// aliasOwner returns the id of the repo carrying alias.
func aliasOwner(cfg *config.Config, alias string) (string, bool) {
	for _, id := range slices.Sorted(maps.Keys(cfg.Repos)) {
		if slices.Contains(cfg.Repos[id].Aliases, alias) {
			return id, true
		}
	}
	return "", false
}

// addAliases gives repo id the aliases it does not carry yet. An alias is
// kebab-case and never equals any id or another repo's alias.
func addAliases(cfg *config.Config, id string, aliases []string) error {
	r := cfg.Repos[id]
	for _, a := range aliases {
		if !brain.IsKebab(a) {
			return fmt.Errorf("invalid alias %q: use lowercase kebab-case, e.g. multi", a)
		}
		if _, ok := cfg.Repos[a]; ok {
			return fmt.Errorf("alias %q is already a repo id - remove that repo first: multi repo rm %s", a, a)
		}
		if owner, ok := aliasOwner(cfg, a); ok && owner != id {
			return fmt.Errorf("alias %q already belongs to repo %q", a, owner)
		}
		if !slices.Contains(r.Aliases, a) {
			r.Aliases = append(r.Aliases, a)
		}
	}
	cfg.Repos[id] = r
	return nil
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
