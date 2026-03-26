// Command api starts the HTTP API server only (no background workers or scheduler).
// Use this when workers run in a separate process or container.
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

	if err := app.RunAPI(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}
