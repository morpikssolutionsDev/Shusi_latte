// Package jobs runs the in-process scheduler (TRD §19). The API is the only process that
// owns the database, so no distributed lock is needed.
package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"sushilatte/api/internal/platform"
)

type Job struct {
	Name string
	Run  func(ctx context.Context) error
	next func(now time.Time) time.Time
}

// Every runs a job at a fixed interval.
func Every(name string, interval time.Duration, run func(context.Context) error) Job {
	return Job{Name: name, Run: run, next: func(now time.Time) time.Time { return now.Add(interval) }}
}

// DailyAt runs a job once a day at hh:mm WIB.
func DailyAt(name string, hour, minute int, run func(context.Context) error) Job {
	return Job{Name: name, Run: run, next: func(now time.Time) time.Time {
		local := now.In(platform.Jakarta)
		t := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, platform.Jakarta)
		if !t.After(local) {
			t = t.AddDate(0, 0, 1)
		}
		return t
	}}
}

type Scheduler struct {
	log  *slog.Logger
	jobs []Job
	wg   sync.WaitGroup
}

func NewScheduler(log *slog.Logger, jobs ...Job) *Scheduler {
	return &Scheduler{log: log, jobs: jobs}
}

// Start runs every job on its schedule until ctx is cancelled. Jobs are idempotent, so a
// run missed during downtime is simply caught up by the next one.
func (s *Scheduler) Start(ctx context.Context) {
	for _, j := range s.jobs {
		s.wg.Add(1)
		go func(j Job) {
			defer s.wg.Done()
			for {
				wait := time.Until(j.next(time.Now()))
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait):
				}
				started := time.Now()
				if err := j.Run(ctx); err != nil && ctx.Err() == nil {
					s.log.Error("job failed", "job", j.Name, "err", err)
				} else {
					s.log.Debug("job ran", "job", j.Name, "took", time.Since(started))
				}
			}
		}(j)
	}
}

// Wait blocks until all job goroutines have stopped after ctx cancellation.
func (s *Scheduler) Wait() { s.wg.Wait() }
