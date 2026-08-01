package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func guideCmd() *cli.Command {
	return &cli.Command{
		Name:    "guide",
		Aliases: []string{"agent", "llm"},
		Usage:   "print a compact usage guide for agents/LLMs (--claude-md emits a CLAUDE.md block)",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "claude-md", Usage: "emit a CLAUDE.md section to paste into a project"},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.Bool("claude-md") {
				fmt.Print(guideClaudeMD)
			} else {
				fmt.Print(guideText)
			}
			return nil
		},
	}
}

const guideText = `multi — a git-backed markdown second brain. You read from and write to "brains"
(git repos of markdown notes). Git transport is hidden; you never run git yourself.

MENTAL MODEL
- A brain is a directory of markdown notes under git. Every note has YAML front
  matter; its one-line ` + "`summary`" + ` is the gate — judge relevance by the summary
  before reading a body.
- The directory you work in is bound (via ./.multi.yaml, walked up like .git) to
  SOURCES (brains you read) and TARGETS (brains you write). Reads span all sources;
  writes land in a target.
- Scope resolution per command: --brain <name|path> overrides everything, else the
  nearest ./.multi.yaml, else the brain the cwd sits inside, else the active brain.
  Run ` + "`multi scope`" + ` to see what applies here.

CORE LOOP
  boot     multi wake-up                    session bootstrap: identity notes + pinned facts (run FIRST)
  orient   multi list                       index: every note + summary (all sources)
           multi search "<terms>"           ranked; ALL terms must match (path>tags>summary; --body adds bodies)
           multi find --type reference --tag <tag>
           multi similar "<query>"          semantic: matches meaning, not words (needs one reindex)
           multi similar --note "<note>"    nearest neighbors: dedup & related notes
  read     multi summary "<note>"           the one-line gate
           multi read "<note>"              the deliberate full body read
           multi fm "<note>"                full front matter
  write    multi write --title "<t>" --summary "<one line>" --tags <tag> \
                 --source "<where>" --freshness "<currency>" --body "<md>"
           multi write --pinned ...         pin a critical fact into wake-up
           multi append "<note>" --content "<md>"
  graph    multi backlinks "<note>" · multi links "<note>" · multi orphans
  sync     multi sync                       commit + pull + push every brain in scope
  check    multi lint                       verify summary / split tag / freshness / kebab-case / taxonomy
  fix      multi fix                        rename files+dirs to kebab-case & rewrite links (--dry-run)
  index    multi reindex                    refresh the semantic shadow index (incremental, disposable)

WRITE CONTRACT (enforced)
- --summary is REQUIRED: one line, about the note's contents (not its title).
- type: moc|reference|decision|hub|meta (default reference); status: active|draft|deprecated.
  A brain with a closed taxonomy (.multi/brain.yaml) rejects values outside it — the
  error lists what's allowed.
- A write that looks like a near-duplicate of an existing note prints a warning with
  candidates — prefer ` + "`multi append`" + ` to writing the same fact under a new name.
- Content notes should carry their split tag plus source/retrieved/freshness; ` + "`multi lint`" + ` checks.
- created/retrieved are auto-filled; every write auto-commits.
- Filenames are forced to kebab-case (lowercase-with-hyphens) for cross-platform safety:
  --title "Formula Student" lands at .../formula-student.md, while the in-note H1 stays human.
  Reference notes by name in any case ("Formula Student" or "formula-student") — both resolve.

AGENT TIPS
- Add --json to list / search / find for structured output.
- A note that exists in several brains: qualify it, e.g. ` + "`multi read personal:\"Home\"`" + `.
- One fact per file: write small, append-friendly notes; never co-edit a shared note.
- "no brain in scope" → run ` + "`multi use <brain>`" + ` here, or pass --brain <name>.
`

const guideClaudeMD = `## Knowledge base: ` + "`multi`" + `

This project is bound to one or more ` + "`multi`" + ` brains (git-backed markdown second brains).
Search/read the brain before answering; capture durable findings as notes. You never run git —
` + "`multi`" + ` handles commit/sync.

- Session start: ` + "`multi wake-up`" + ` (identity notes + pinned facts — run it first).
- Discover scope: ` + "`multi scope`" + ` (shows sources = read, targets = write).
- Find:  ` + "`multi search \"<terms>\" --json`" + ` (ranked, ALL terms must match) · ` + "`multi similar \"<query>\"`" + ` (semantic) · ` + "`multi list --json`" + ` · ` + "`multi find --type reference --tag <tag> --json`" + `
- Read:  ` + "`multi summary \"<note>\"`" + ` (the one-line gate) · ` + "`multi read \"<note>\"`" + ` (full body)
- Write (auto-committed; ` + "`--summary`" + ` is REQUIRED, one line about contents):
  ` + "`multi write --title \"<t>\" --summary \"<one line>\" --tags <tag> --source \"<where>\" --freshness \"<currency>\" --body \"<md>\"`" + `
- Append to a note: ` + "`multi append \"<note>\" --content \"<md>\"`" + `
- Sync everything in scope: ` + "`multi sync`" + `

Conventions: judge relevance by ` + "`summary`" + ` before reading bodies; one fact per file; never co-edit
a shared note; a name in multiple brains is qualified as ` + "`brain:note`" + `.
`
