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

## Ingester — automatic session-end integration (`ingester`)

A companion binary that runs on Claude Code's **Stop** hook and weaves each session's
learnings into a brain — **extending and cross-linking existing notes** instead of
dumping standalone files. It keeps a per-session **ledger** (transcript cursor + the
notes touched so far) so repeated Stop firings *continue* one integration rather than
duplicating, and it does the actual editing by steering a dedicated, **subscription-
backed** interactive Claude session over tmux (so it never spends Agent-SDK / API
credit).

Flow: `Stop → ingester hook` (enqueue + kick the dispatcher, returns instantly) →
detached `ingester dispatch` (single-flight via `flock`) computes the transcript delta,
steers the tmux session to integrate it through `multi --brain <brain>`, waits for a
sentinel report, updates the ledger, then `multi sync`s. A job that stalls times out
into a dead-letter queue — nothing is lost and the host session never blocks.

```
ingester hook       # Stop-hook entry: reads hook JSON on stdin (fast, non-blocking)
ingester dispatch   # detached worker; drains the queue (one integration per session)
ingester run --session <id> --transcript <path>   # one cycle in the foreground (debug)
ingester status     # queue, tmux session, ledgers
ingester session ensure | kill                     # manage the tmux session
```

### Local install (LLM-friendly runbook)

Prerequisites: `go` ≥ 1.26, `tmux`, the `multi` binary, a registered **target brain**
(`multi brain list` shows it), and `claude` (Claude Code) logged in on a subscription.

1. **Build & install both binaries** (ingester ships alongside multi):
   ```bash
   cd <multiverse-repo>
   just install            # → $GOBIN/multi and $GOBIN/ingester  (e.g. ~/go/bin)
   ```
2. **Choose the target brain.** The ingester writes to the brain named `deep-thought`
   by default — see `const BrainName` in `internal/ingest/brain.go`. To target another,
   change that constant and re-run `just install`. Confirm it resolves:
   ```bash
   multi brain list        # the target brain must be registered
   ingester status         # prints the home dir, a (down) session, and 0 jobs
   ```
3. **Wire the Stop hook.** Add this object to the `Stop` array in
   `~/.claude/settings.json` (keep any existing entries; use an absolute path):
   ```json
   { "type": "command", "command": "/ABSOLUTE/PATH/TO/ingester hook", "timeout": 10 }
   ```
4. **Recursion guard — already built in, nothing to do.** The steered session is
   launched with `DEEPTHOUGHT_INGEST=1` and a hooks-disabled settings file, and
   `ingester hook` no-ops whenever that variable is set, so ingestion can never trigger
   itself.
5. **Verify steering** without a real integration:
   ```bash
   ingester session ensure              # launches the tmux Claude session (subscription)
   tmux attach -t deepthought-ingest     # watch it; Ctrl-b then d to detach
   ingester session kill
   ```
6. **Full dry-run** on a throwaway transcript (a trivial one should report
   "nothing worth keeping"):
   ```bash
   ingester run --session test --transcript <some-session>.jsonl
   cat ~/.claude/ingester/reports/*_test.md     # ends with:  INGEST-STATUS: done
   cat ~/.claude/ingester/state/test.json        # the ledger
   ```

Data lives under `~/.claude/ingester/` (override via `INGESTER_HOME`): `state/` ledgers ·
`queue/`(+`done`,`failed`) jobs · `reports/` sentinels · `prompts/` per-job instructions ·
`dispatch.log`. Model: **Sonnet** (interactive = subscription quota). To disable, remove
the Stop-hook entry — any queued jobs are inert without it.

## Layout

```
cmd/multi            multi entrypoint
cmd/ingester         ingester entrypoint (session-end integration)
internal/brain       Brain, Note/front matter, index/search, graph, lint, git, scaffold
internal/cli         urfave/cli v3 command tree
internal/config      ~/.config/multi registry of brains
internal/ingest      ingester: ledger, transcript delta, tmux steering, dispatcher
internal/tui         Bubble Tea control panel
```

## Roadmap (deferred from v1)

- Object-storage attachment path (`mv`-free large-file handling).
- Embedding / semantic search + an MCP server surface for agents.
- Background sync daemon (watch + debounced sync) so reads are always current.
- Safe front-matter updates that preserve unknown keys.
```
