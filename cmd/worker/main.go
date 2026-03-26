// Command worker starts the background job worker and scheduler only (no HTTP server).
// Use this when the API runs in a separate process or container.
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

	if err := app.RunWorker(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}
