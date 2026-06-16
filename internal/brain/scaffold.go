package brain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Init scaffolds a new brain at root: governance docs, templates, the per-brain
// settings, and (optionally) a git repository with an initial commit. Existing
// files are never overwritten.
func Init(root string, s Settings, withGit bool) (*Brain, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	for rel, content := range scaffoldFiles(s) {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(abs); err == nil {
			continue // never clobber
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return nil, err
		}
	}

	b := &Brain{Root: mustAbs(root), Settings: s}
	if err := b.SaveSettings(); err != nil {
		return nil, err
	}

	if withGit {
		if err := InitGit(b.Root); err != nil {
			return nil, err
		}
		if err := b.Commit(nil, "init: scaffold brain"); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func mustAbs(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func scaffoldFiles(s Settings) map[string]string {
	name := s.Name
	if name == "" {
		name = "Brain"
	}

	var splitRule, splitTable, splitTags string
	if len(s.Split) > 0 {
		splitTags = strings.Join(s.Split, " / ")
		splitRule = fmt.Sprintf("- **One split.** Every content note carries exactly one of the split tags `%s`, and lives under the matching top-level directory.\n", strings.Join(s.Split, "`, `"))
		var rows strings.Builder
		for _, half := range s.Split {
			rows.WriteString(fmt.Sprintf("| `%s` | `%s/` | `%s` |\n", half, Slugify(half), half))
		}
		splitTable = "| Half | Directory | Tag |\n|------|-----------|-----|\n" + rows.String()
	} else {
		splitTags = "(none configured)"
		splitTable = "_No split configured for this brain. Set one with `multi brain split a,b`._\n"
	}

	repl := strings.NewReplacer(
		"{{NAME}}", name,
		"{{DATE}}", time.Now().Format("2006-01-02"),
		"{{SPLIT_RULE}}", splitRule,
		"{{SPLIT_TABLE}}", splitTable,
		"{{SPLIT_TAGS}}", splitTags,
	)

	files := map[string]string{
		"readme.md":                    tplREADME,
		"read.md":                      tplRead,
		"write.md":                     tplWrite,
		"conventions.md":               tplConventions,
		"code-of-conduct.md":           tplCodeOfConduct,
		"home.md":                      tplHome,
		"templates/reference-note.md":  tplReference,
		"templates/decision-record.md": tplDecision,
		"templates/hub-note.md":        tplHub,
		"templates/map-of-content.md":  tplMOC,
		".gitignore":                   tplGitignore,
		".gitattributes":               tplGitattributes,
	}
	for k, v := range files {
		files[k] = repl.Replace(v)
	}
	return files
}

const tplREADME = `---
type: meta
status: active
tags: [meta, readme, entrypoint]
created: {{DATE}}
summary: Entry point for the {{NAME}} brain — the standing rules, and where readers and writers go next
---

# {{NAME}} — Start Here

A navigable, trustworthy markdown second brain for humans and AI agents. Decide a note's
relevance from its one-line ` + "`summary`" + ` before opening the body.

## Prime yourself first

Read this README, then branch:

- **Only reading / looking something up?** → read [[read]] (the reading protocol).
- **Writing, ingesting, or editing?** → read [[write]] (the writing protocol — mandatory).

## The standing rules (in brief)

- **Summary first.** Every note has a one-line ` + "`summary`" + `; judge relevance by it and open the body only when it confirms relevance. Never bulk-read while exploring.
{{SPLIT_RULE}}- **Freshness.** Every content note records where its information came from and how current it is.
- **Hubs & links.** Each subject has one hub note; the ` + "`[[wikilink]]`" + ` graph is the index, not folders.

## The tool

Use ` + "`multi`" + ` (this brain's CLI) to read summaries and write notes without opening whole files:

` + "```bash" + `
multi list                 # index: every note + its summary
multi summary "<note>"     # one note's summary
multi search "<query>"     # match path / summary / tags
multi read "<note>"        # the deliberate full read
multi write --title "..." --summary "..." --tags ...   # create a note (summary enforced)
multi sync                 # pull + commit + push (transport you don't think about)
multi lint                 # verify the standing rules hold
` + "```" + `

## Table of contents

- [[home]] — the dashboard / top Map of Content
- [[conventions]] · [[code-of-conduct]] · [[read]] · [[write]]
`

const tplRead = `---
type: meta
status: active
tags: [meta, protocol, reading]
created: {{DATE}}
summary: Reading protocol — orient first, judge relevance by a note's summary, and read a full note only once the summary confirms it
---

# read.md — How to Read the {{NAME}} Brain

> Read this before reading anything else. If you will also write or ingest, read [[write]] too.

This brain grows over time. Reading whole notes to find out whether they matter buries the signal.
So we read in two stages: **summary first, full text only on confirmed relevance.**

## The standing rule (reading)

**Every note carries a one-line ` + "`summary`" + ` in its front matter. Use the summary — not the body —
to decide whether a note is relevant. Read the full note only after its summary confirms relevance.**

## The reading loop

1. **Orient.** Start at [[readme]] → [[home]], then follow ` + "`[[wikilinks]]`" + ` to narrow down.
2. **Scan summaries, not bodies:**
   - everything at once: ` + "`multi list`" + `
   - one note: ` + "`multi summary \"<note>\"`" + `
   - by keyword: ` + "`multi search \"<query>\"`" + `
   - full front matter: ` + "`multi fm \"<note>\"`" + `
3. **Decide.** If the summary confirms the note holds what you need → read the body. Otherwise move on.
4. **Read the body** (` + "`multi read \"<note>\"`" + `) only for the few notes that survived step 3.

## Related

- [[readme]] · [[write]] · [[conventions]] · [[home]]
`

const tplWrite = `---
type: meta
status: active
tags: [meta, protocol, writing]
created: {{DATE}}
summary: Writing/ingestion protocol — every note needs a summary and (for content notes) a split tag and freshness; link the hub and re-evaluate the summary on edits
---

# write.md — How to Write & Ingest into the {{NAME}} Brain

> Read this before writing, ingesting, or editing. If you only intend to read, read [[read]].

Every note must stay machine- and human-navigable so readers can judge it by its summary alone.
That contract is your responsibility on every write — and ` + "`multi write`" + ` enforces the core of it.

## Required front matter

` + "```yaml" + `
---
type: reference        # moc | reference | decision | hub | meta
status: active         # active | draft | deprecated
tags: [<split>, ...]   # content notes carry one split tag ({{SPLIT_TAGS}}) + topic tags
created: <YYYY-MM-DD>   # auto-filled by ` + "`multi write`" + `
summary: One-line description of the note's contents — the gate readers read first
source: ...            # where the information came from
retrieved: <YYYY-MM-DD># date pulled into the brain (auto-filled)
freshness: ...         # one-line read of how current/trustworthy this is
---
` + "```" + `

### A good summary
- **One line.** No line breaks.
- **About contents, not the title.** A reader decides relevance from this line alone.
- **Concrete and specific.** Name the scope and the distinguishing detail.

## Create / update

- **Create:** ` + "`multi write --title \"...\" --summary \"...\" --tags <split>,topic --source \"...\" --freshness \"...\"`" + `. Then link it from its MOC and hub.
- **Update:** edit the body, then **re-evaluate the ` + "`summary`" + `** against it. A stale summary is a bug.

## Verify before you finish

` + "```bash" + `
multi lint        # summary on every note; split tag + freshness on every content note
` + "```" + `

## Related

- [[code-of-conduct]] · [[readme]] · [[read]] · [[conventions]] · [[home]]
`

const tplConventions = `---
type: meta
status: active
tags: [meta, conventions]
created: {{DATE}}
summary: How the {{NAME}} brain is organized — front-matter schema, the split, note types, linking, and where large files go
---

# Conventions

## Front-matter schema

` + "```yaml" + `
type: reference        # moc | reference | decision | hub | meta
status: active         # active | draft | deprecated
tags: [<split>, ...]   # one split tag on content notes + topic tags
created: <YYYY-MM-DD>
summary: One-line description used in previews, search, and indexes
source: ...            # provenance
source_created: ...    # source creation date (when known)
source_updated: ...    # source last-change date (when known)
retrieved: <YYYY-MM-DD>
freshness: ...         # one-line read of currency/trust
` + "```" + `

` + "`type`" + ` values: ` + "`moc`" + ` (index), ` + "`reference`" + ` (default), ` + "`decision`" + `, ` + "`hub`" + ` (single entry note for an entity), ` + "`meta`" + ` (governance).

## The split

{{SPLIT_TABLE}}

## Linking

- Connect notes with ` + "`[[wikilinks]]`" + `; aliases for prose (` + "`[[Long Name|short]]`" + `).
- Forward links to not-yet-written notes are encouraged — a backlog signal.
- MOCs are the index, not folders. A content note nothing links to is an orphan (` + "`multi orphans`" + `).

## Large files

Markdown is the source of truth and lives in git. **Large binaries (PDFs, images, archives) do not
belong in git** — they bloat every clone forever. Keep them in object storage and link them; see
` + "`.gitignore`" + `. Only small note-embedded assets belong in the repo.

## Tooling

` + "`multi lint`" + ` after any write. It is the source of truth for the standing rules.
`

const tplCodeOfConduct = `---
type: meta
status: active
tags: [meta, conduct]
created: {{DATE}}
summary: The mandatory rules for the {{NAME}} brain — summary, split, freshness, hubs, and the link graph as index
---

# Code of Conduct

The non-negotiable rules. Details live in [[conventions]]; protocols in [[read]] and [[write]].

1. **Summary.** Every note has a single-line ` + "`summary`" + ` in front matter.
2. **Split.** Every content note is tagged with exactly one split tag ({{SPLIT_TAGS}}).
3. **Freshness.** Every content note records ` + "`source`" + `, ` + "`retrieved`" + `, and ` + "`freshness`" + `.
4. **Hubs.** Each subject has one hub/entry note; related notes link to it.
5. **Graph is the index.** Navigation is by ` + "`[[wikilinks]]`" + `, not folder spelunking.
`

const tplHome = `---
type: moc
status: active
tags: [moc, home]
created: {{DATE}}
summary: The dashboard and top Map of Content for the {{NAME}} brain — start navigation here
---

# {{NAME}} — Home

The top-level Map of Content. Start here and follow ` + "`[[wikilinks]]`" + ` outward.

## Governance

- [[readme]] · [[read]] · [[write]] · [[conventions]] · [[code-of-conduct]]

## Areas

_Add Maps of Content for your areas here as the brain grows._
`

const tplReference = `---
type: reference
status: active
tags: []
created: {{DATE}}
source: <where this came from>
retrieved: {{DATE}}
freshness: <one-line currency note>
summary: One-line description of the note's contents — what a reader learns here
---

# {{title}}

One paragraph stating what this is.

## Details

The substance. Cite sources; do not paste large documents — summarize and link.

## Related

- [[ ]]
`

const tplDecision = `---
type: decision
status: active
tags: []
created: {{DATE}}
source: <meeting / minute / resolution>
retrieved: {{DATE}}
freshness: <currency note>
summary: One-line summary of the decision and its outcome
---

# {{title}}

## Context

The situation and the forces that made a decision necessary.

## Decision

What was decided, by whom, and when.

## Alternatives considered

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| | | | |

## Consequences

What this enables, what it costs, what to revisit later.

## Related

- [[ ]]
`

const tplHub = `---
type: hub
status: active
tags: []
created: {{DATE}}
source: <primary source for this entity>
retrieved: {{DATE}}
freshness: <currency note>
summary: One-line description of this entity and what it is the hub for
---

# {{title}}

The single entry point for everything about this entity — all related notes link here.

## What it is

One paragraph.

## Key facts

| | |
|---|---|
| | |

## Related

- [[ ]]
`

const tplMOC = `---
type: moc
status: active
tags: []
created: {{DATE}}
summary: Map of Content for <area> — the index note linking its related notes
---

# {{title}}

What this area covers, in one paragraph.

## Notes

- [[ ]]
`

const tplGitignore = `.DS_Store
.obsidian/workspace*
.obsidian/cache

# multi's local cache (never synced)
.multi/cache/

# Large binaries belong in object storage, not git. Uncomment and adjust the
# attachment path(s) for this brain so clones stay small:
# **/_attachments/
# **/*.pdf
# **/*.mp4
`

const tplGitattributes = `# Markdown notes union-merge: concurrent edits concatenate instead of
# conflict-markering. With one-fact-per-file discipline, true clashes are rare.
*.md merge=union
`
