package cli

import (
	"context"
	"fmt"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/urfave/cli/v3"
)

func fixCmd() *cli.Command {
	return &cli.Command{
		Name:  "fix",
		Usage: "rename every file and directory to kebab-case and update all wikilinks (auto-committed)",
		Description: "Brings a brain into the enforced kebab-case naming: each note file and\n" +
			"directory is renamed to its slug (lowercase, hyphen-separated, ASCII) and\n" +
			"every [[wikilink]] is rewritten to match, so the link graph stays intact.\n" +
			"File moves use `git mv` to preserve history, and the result is committed.\n\n" +
			"Run `multi fix --dry-run` first to preview the plan. Use --keep-display to\n" +
			"keep each link's rendered text as an alias, e.g. [[foo-bar|Foo Bar]].",
		Flags: withBrain(
			&cli.BoolFlag{Name: "dry-run", Aliases: []string{"n"}, Usage: "show what would change without touching anything"},
			&cli.BoolFlag{Name: "keep-display", Usage: "preserve a link's rendered text by adding it as an alias ([[foo-bar|Foo Bar]])"},
			&cli.BoolFlag{Name: "json"},
			&cli.BoolFlag{Name: "no-commit", Usage: "apply changes but do not git-commit them"},
			&cli.StringFlag{Name: "message", Aliases: []string{"m"}, Usage: "commit message"},
		),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			opts := brain.FixOptions{
				DryRun:      cmd.Bool("dry-run"),
				KeepDisplay: cmd.Bool("keep-display"),
			}

			brains := sc.union()
			multi := len(brains) > 1
			reports := map[string]brain.FixReport{}
			for _, sb := range brains {
				rep, err := sb.Fix(opts)
				if err != nil {
					if cmd.Bool("json") {
						return err
					}
					if multi {
						fmt.Printf("# %s\n", sb.Name)
					}
					fmt.Printf("error: %v\n", err)
					continue
				}
				reports[sb.Name] = rep

				if !cmd.Bool("json") {
					if multi {
						fmt.Printf("# %s\n", sb.Name)
					}
					printFixReport(rep, opts.DryRun)
				}

				if !opts.DryRun && rep.Changed() && !cmd.Bool("no-commit") && sb.IsGit() {
					msg := cmd.String("message")
					if msg == "" {
						msg = "fix: kebab-case file names and wikilinks"
					}
					if err := sb.Commit(nil, msg); err != nil {
						return fmt.Errorf("%s: changes applied but commit failed: %w", sb.Name, err)
					}
				}
			}

			if cmd.Bool("json") {
				return printJSON(reports)
			}
			return nil
		},
	}
}

func printFixReport(rep brain.FixReport, dryRun bool) {
	if !rep.Changed() {
		fmt.Println("already kebab-case: nothing to do")
		return
	}
	verb := "renamed"
	if dryRun {
		verb = "would rename"
	}
	for _, r := range rep.Renames {
		fmt.Printf("%-12s %s -> %s\n", verb, r.From, r.To)
	}
	linkVerb := "rewrote links in"
	if dryRun {
		linkVerb = "would rewrite links in"
	}
	for _, p := range rep.LinksChanged {
		fmt.Printf("%-12s %s\n", "links:", p)
	}
	summary := fmt.Sprintf("%s %d file(s); %s %d note(s)",
		verb, len(rep.Renames), linkVerb, len(rep.LinksChanged))
	fmt.Println(summary)
}
