package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mxcd/multiverse/internal/config"
	"github.com/urfave/cli/v3"
)

func useCmd() *cli.Command {
	return &cli.Command{
		Name:      "use",
		Usage:     "bind the current directory to brains (writes ./.multi.yaml)",
		ArgsUsage: "<brain> [brain...]",
		Description: "Shorthand for `multi scope set`: the named brains become both sources and\n" +
			"targets for this directory and everything beneath it. Use `multi scope set`\n" +
			"for read-only sources or a restricted target set.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "read-only", Usage: "bind as sources only (no write targets)"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			refs := splitCSV(cmd.Args().Slice())
			if len(refs) == 0 {
				return errors.New("usage: multi use <brain> [brain...]")
			}
			bnd := config.Binding{Sources: refs, ReadOnly: cmd.Bool("read-only")}
			return setBinding(bnd)
		},
	}
}

func scopeCmd() *cli.Command {
	return &cli.Command{
		Name:  "scope",
		Usage: "inspect or configure which brains this directory reads from and writes to",
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			printScope(sc)
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "set",
				Usage: "write ./.multi.yaml binding sources and targets",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{Name: "source", Aliases: []string{"s"}, Usage: "brain to read from (repeatable or comma-separated)"},
					&cli.StringSliceFlag{Name: "target", Aliases: []string{"t"}, Usage: "brain to write to (repeatable or comma-separated)"},
					&cli.BoolFlag{Name: "read-only", Usage: "bind sources only — no write targets"},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					src := splitCSV(cmd.StringSlice("source"))
					tgt := splitCSV(cmd.StringSlice("target"))
					if len(src) == 0 && len(tgt) == 0 {
						return errors.New("pass --source and/or --target")
					}
					return setBinding(config.Binding{Sources: src, Targets: tgt, ReadOnly: cmd.Bool("read-only")})
				},
			},
			{
				Name:  "clear",
				Usage: "remove ./.multi.yaml in the current directory",
				Action: func(_ context.Context, _ *cli.Command) error {
					cwd, err := os.Getwd()
					if err != nil {
						return err
					}
					p := filepath.Join(cwd, config.BindingFile)
					if err := os.Remove(p); err != nil {
						return err
					}
					fmt.Printf("removed %s\n", p)
					return nil
				},
			},
		},
	}
}

// setBinding validates referenced brains, then writes the binding into the cwd.
func setBinding(bnd config.Binding) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for _, ref := range append(append([]string{}, bnd.Sources...), bnd.Targets...) {
		if _, err := openRef(cfg, ref); err != nil {
			return err
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	p, err := config.WriteBinding(cwd, bnd)
	if err != nil {
		return err
	}
	fmt.Printf("bound %s\n", p)
	fmt.Printf("  sources: %s\n", strings.Join(bnd.Sources, ", "))
	switch {
	case bnd.ReadOnly:
		fmt.Println("  targets: (none — read-only)")
	case len(bnd.Targets) == 0:
		fmt.Printf("  targets: %s (default = sources)\n", strings.Join(bnd.Sources, ", "))
	default:
		fmt.Printf("  targets: %s\n", strings.Join(bnd.Targets, ", "))
	}
	return nil
}

func printScope(sc *Scope) {
	srcs := make([]string, 0, len(sc.Sources))
	for _, sb := range sc.Sources {
		srcs = append(srcs, fmt.Sprintf("%s (%s)", sb.Name, sb.Root))
	}
	tgts := make([]string, 0, len(sc.Targets))
	for i, sb := range sc.Targets {
		label := sb.Name
		if i == 0 && len(sc.Targets) > 1 {
			label += " [default]"
		}
		tgts = append(tgts, label)
	}
	fmt.Printf("origin:  %s\n", sc.Origin)
	fmt.Printf("sources: %s\n", strings.Join(srcs, "\n         "))
	if len(tgts) == 0 {
		fmt.Println("targets: (none — read-only)")
	} else {
		fmt.Printf("targets: %s\n", strings.Join(tgts, ", "))
	}
}
