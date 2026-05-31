package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/mxcd/multiverse/internal/config"
	"github.com/urfave/cli/v3"
)

func onboardCmd() *cli.Command {
	return &cli.Command{
		Name:  "onboard",
		Usage: "interactive setup: create a new brain or clone an existing one",
		Action: func(_ context.Context, _ *cli.Command) error {
			r := bufio.NewReader(os.Stdin)
			fmt.Println("Welcome to multi — let's set up a brain.")
			mode := strings.ToLower(prompt(r, "Create a [n]ew brain or [c]lone an existing one? [n/c]: "))
			if strings.HasPrefix(mode, "c") {
				url := prompt(r, "Git URL to clone: ")
				if url == "" {
					return fmt.Errorf("a git URL is required to clone")
				}
				dest := prompt(r, "Destination directory (blank = derive from URL): ")
				name := prompt(r, "Brain name (blank = derive): ")
				return doClone(url, dest, name, true)
			}
			name := prompt(r, "Brain name: ")
			if name == "" {
				return fmt.Errorf("a brain name is required")
			}
			def := "./" + name
			path := prompt(r, fmt.Sprintf("Path [%s]: ", def))
			if path == "" {
				path = def
			}
			split := splitCSV([]string{prompt(r, "Split tags, comma-separated (blank = none, e.g. domain,operations): ")})
			return doInit(path, name, split, true, true)
		},
	}
}

func initCmd() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "create and scaffold a new brain, then git-init it",
		ArgsUsage: "[path]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "brain name (default: directory name)"},
			&cli.StringSliceFlag{Name: "split", Usage: "split tags every content note must carry one of"},
			&cli.BoolFlag{Name: "no-git", Usage: "do not initialize a git repository"},
			&cli.BoolFlag{Name: "activate", Value: true, Usage: "set as the active brain"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			path := cmd.Args().First()
			name := cmd.String("name")
			if path == "" {
				if name != "" {
					path = name
				} else {
					path = "."
				}
			}
			return doInit(path, name, splitCSV(cmd.StringSlice("split")), !cmd.Bool("no-git"), cmd.Bool("activate"))
		},
	}
}

func cloneCmd() *cli.Command {
	return &cli.Command{
		Name:      "clone",
		Usage:     "clone an existing brain and register it",
		ArgsUsage: "<git-url> [dest]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "brain name (default: derive from settings or URL)"},
			&cli.BoolFlag{Name: "activate", Value: true, Usage: "set as the active brain"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			url := cmd.Args().First()
			if url == "" {
				return fmt.Errorf("usage: multi clone <git-url> [dest]")
			}
			dest := cmd.Args().Get(1)
			return doClone(url, dest, cmd.String("name"), cmd.Bool("activate"))
		},
	}
}

func brainCmd() *cli.Command {
	return &cli.Command{
		Name:  "brain",
		Usage: "manage registered brains",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list registered brains",
				Action: func(_ context.Context, _ *cli.Command) error {
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					if len(cfg.Brains) == 0 {
						fmt.Println("no brains registered — run `multi onboard`")
						return nil
					}
					for _, br := range cfg.Brains {
						marker := "  "
						if br.Name == cfg.Active {
							marker = "* "
						}
						fmt.Printf("%s%-20s %s\n", marker, br.Name, br.Path)
					}
					return nil
				},
			},
			{
				Name:      "use",
				Usage:     "set the active brain",
				ArgsUsage: "<name>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					name := cmd.Args().First()
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					if cfg.Find(name) == nil {
						return fmt.Errorf("unknown brain %q", name)
					}
					cfg.Active = name
					if err := cfg.Save(); err != nil {
						return err
					}
					fmt.Printf("active brain: %s\n", name)
					return nil
				},
			},
			{
				Name:  "current",
				Usage: "print the active brain",
				Action: func(_ context.Context, _ *cli.Command) error {
					cfg, err := config.Load()
					if err != nil {
						return err
					}
					if ab := cfg.ActiveBrain(); ab != nil {
						fmt.Printf("%s\t%s\n", ab.Name, ab.Path)
					} else {
						fmt.Println("no active brain")
					}
					return nil
				},
			},
			{
				Name:      "add",
				Usage:     "register an existing brain directory",
				ArgsUsage: "<path>",
				Flags:     []cli.Flag{&cli.StringFlag{Name: "name"}, &cli.BoolFlag{Name: "activate"}},
				Action: func(_ context.Context, cmd *cli.Command) error {
					path := cmd.Args().First()
					if path == "" {
						return fmt.Errorf("usage: multi brain add <path>")
					}
					b, err := brain.Open(path)
					if err != nil {
						return err
					}
					name := cmd.String("name")
					if name == "" {
						name = b.Settings.Name
					}
					if name == "" {
						name = filepath.Base(b.Root)
					}
					return register(name, b.Root, cmd.Bool("activate"))
				},
			},
			{
				Name:      "split",
				Usage:     "set the split tags for a brain",
				ArgsUsage: "<tag,tag>",
				Flags:     withBrain(),
				Action: func(_ context.Context, cmd *cli.Command) error {
					sc, err := resolveScope(cmd)
					if err != nil {
						return err
					}
					target, err := sc.writeTarget()
					if err != nil {
						return err
					}
					target.Settings.Split = splitCSV(cmd.Args().Slice())
					if target.Settings.Name == "" {
						target.Settings.Name = target.Name
					}
					if err := target.SaveSettings(); err != nil {
						return err
					}
					fmt.Printf("split for %s: %s\n", target.Name, strings.Join(target.Settings.Split, ", "))
					return nil
				},
			},
		},
	}
}

func doInit(path, name string, split []string, withGit, activate bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if name == "" {
		name = filepath.Base(abs)
	}
	b, err := brain.Init(abs, brain.Settings{Name: name, Split: split}, withGit)
	if err != nil {
		return err
	}
	fmt.Printf("initialized brain %q at %s\n", name, b.Root)
	if withGit {
		fmt.Println("git: repository initialized with an initial commit")
		fmt.Println("hint: add a remote and `multi sync` to push:  git -C <path> remote add origin <url>")
	}
	return register(name, b.Root, activate)
}

func doClone(url, dest, name string, activate bool) error {
	if dest == "" {
		dest = deriveDest(url)
	}
	root, err := brain.Clone(url, dest)
	if err != nil {
		return err
	}
	b, err := brain.Open(root)
	if err != nil {
		return err
	}
	if name == "" {
		name = b.Settings.Name
	}
	if name == "" {
		name = filepath.Base(root)
	}
	fmt.Printf("cloned brain %q into %s\n", name, root)
	return register(name, root, activate)
}

func register(name, path string, activate bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Add(config.Brain{Name: name, Path: path})
	if activate || cfg.Active == "" {
		cfg.Active = name
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Printf("registered brain %q (active: %s)\n", name, cfg.Active)
	return nil
}

func deriveDest(url string) string {
	base := url
	if i := strings.LastIndexAny(base, "/:"); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".git")
}

func prompt(r *bufio.Reader, q string) string {
	fmt.Print(q)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}
