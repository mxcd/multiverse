package cli

import (
	"context"
	"fmt"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/urfave/cli/v3"
)

func syncCmd() *cli.Command {
	return &cli.Command{
		Name:  "sync",
		Usage: "commit, pull (rebase), and push every brain in scope — the transport you don't think about",
		Flags: withBrain(&cli.StringFlag{Name: "message", Aliases: []string{"m"}, Usage: "commit message for local changes"}),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			for _, sb := range sc.union() {
				res, err := sb.Sync(cmd.String("message"))
				if err != nil {
					fmt.Printf("%-14s error: %v\n", sb.Name, err)
					continue
				}
				printSync(sb.Name, res)
			}
			return nil
		},
	}
}

func statusCmd() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "git working-tree and sync status for every brain in scope",
		Flags: withBrain(),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			multi := len(sc.union()) > 1
			for _, sb := range sc.union() {
				out, err := sb.Status()
				if multi {
					fmt.Printf("# %s (%s)\n", sb.Name, sb.Root)
				}
				switch {
				case err != nil:
					fmt.Printf("error: %v\n", err)
				case out == "":
					fmt.Println("clean")
				default:
					fmt.Println(out)
				}
			}
			return nil
		},
	}
}

func lintCmd() *cli.Command {
	return &cli.Command{
		Name:  "lint",
		Usage: "verify the standing rules (summary, split tag, freshness) for every brain in scope",
		Flags: withBrain(
			&cli.BoolFlag{Name: "summary", Usage: "only the summary rule"},
			&cli.BoolFlag{Name: "tags", Usage: "only the split-tag rule"},
			&cli.BoolFlag{Name: "fresh", Usage: "only the freshness rule"},
			&cli.BoolFlag{Name: "json"},
		),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			opts := brain.LintOptions{
				Summary: cmd.Bool("summary"),
				Tags:    cmd.Bool("tags"),
				Fresh:   cmd.Bool("fresh"),
			}
			if !opts.Summary && !opts.Tags && !opts.Fresh {
				opts = brain.AllRules()
			}

			brains := sc.union()
			multi := len(brains) > 1
			reports := map[string]brain.LintReport{}
			anyFail := false
			for _, sb := range brains {
				rep, err := sb.Lint(opts)
				if err != nil {
					return err
				}
				reports[sb.Name] = rep
				if !rep.OK() {
					anyFail = true
				}
				if cmd.Bool("json") {
					continue
				}
				if multi {
					fmt.Printf("# %s\n", sb.Name)
				}
				for _, f := range rep.Findings {
					fmt.Printf("%-8s %-52s %s\n", f.Rule, f.Path, f.Message)
				}
				if rep.OK() {
					fmt.Printf("OK: %d note(s) pass\n", rep.Checked)
				} else {
					fmt.Printf("FAIL: %d finding(s) across %d note(s)\n", len(rep.Findings), rep.Checked)
				}
			}
			if cmd.Bool("json") {
				if err := printJSON(reports); err != nil {
					return err
				}
			}
			if anyFail {
				return cli.Exit("", 1)
			}
			return nil
		},
	}
}

func printSync(name string, res brain.SyncResult) {
	var parts []string
	if res.Committed {
		parts = append(parts, "committed")
	}
	if res.Pulled {
		parts = append(parts, "pulled")
	}
	if res.Pushed {
		parts = append(parts, "pushed")
	}
	if len(parts) == 0 {
		parts = append(parts, "nothing to do")
	}
	fmt.Printf("%-14s %s\n", name, join(parts))
	if res.Note != "" {
		fmt.Printf("%-14s note: %s\n", "", res.Note)
	}
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
