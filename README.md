# multiverse — `multi`

A git-backed markdown **second brain** for agents and humans. `multi` is the
storage-layer CLI: it hides git transport behind `read` / `write` / `search` so
an agent never thinks about sync, and it enforces the conventions that keep a
brain navigable — above all, **every note has a one-line summary**.

## Why

- **Markdown is the source of truth.** Plain `.md` in a git repo. No database, no
  proprietary store, no obfuscation — readable and editable anywhere (Obsidian, an
  editor, `grep`).
- **Git is the transport.** Real conflict resolution (`*.md merge=union`), offline
  commits, full history/audit. The agent calls `read`/`write`/`search`; `multi`
  does pull/commit/push underneath.
- **Writes are guaranteed structured.** `multi write` refuses without a `summary`,
  auto-fills `created`/`retrieved`, and auto-commits. The "never forgotten"
  guarantee is enforced at the only write path.
- **Headless.** No Obsidian app required — works on servers where agents run.

## Install

```bash
just install          # go install ./cmd/multi  →  $GOBIN/multi
# or
just build            # → ./bin/multi
```

## Control panel (TUI)

```bash
multi                 # no args on a terminal → launches the TUI
multi tui             # explicit (aliases: dashboard, ui)
```

A Bubble Tea control panel with three tabbed views:

- **Dashboard** — every brain with git state (branch · clean/dirty · ↑ahead ↓behind), note count, and lint status; plus the resolved scope for the current directory. Keys: `s` sync · `S` sync all · `l`/`L` lint · `r` refresh · `enter` set active.
- **Brains** — manage the global registry: `a` add · `e` rename · `p` set path · `d` delete · `enter` set active.
- **Scope** — bind the current directory: toggle each brain as `space` source / `t` target, then `w` to write `./.multi.yaml` (`c` clears it).

`tab`/`shift+tab` switch views, `q` quits. On a non-interactive shell (agent/CI), `multi` with no args prints help instead of launching.

## Onboarding

```bash
multi onboard                                   # interactive: new or clone
multi init ~/vaults/mybrain --name mybrain --split domain,operations
multi clone git@host:me/brain.git ~/vaults/brain
multi brain list            # registered brains (* = active)
multi brain use mybrain     # set active brain
```

`init` scaffolds the governance docs (`README`, `read`, `write`, `Conventions`,
`Code of Conduct`, `Home`), `Templates/`, a `.gitignore` that keeps large
binaries out of git, and a `.gitattributes` with `*.md merge=union` — then
`git init`s with an initial commit.

## Per-directory scope (multiple brains)

The directory you work in declares which brains it **reads from** (sources) and
**writes to** (targets) via a `.multi.yaml`, walked up the tree like `.git`:

```bash
cd ~/github.com/asolabs/qvm-website
multi use qvm                       # read+write the qvm brain here and below
multi use qvm --read-only           # read-only: no writes land here
multi scope set --source qvm,deepthought --target qvm   # read both, write only qvm
multi scope                         # show the resolved scope
multi scope clear                   # remove ./.multi.yaml
```

`.multi.yaml`:

```yaml
sources: [qvm, deepthought]   # read commands span all of these
targets: [qvm]                # writes land here (first = default); omit → = sources
read_only: true               # no write targets at all (overrides targets)
```

With a scope active, **reads span every source** (results labeled by brain) and
**writes go to the target**:

```bash
multi list                    # notes from qvm AND deepthought, brain-labeled
multi search "iso 17024"      # searches all sources
multi read deepthought:Home   # brain:note qualifier when a name exists in several
multi write --title ... --summary ...   # lands in qvm (the target)
multi write -b deepthought --title ...  # override target for one write
multi sync                    # syncs every brain in scope
```

## Resolution order

Each command resolves its scope as: `--brain <name|path>` (one brain, overrides
everything) → nearest `.multi.yaml` → the brain the cwd sits inside → the active
brain in the registry.

## For agents / LLMs

`multi` is meant to be driven directly by shell-capable agents (Claude Code,
Hermes) — no MCP server needed. To make one fluent in one shot:

```bash
multi guide              # compact mental model + cheatsheet (the read→write→sync loop)
multi guide --claude-md  # emit a CLAUDE.md block to paste into a project repo
```

Errors are self-correcting (they say how to fix), and read commands take `--json`.

## The agent loop

```bash
# read — summary first, body only on confirmed relevance
multi list                          # index: every note + its summary
multi summary "QVM Overview"        # one note's summary
multi fm "QVM Overview"             # full front-matter block
multi search "iso 17024" [--body]   # match path/summary/tags (+ bodies)
multi find --type reference --tag domain --status active
multi read "QVM Overview"           # the deliberate full read
multi links / backlinks "..." / orphans

# write — structured + auto-committed
multi write --title "Mediation Basics" --dir domain \
  --summary "what mediation is and why QVM certifies it" \
  --tags domain,mediation --source "..." --freshness "current" \
  --body "..."                      # or --stdin
multi append "Mediation Basics" --content "addendum"   # or --stdin

# transport & integrity
multi sync [-m "msg"]               # commit local → pull --rebase → push
multi status
multi lint [--summary|--tags|--fresh] [--json]
```

Most read commands accept `--json` for machine consumption.

## The brain conventions

Front matter (`multi write` enforces the core; `multi lint` checks all):

```yaml
type: reference        # moc | reference | decision | hub | meta
status: active         # active | draft | deprecated
tags: [<split>, ...]   # content notes carry one split tag + topic tags
created: <YYYY-MM-DD>   # auto-filled
summary: One-line description — the gate readers read first
source: ...            # provenance
retrieved: <YYYY-MM-DD># auto-filled
freshness: ...         # one-line currency/trust note
```

**Standing rules** (enforced by `multi lint`): every note has a `summary`; every
*content* note carries one split tag and records `source`/`retrieved`/`freshness`;
hubs and the `[[wikilink]]` graph are the index, not folders.

## Large files

Markdown lives in git. **Large binaries (PDFs, images, archives) do not** — they
bloat every clone forever. Keep them in object storage and link them (see a
brain's `.gitignore`). Object-storage attachment handling is a planned follow-up.

## Layout

```
cmd/multi            entrypoint
internal/brain       Brain, Note/front matter, index/search, graph, lint, git, scaffold
internal/cli         urfave/cli v3 command tree
internal/config      ~/.config/multi registry of brains
```

## Roadmap (deferred from v1)

- Object-storage attachment path (`mv`-free large-file handling).
- Embedding / semantic search + an MCP server surface for agents.
- Background sync daemon (watch + debounced sync) so reads are always current.
- Safe front-matter updates that preserve unknown keys.
```
