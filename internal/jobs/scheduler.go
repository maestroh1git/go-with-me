package jobs

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// ScheduledJob defines a periodic job with a fixed interval.
type ScheduledJob struct {
	Name     string
	Interval time.Duration
	Handler  func(ctx context.Context) error
}

// Scheduler runs periodic jobs on independent tickers.
type Scheduler struct {
	jobs   []ScheduledJob
	logger *zap.Logger
}

// NewScheduler creates a Scheduler with the given logger.
func NewScheduler(logger *zap.Logger) *Scheduler {
	return &Scheduler{logger: logger}
}

// Every registers a periodic job that runs at the given interval.
func (s *Scheduler) Every(interval time.Duration, name string, fn func(ctx context.Context) error) {
	s.jobs = append(s.jobs, ScheduledJob{
		Name:     name,
		Interval: interval,
		Handler:  fn,
	})
}

// Run starts all scheduled jobs in separate goroutines and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.logger.Info("job scheduler starting", zap.Int("job_count", len(s.jobs)))

	for _, job := range s.jobs {
		go s.runJob(ctx, job)
	}

	<-ctx.Done()
	s.logger.Info("job scheduler stopped")
}

func (s *Scheduler) runJob(ctx context.Context, job ScheduledJob) {
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	s.logger.Info("scheduled job registered",
		zap.String("name", job.Name),
		zap.Duration("interval", job.Interval),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.logger.Debug("running scheduled job", zap.String("name", job.Name))
			if err := job.Handler(ctx); err != nil {
				s.logger.Error("scheduled job failed",
					zap.String("name", job.Name),
					zap.Error(err),
				)
			}
		}
	}
}
