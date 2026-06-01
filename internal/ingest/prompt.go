package ingest

import (
	"fmt"
	"strings"
)

// BuildJobPrompt assembles the markdown instruction file the steered agent reads. It
// carries three things: the brain-safety rules, the per-session continuity ledger
// (notes already touched this session, so the agent extends rather than duplicates),
// and the new conversation delta to integrate. The agent signals completion by
// writing the report file with an INGEST-STATUS trailer.
func BuildJobPrompt(j Job, l *Ledger, digest, reportPath string) string {
	var prior strings.Builder
	if len(l.TouchedNotes) == 0 {
		prior.WriteString("_(none yet — this is the first integration run for this session)_\n")
	} else {
		for _, n := range l.TouchedNotes {
			fmt.Fprintf(&prior, "- `%s` — %s (%s)\n", n.Path, n.Summary, n.Action)
		}
	}

	return fmt.Sprintf(`# DeepThought ingestion job

You are integrating the learnings of a just-finished Claude Code session into the
**deep-thought** brain. This is NOT "evaluate, then drop a new file somewhere." Your
job is to weave durable knowledge into the EXISTING note graph: extend related notes,
add bidirectional wikilinks, keep MOC/Atlas maps current, and create a new note only
when nothing suitable exists.

## Brain safety (non-negotiable)
- Every brain command MUST be pinned: `+"`multi --brain %s …`"+` (the `+"`--brain`"+` flag goes
  BEFORE the subcommand). A bare `+"`multi`"+` targets the wrong (active) brain.
- NEVER use the `+"`obsidian`"+` CLI and never touch ~/Nextcloud/DeepThought.
- All structured writes go through `+"`multi --brain %s write/append`"+`. For surgical edits
  of an existing note, edit the file under the brain dir directly, then let multi commit.

## Already integrated THIS session — continue, don't duplicate
%s
When the new content below belongs in one of these notes, EXTEND that note instead of
making a new one.

## Integration procedure
1. Read the conversation delta at the end of this file.
2. Identify durable learnings (decisions, patterns, gotchas, research outcomes,
   corrections, deployment knowledge). Skip routine/no-insight chatter.
3. For each, explore the neighborhood first:
   - `+"`multi --brain %s list --json`"+`            (full index + summaries)
   - `+"`multi --brain %s search \"<topic>\" --body`"+` , `+"`read`"+` , `+"`backlinks`"+` , `+"`links`"+`
4. Integrate:
   - If a closely related note exists → extend/rewrite it to fold in the new knowledge.
   - Add bidirectional `+"`[[wikilinks]]`"+`: link the note to its neighbours AND update
     neighbours to link back.
   - Maintain the relevant `+"`Atlas/*-MOC.md`"+` map membership.
   - Only `+"`multi --brain %s write`"+` a NEW note when nothing fits (summary required).
5. Be conservative with rewrites — improve, don't vandalise. Everything is git-tracked,
   but aim for net-positive edits only.

## When done — write the report (this is how completion is detected)
Write your report to this exact path, OVERWRITING it:

    %s

The report MUST contain a machine-readable touched-notes block and a status trailer:

    <!-- INGEST-TOUCHED
    relative/note/path.md | created|updated|linked | one-line what-changed
    relative/other.md | updated | one-line what-changed
    -->

    (a short human summary of what you integrated, or "nothing worth keeping")

    INGEST-STATUS: done

Use `+"`multi --brain %s sync`"+` as your final step to push. Write the report LAST.

---

## Conversation delta to integrate

%s
`,
		BrainName, BrainName,
		prior.String(),
		BrainName, BrainName,
		BrainName,
		reportPath,
		BrainName,
		digest,
	)
}
