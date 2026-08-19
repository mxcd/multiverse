package ingest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GroomTimeout bounds one grooming run. Grooming touches many notes, so it
// gets more room than a single ingestion job.
const GroomTimeout = 45 * time.Minute

// Groom steers the agent through one vault-maintenance cycle: fix lint
// findings, wire orphans into the graph, merge fresh near-duplicates, refresh
// MOCs. The deterministic weekly health check only OBSERVES decay — this is
// the counterpart that repairs it. Meant to run from cron.
func Groom(timeout time.Duration, verbose bool) error {
	lock, held, err := acquireLock()
	if err != nil {
		return err
	}
	if !held {
		return fmt.Errorf("the ingestion session is busy (another dispatcher/run holds the lock); try again later")
	}
	defer releaseLock(lock)
	if err := ensureDirs(); err != nil {
		return err
	}

	// lint exits non-zero on findings and orphans can be long — both outputs
	// are input for the agent, not errors for us.
	lint := multiOut("lint")
	orphans := clipLines(multiOut("orphans"), 120)

	stamp := fileStamp() + "_groom"
	reportPath := filepath.Join(reportsDir(), stamp+".md")
	promptPath := filepath.Join(promptsDir(), stamp+".md")
	if err := os.WriteFile(promptPath, []byte(buildGroomPrompt(lint, orphans, reportPath)), 0o644); err != nil {
		return err
	}

	if err := EnsureSession(); err != nil {
		return err
	}
	if err := SteerJob(promptPath); err != nil {
		return err
	}
	if _, ok := waitForReport(reportPath, timeout); !ok {
		return fmt.Errorf("timeout waiting for groom report: %s", reportPath)
	}
	_ = SyncBrain()
	_ = ReindexBrain()
	if verbose {
		fmt.Println("grooming run complete")
	}
	return nil
}

func multiOut(args ...string) string {
	out, _ := exec.Command(multiPath(), append([]string{"--brain", BrainName}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out))
}

func clipLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + fmt.Sprintf("\n… (%d more lines — run the command yourself for the rest)", len(lines)-n)
}

// buildGroomPrompt assembles the maintenance instructions with the current
// lint and orphan state baked in, so the agent starts from facts, not scans.
func buildGroomPrompt(lint, orphans, reportPath string) string {
	return fmt.Sprintf(`# Brain grooming job

You are running the weekly maintenance cycle for the %s brain. The automated
health check only observes decay — your job is to repair it. Work the phases in
order and stay conservative: everything is git-tracked, but aim for net-positive
edits only.

## Brain safety (non-negotiable)
- Every brain command MUST be pinned: `+"`multi --brain %s …`"+` (the `+"`--brain`"+` flag goes
  BEFORE the subcommand). A bare `+"`multi`"+` targets the wrong (active) brain.
- NEVER bypass `+"`multi`"+` to edit the brain (no direct git, no other note app).
- Create every new note with `+"`multi --brain %s write`"+` (front matter is generated;
  pass --summary, --source, --freshness). Direct file edits: BODY only, never front matter.

## Phase 1 — lint findings: fix every one
`+"```"+`
%s
`+"```"+`
Missing freshness/source: derive honest values from the note's created date and
content; never fabricate. Other rule violations: repair per the rule's intent.

## Phase 2 — orphans: wire them into the graph
`+"```"+`
%s
`+"```"+`
Skip journals/ entries. For each remaining orphan: find its neighborhood
(`+"`multi --brain %s search`"+`, `+"`similar --note`"+`) and link it from the fitting
Atlas MOC or closest related note, adding the backlink in the orphan too. An
orphan that is superseded or worthless: set `+"`status: deprecated`"+` instead of linking.

## Phase 3 — near-duplicates among recent notes
For roughly the 20 newest notes (`+"`multi --brain %s list --json`"+`, sort by created):
run `+"`multi --brain %s similar --note <name>`"+`. Neighbors above ~0.85 cosine that
restate the same fact: merge into the older note (zero-loss — fold in anything
new, preserve contradictions as an "Open question" block), delete the loser, and
retarget its wikilinks.

## Phase 4 — MOC freshness
For every MOC you touched above, confirm its entries still exist and its counts
or section lists reflect reality. Fix what drifted.

## When done — write the report (this is how completion is detected)
Write your report to this exact path, OVERWRITING it:

    %s

The report MUST end with the status trailer:

    (short human summary: findings fixed, orphans linked, dupes merged, MOCs updated)

    INGEST-STATUS: done

Use `+"`multi --brain %s sync`"+` as your final step to push. Write the report LAST.
`,
		BrainName, BrainName, BrainName, // intro + safety pin + write
		lint,
		orphans,
		BrainName,            // phase 2 search
		BrainName, BrainName, // phase 3 list + similar
		reportPath,
		BrainName, // sync line
	)
}
