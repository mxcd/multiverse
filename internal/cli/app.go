// Package cli wires the multi command tree (urfave/cli v3) onto the brain layer.
package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/urfave/cli/v3"
)

// NewApp builds the root multi command.
func NewApp(version string) *cli.Command {
	return &cli.Command{
		Name:    "multi",
		Usage:   "a git-backed markdown second brain for agents and humans",
		Version: version,
		Description: "multi is the storage-layer CLI for \"brains\": git repositories of markdown\n" +
			"notes. A per-directory .multi.yaml binds the directory you work in to the\n" +
			"brains it reads from (sources) and writes to (targets); reads span all\n" +
			"sources, writes land in a target. Git transport stays hidden behind\n" +
			"read / write / search so an agent never thinks about sync.",
		Flags:  []cli.Flag{brainFlag()},
		Action: rootAction,
		Commands: []*cli.Command{
			// interactive control panel
			tuiCmd(),
			// agent / LLM usage guide
			guideCmd(),
			// onboarding & registry
			onboardCmd(),
			initCmd(),
			cloneCmd(),
			brainCmd(),
			// per-directory scope
			useCmd(),
			scopeCmd(),
			// read (span all sources)
			listCmd(),
			summaryCmd(),
			fmCmd(),
			readCmd(),
			searchCmd(),
			findCmd(),
			linksCmd(),
			backlinksCmd(),
			orphansCmd(),
			// write (to a target)
			writeCmd(),
			appendCmd(),
			// transport & integrity (over the scope's brains)
			syncCmd(),
			statusCmd(),
			lintCmd(),
		},
	}
}

func listCmd() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls"},
		Usage:   "index: every note and its summary, across all source brains",
		Flags:   withBrain(&cli.BoolFlag{Name: "json"}),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			notes, err := sc.index()
			if err != nil {
				return err
			}
			if cmd.Bool("json") {
				return printJSON(notes)
			}
			printScopedNotes(notes, sc.multiSource())
			return nil
		},
	}
}

func summaryCmd() *cli.Command {
	return &cli.Command{
		Name:      "summary",
		Aliases:   []string{"get"},
		Usage:     "print one note's summary",
		ArgsUsage: "<note>",
		Flags:     withBrain(),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			sb, rel, err := sc.resolveNote(cmd.Args().First())
			if err != nil {
				return err
			}
			n, err := sb.Load(rel)
			if err != nil {
				return err
			}
			if strings.TrimSpace(n.FM.Summary) == "" {
				fmt.Println("(no summary)")
			} else {
				fmt.Println(n.FM.Summary)
			}
			return nil
		},
	}
}

func fmCmd() *cli.Command {
	return &cli.Command{
		Name:      "fm",
		Aliases:   []string{"frontmatter"},
		Usage:     "print one note's full front-matter block",
		ArgsUsage: "<note>",
		Flags:     withBrain(),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			sb, rel, err := sc.resolveNote(cmd.Args().First())
			if err != nil {
				return err
			}
			n, err := sb.Load(rel)
			if err != nil {
				return err
			}
			fmt.Printf("# %s:%s\n%s\n", sb.Name, n.Rel, n.RawFM)
			return nil
		},
	}
}

func readCmd() *cli.Command {
	return &cli.Command{
		Name:      "read",
		Usage:     "the deliberate full read: print a note's body",
		ArgsUsage: "<note>",
		Flags:     withBrain(&cli.BoolFlag{Name: "full", Usage: "include the front matter"}),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			sb, rel, err := sc.resolveNote(cmd.Args().First())
			if err != nil {
				return err
			}
			n, err := sb.Load(rel)
			if err != nil {
				return err
			}
			if cmd.Bool("full") && n.HasFM {
				fmt.Printf("---\n%s\n---\n", n.RawFM)
			}
			fmt.Print(n.Body)
			if !strings.HasSuffix(n.Body, "\n") {
				fmt.Println()
			}
			return nil
		},
	}
}

func searchCmd() *cli.Command {
	return &cli.Command{
		Name:      "search",
		Aliases:   []string{"grep"},
		Usage:     "notes whose path/summary/tags match a query, across all sources",
		ArgsUsage: "<query>",
		Flags: withBrain(
			&cli.BoolFlag{Name: "body", Usage: "also match note bodies"},
			&cli.BoolFlag{Name: "json"},
		),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			q := strings.Join(cmd.Args().Slice(), " ")
			if strings.TrimSpace(q) == "" {
				return errors.New("usage: multi search <query>")
			}
			notes, err := sc.search(q, cmd.Bool("body"))
			if err != nil {
				return err
			}
			if cmd.Bool("json") {
				return printJSON(notes)
			}
			printScopedNotes(notes, sc.multiSource())
			return nil
		},
	}
}

func findCmd() *cli.Command {
	return &cli.Command{
		Name:  "find",
		Usage: "structured query by type/status/tag, across all sources",
		Flags: withBrain(
			&cli.StringFlag{Name: "type", Usage: "filter by type (reference|moc|decision|hub|meta)"},
			&cli.StringFlag{Name: "status", Usage: "filter by status (active|draft|deprecated)"},
			&cli.StringSliceFlag{Name: "tag", Usage: "require tag (repeatable)"},
			&cli.BoolFlag{Name: "json"},
		),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			notes, err := sc.find(brain.FindFilter{
				Type:   cmd.String("type"),
				Status: cmd.String("status"),
				Tags:   splitCSV(cmd.StringSlice("tag")),
			})
			if err != nil {
				return err
			}
			if cmd.Bool("json") {
				return printJSON(notes)
			}
			printScopedNotes(notes, sc.multiSource())
			return nil
		},
	}
}

func linksCmd() *cli.Command {
	return &cli.Command{
		Name:      "links",
		Usage:     "outgoing wikilink targets of a note",
		ArgsUsage: "<note>",
		Flags:     withBrain(),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			sb, rel, err := sc.resolveNote(cmd.Args().First())
			if err != nil {
				return err
			}
			links, err := sb.Links(rel)
			if err != nil {
				return err
			}
			for _, l := range links {
				fmt.Println(l)
			}
			return nil
		},
	}
}

func backlinksCmd() *cli.Command {
	return &cli.Command{
		Name:      "backlinks",
		Usage:     "notes that link to a note (within its brain)",
		ArgsUsage: "<note>",
		Flags:     withBrain(),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			sb, rel, err := sc.resolveNote(cmd.Args().First())
			if err != nil {
				return err
			}
			links, err := sb.Backlinks(rel)
			if err != nil {
				return err
			}
			for _, l := range links {
				fmt.Println(l)
			}
			return nil
		},
	}
}

func orphansCmd() *cli.Command {
	return &cli.Command{
		Name:  "orphans",
		Usage: "content notes nothing links to, across all sources",
		Flags: withBrain(),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			orphans, err := sc.orphans()
			if err != nil {
				return err
			}
			for _, o := range orphans {
				if sc.multiSource() {
					fmt.Printf("%-14s %s\n", o.Brain, o.Path)
				} else {
					fmt.Println(o.Path)
				}
			}
			return nil
		},
	}
}

func splitCSV(in []string) []string {
	var out []string
	for _, v := range in {
		for _, part := range strings.Split(v, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}
