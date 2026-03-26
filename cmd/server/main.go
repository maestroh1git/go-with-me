// Command server starts the full application: HTTP API + job worker + scheduler.
// This is the recommended entrypoint for single-process deployments (e.g., Fly.io, Railway, Render).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-with-me/app/internal/bootstrap"
)

func main() {
	ctx := context.Background()

	app, err := bootstrap.New(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	if err := app.RunAll(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}
