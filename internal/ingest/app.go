package ingest

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

// NewApp builds the root `ingester` command.
func NewApp(version string) *cli.Command {
	return &cli.Command{
		Name:    "ingester",
		Usage:   "steer an interactive Claude session to weave session learnings into the deep-thought brain",
		Version: version,
		Description: "ingester runs on Claude Code's Stop hook. It keeps a per-session ledger\n" +
			"(transcript cursor + notes touched so far) and steers a dedicated, subscription-\n" +
			"backed interactive Claude session (via tmux) to integrate new learnings into the\n" +
			"existing note graph — extending and cross-linking notes rather than dumping files.",
		Commands: []*cli.Command{
			hookCmd(),
			dispatchCmd(),
			runCmd(),
			statusCmd(),
			sessionCmd(),
		},
	}
}

func hookCmd() *cli.Command {
	return &cli.Command{
		Name:  "hook",
		Usage: "consume a Stop-hook payload on stdin, enqueue a job, kick the dispatcher (fast, non-blocking)",
		Action: func(_ context.Context, _ *cli.Command) error {
			return RunHook()
		},
	}
}

func dispatchCmd() *cli.Command {
	return &cli.Command{
		Name:  "dispatch",
		Usage: "drain the job queue (one integration per session); single-flight via flock",
		Flags: []cli.Flag{
			&cli.DurationFlag{Name: "timeout", Value: DefaultTimeout, Usage: "max wait per job for the agent's report"},
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return Dispatch(cmd.Duration("timeout"), cmd.Bool("verbose"))
		},
	}
}

func runCmd() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "run one integration cycle in the foreground (for testing/debugging)",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "session", Required: true, Usage: "session id"},
			&cli.StringFlag{Name: "transcript", Required: true, Usage: "path to the session transcript (.jsonl)"},
			&cli.DurationFlag{Name: "timeout", Value: DefaultTimeout},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			lock, held, err := acquireLock()
			if err != nil {
				return err
			}
			if !held {
				return cli.Exit("the ingestion session is busy (another dispatcher/run holds the lock); try again later", 1)
			}
			defer releaseLock(lock)
			j := Job{SessionID: cmd.String("session"), TranscriptPath: cmd.String("transcript")}
			if ok := processJob(j, cmd.Duration("timeout"), true); !ok {
				return cli.Exit("integration failed (see dispatch.log)", 1)
			}
			return nil
		},
	}
}

func statusCmd() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "show queue, tmux session, and recent ledgers",
		Action: func(_ context.Context, _ *cli.Command) error {
			jobs, _ := PendingJobs()
			fmt.Printf("home:    %s\n", Home())
			fmt.Printf("session: %s (%s)\n", SessionName, aliveLabel(SessionAlive()))
			fmt.Printf("pending: %d job(s)\n", len(jobs))
			for _, j := range jobs {
				fmt.Printf("  - %s  %s\n", j.SessionID, j.EnqueuedAt)
			}
			entries, _ := os.ReadDir(stateDir())
			fmt.Printf("ledgers: %d session(s)\n", len(entries))
			for _, e := range entries {
				l, err := LoadLedger(trimJSON(e.Name()))
				if err != nil {
					continue
				}
				fmt.Printf("  - %s  cursor=%d runs=%d touched=%d  (updated %s)\n",
					l.SessionID, l.Cursor, l.Runs, len(l.TouchedNotes), l.UpdatedAt)
			}
			return nil
		},
	}
}

func sessionCmd() *cli.Command {
	return &cli.Command{
		Name:  "session",
		Usage: "manage the dedicated tmux ingestion session",
		Commands: []*cli.Command{
			{
				Name:  "ensure",
				Usage: "create the session if it is not already running",
				Action: func(_ context.Context, _ *cli.Command) error {
					if err := EnsureSession(); err != nil {
						return err
					}
					fmt.Printf("%s is up\n", SessionName)
					return nil
				},
			},
			{
				Name:  "kill",
				Usage: "terminate the session",
				Action: func(_ context.Context, _ *cli.Command) error {
					if err := KillSession(); err != nil {
						return err
					}
					fmt.Printf("%s killed\n", SessionName)
					return nil
				},
			},
		},
	}
}

func aliveLabel(up bool) string {
	if up {
		return "running"
	}
	return "down"
}

func trimJSON(name string) string {
	if len(name) > 5 && name[len(name)-5:] == ".json" {
		return name[:len(name)-5]
	}
	return name
}
