package jobs

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"
)

// Job is a named background task handler.
type Job struct {
	Name    string
	Handler func(ctx context.Context) error
}

// Worker runs a collection of named background jobs concurrently.
// All jobs start together and run until ctx is cancelled.
type Worker struct {
	jobs   []Job
	logger *zap.Logger
}

// New creates a Worker with the given logger.
func New(logger *zap.Logger) *Worker {
	return &Worker{logger: logger}
}

// Register adds a job handler to the worker.
func (w *Worker) Register(name string, fn func(ctx context.Context) error) {
	w.jobs = append(w.jobs, Job{Name: name, Handler: fn})
}

// Run starts all registered jobs concurrently and blocks until ctx is cancelled
// or all jobs have exited. Returns the first non-cancellation error encountered.
func (w *Worker) Run(ctx context.Context) error {
	if len(w.jobs) == 0 {
		w.logger.Info("job worker: no jobs registered, waiting for ctx")
		<-ctx.Done()
		return ctx.Err()
	}

	w.logger.Info("job worker starting", zap.Int("job_count", len(w.jobs)))

	errCh := make(chan error, len(w.jobs))
	var wg sync.WaitGroup

	for _, job := range w.jobs {
		j := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.logger.Info("job starting", zap.String("name", j.Name))
			if err := j.Handler(ctx); err != nil && !errors.Is(err, context.Canceled) {
				w.logger.Error("job exited with error",
					zap.String("name", j.Name),
					zap.Error(err),
				)
				errCh <- err
			} else {
				w.logger.Info("job stopped", zap.String("name", j.Name))
			}
		}()
	}

	// Wait for all goroutines to finish after ctx cancellation.
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Return the first error, or nil on clean shutdown.
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
