package cli

import (
	"context"
	"os"

	"github.com/mxcd/multiverse/internal/tui"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

func tuiCmd() *cli.Command {
	return &cli.Command{
		Name:    "tui",
		Aliases: []string{"dashboard", "ui"},
		Usage:   "interactive control panel: dashboard, registry, and scope config",
		Action: func(_ context.Context, _ *cli.Command) error {
			return tui.Run()
		},
	}
}

// rootAction launches the TUI when multi is run with no subcommand on an
// interactive terminal; otherwise it prints help.
func rootAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Present() {
		return cli.ShowAppHelp(cmd)
	}
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		return tui.Run()
	}
	return cli.ShowAppHelp(cmd)
}
