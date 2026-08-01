// Recall commands: the session bootstrap (wake-up) and the semantic shadow
// index (reindex, similar). Search stays the lexical workhorse; these cover
// what substring matching can't — session priming and meaning-based recall.
package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mxcd/multiverse/internal/brain"
	"github.com/urfave/cli/v3"
)

func wakeupCmd() *cli.Command {
	return &cli.Command{
		Name:    "wake-up",
		Aliases: []string{"wakeup"},
		Usage:   "print the session bootstrap: identity notes in full, pinned facts as one-liners",
		Description: "Emits a small, deterministic context block to load at session start: the\n" +
			"brain's `wakeup:` notes (L0, printed in full — keep them short) followed by\n" +
			"every note with `pinned: true` front matter as `path | summary` lines (L1).",
		Flags: withBrain(&cli.BoolFlag{Name: "json"}),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			type brainWakeup struct {
				Brain    string                `json:"brain"`
				Sections []brain.WakeupSection `json:"sections,omitempty"`
				Facts    []brain.NoteInfo      `json:"facts,omitempty"`
			}
			var all []brainWakeup
			for _, sb := range sc.Sources {
				sections, facts, err := sb.Wakeup()
				if err != nil {
					return fmt.Errorf("%s: %w", sb.Name, err)
				}
				if len(sections) > 0 || len(facts) > 0 {
					all = append(all, brainWakeup{Brain: sb.Name, Sections: sections, Facts: facts})
				}
			}
			if cmd.Bool("json") {
				return printJSON(all)
			}
			if len(all) == 0 {
				fmt.Println("(nothing to wake up with: list notes under `wakeup:` in .multi/brain.yaml or pin notes with `pinned: true`)")
				return nil
			}
			for _, bw := range all {
				fmt.Printf("# wake-up: %s\n", bw.Brain)
				for _, s := range bw.Sections {
					fmt.Printf("\n## %s\n%s", s.Path, s.Body)
					if !strings.HasSuffix(s.Body, "\n") {
						fmt.Println()
					}
				}
				if len(bw.Facts) > 0 {
					fmt.Printf("\n## pinned\n")
					for _, f := range bw.Facts {
						fmt.Printf("- %s | %s\n", f.Path, f.Summary)
					}
				}
			}
			return nil
		},
	}
}

func reindexCmd() *cli.Command {
	return &cli.Command{
		Name:  "reindex",
		Usage: "build/refresh the semantic shadow index (embeddings) for every brain in scope",
		Description: "Embeds each note via the brain's configured OpenAI-compatible endpoint and\n" +
			"caches one vector per note outside the repo. Incremental: unchanged notes are\n" +
			"skipped by content hash. The index is disposable — markdown stays the source\n" +
			"of truth. Brains without an `embeddings:` config are skipped.",
		Flags: withBrain(&cli.BoolFlag{Name: "force", Usage: "discard the index and re-embed everything"}),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			for _, sb := range sc.union() {
				stats, err := sb.Reindex(cmd.Bool("force"), func(line string) {
					fmt.Printf("%-14s %s\n", sb.Name, line)
				})
				if errors.Is(err, brain.ErrNoEmbeddings) {
					fmt.Printf("%-14s skipped (no embeddings config)\n", sb.Name)
					continue
				}
				if err != nil {
					return fmt.Errorf("%s: %w", sb.Name, err)
				}
				fmt.Printf("%-14s embedded %d, kept %d, removed %d\n", sb.Name, stats.Embedded, stats.Kept, stats.Removed)
			}
			return nil
		},
	}
}

func similarCmd() *cli.Command {
	return &cli.Command{
		Name:      "similar",
		Usage:     "semantic search: notes closest in meaning to a query or an existing note",
		ArgsUsage: "<query>",
		Description: "Ranks notes by cosine similarity against the shadow index built by\n" +
			"`multi reindex`. Complements `multi search`: search matches words, similar\n" +
			"matches meaning. With --note, lists the nearest neighbors of that note\n" +
			"instead (dedup and grooming's best friend).",
		Flags: withBrain(
			&cli.StringFlag{Name: "note", Usage: "find neighbors of this note instead of a text query"},
			&cli.IntFlag{Name: "top", Value: 10, Usage: "number of results"},
			&cli.BoolFlag{Name: "json"},
		),
		Action: func(_ context.Context, cmd *cli.Command) error {
			sc, err := resolveScope(cmd)
			if err != nil {
				return err
			}
			topK := int(cmd.Int("top"))

			if ref := cmd.String("note"); ref != "" {
				sb, rel, err := sc.resolveNote(ref)
				if err != nil {
					return err
				}
				hits, err := sb.SimilarNote(rel, topK)
				if err != nil {
					return err
				}
				return printSimilar(cmd, stamp(hits, sb.Name), sc.multiSource())
			}

			query := strings.TrimSpace(strings.Join(cmd.Args().Slice(), " "))
			if query == "" {
				return errors.New("usage: multi similar <query> (or --note <note>)")
			}
			var all []brain.NoteInfo
			configured := false
			for _, sb := range sc.Sources {
				hits, err := sb.Similar(query, topK)
				if errors.Is(err, brain.ErrNoEmbeddings) {
					continue
				}
				if err != nil {
					return fmt.Errorf("%s: %w", sb.Name, err)
				}
				configured = true
				all = append(all, stamp(hits, sb.Name)...)
			}
			if !configured {
				return brain.ErrNoEmbeddings
			}
			brain.SortByScore(all)
			if len(all) > topK {
				all = all[:topK]
			}
			return printSimilar(cmd, all, sc.multiSource())
		},
	}
}

func printSimilar(cmd *cli.Command, notes []brain.NoteInfo, withBrain bool) error {
	if cmd.Bool("json") {
		return printJSON(notes)
	}
	for _, n := range notes {
		s := n.Summary
		if s == "" {
			s = "(no summary)"
		}
		if withBrain {
			fmt.Printf("%.3f  %-14s %-46s | %s\n", n.Score, n.Brain, n.Path, s)
		} else {
			fmt.Printf("%.3f  %-52s | %s\n", n.Score, n.Path, s)
		}
	}
	return nil
}
