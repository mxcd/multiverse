package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mxcd/multiverse/internal/ingest"
)

// version is overridable at build time: -ldflags "-X main.version=..."
var version = "0.1.0-dev"

func main() {
	app := ingest.NewApp(version)
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
