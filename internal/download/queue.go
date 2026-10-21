// Package download implements the shared download queue and the resilient
// downloader used by every platform service.
package download

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Iotatg/iota-downloader/internal/runtime"
)

// Errors returned by the queue.
var (
	// ErrQueueBusy signals that the queue is saturated.
	ErrQueueBusy = errors.New("download queue is busy")
	// ErrRateLimited signals that the caller exceeded its rate limit.
	ErrRateLimited = errors.New("download rate limit exceeded")
)

// Ticket describes the position of a job inside the queue.
type Ticket struct {
	Position      int
	ActiveWorkers int
	QueuedJobs    int
}

// Job is a unit of work handled by the queue.
type Job struct {
	// Key identifies the job. Duplicate keys share the same result.
	Key string
	// UserID scopes the per-user rate limits.
	UserID int64
	// Priority orders the queue, lower values run first.
	Priority int
	// Execute performs the work.
	Execute func(ctx context.Context) (any, error)
}

// QueueOptions configures the queue.
type QueueOptions struct {
	MinWorkers            int
	MaxWorkers            int
	MaxSize               int
	PerUserRateLimit      int
	PerUserWindow         time.Duration
	PerUserMaxPending     int
	PerUserPendingTimeout time.Duration
	ScaleCooldown         time.Duration
	IdleScaleDown         time.Duration
	MaxWorkersCap         int
}

// Queue is an adaptive worker pool with backpressure and per-user limits.
type Queue struct {
	opts QueueOptions
	log  Logger

	mu        sync.Mutex
	cond      *sync.Cond
	jobs      []*queuedJob
	workers   int
	active    int
	closed    bool
	lastScale time.Time
	idleSince time.Time

	rate    *runtime.RateLimiter
	pending map[int64]int

	enqueued uint64
	finished uint64
}

type queuedJob struct {
	job      Job
	enqueued time.Time
}

// Logger is the minimal logging surface required by the queue.
type Logger interface {
	Info(format string, args ...any)
	Warning(format string, args ...any)
	Error(format string, args ...any)
	Perf(name string, durationMS float64, fields map[string]any)
}

// NewQueue creates and starts the worker pool.
func NewQueue(opts QueueOptions, log Logger) *Queue {
	if opts.MinWorkers <= 0 {
		opts.MinWorkers = 1
	}
	if opts.MaxWorkers < opts.MinWorkers {
		opts.MaxWorkers = opts.MinWorkers
	}
	if opts.MaxSize <= 0 {
		opts.MaxSize = 250
	}
	if opts.MaxWorkersCap > 0 && opts.MaxWorkers > opts.MaxWorkersCap {
		opts.MaxWorkers = opts.MaxWorkersCap
	}

	queue := &Queue{
		opts:      opts,
		log:       log,
		pending:   map[int64]int{},
		rate:      runtime.NewRateLimiter(opts.PerUserRateLimit, opts.PerUserWindow, 50000),
		lastScale: time.Now(),
	}
	queue.cond = sync.NewCond(&queue.mu)
	for i := 0; i < opts.MinWorkers; i++ {
		queue.spawnWorker()
	}
	return queue
}

// Submit enqueues a job and returns its ticket.
func (q *Queue) Submit(ctx context.Context, job Job) (Ticket, error) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return Ticket{}, errors.New("download queue is closed")
	}
	if len(q.jobs) >= q.opts.MaxSize {
		position := len(q.jobs) + 1
		q.mu.Unlock()
		return Ticket{Position: position}, fmt.Errorf("%w: %d jobs queued", ErrQueueBusy, position)
	}

	pending := q.pending[job.UserID]
	if q.opts.PerUserMaxPending > 0 && pending >= q.opts.PerUserMaxPending {
		position := len(q.jobs) + 1
		q.mu.Unlock()
		return Ticket{Position: position}, fmt.Errorf("%w: %d pending jobs", ErrQueueBusy, position)
	}
	q.mu.Unlock()

	if !q.rate.Allow(userKey(job.UserID)) {
		return Ticket{}, fmt.Errorf("%w: retry in %s", ErrRateLimited, q.opts.PerUserWindow)
	}

	q.mu.Lock()
	q.pending[job.UserID]++
	q.jobs = append(q.jobs, &queuedJob{job: job, enqueued: time.Now()})
	q.sortLocked()
	q.enqueued++
	ticket := Ticket{Position: len(q.jobs), ActiveWorkers: q.workers, QueuedJobs: len(q.jobs)}
	q.mu.Unlock()

	q.cond.Broadcast()
	q.maybeScaleUp()
	return ticket, nil
}

func userKey(userID int64) string { return fmt.Sprintf("user:%d", userID) }

// sortLocked keeps the queue ordered by priority and then by arrival time.
func (q *Queue) sortLocked() {
	jobs := q.jobs
	for i := 1; i < len(jobs); i++ {
		current := jobs[i]
		j := i - 1
		for j >= 0 && lessJob(current, jobs[j]) {
			jobs[j+1] = jobs[j]
			j--
		}
		jobs[j+1] = current
	}
}

func lessJob(a, b *queuedJob) bool {
	if a.job.Priority != b.job.Priority {
		return a.job.Priority < b.job.Priority
	}
	return a.enqueued.Before(b.enqueued)
}

func (q *Queue) spawnWorker() {
	q.workers++
	go q.worker()
}

func (q *Queue) worker() {
	for {
		q.mu.Lock()
		for len(q.jobs) == 0 && !q.closed {
			q.mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			q.mu.Lock()
		}
		if q.closed && len(q.jobs) == 0 {
			q.workers--
			q.mu.Unlock()
			return
		}
		entry := q.jobs[0]
		q.jobs = q.jobs[1:]
		q.active++
		q.mu.Unlock()

		q.run(entry)

		q.mu.Lock()
		q.active--
		q.finished++
		if q.pending[entry.job.UserID] > 0 {
			q.pending[entry.job.UserID]--
		}
		if len(q.jobs) == 0 {
			q.idleSince = time.Now()
		}
		q.mu.Unlock()

		q.maybeScale()
	}
}

func (q *Queue) run(entry *queuedJob) {
	wait := time.Since(entry.enqueued)
	ctx := context.Background()
	result, err := entry.job.Execute(ctx)
	if err != nil {
		q.log.Error("download job failed: key=%s error=%s", entry.job.Key, err)
	} else {
		q.log.Perf("download_job", float64(wait.Milliseconds()), map[string]any{
			"key":     entry.job.Key,
			"user_id": entry.job.UserID,
		})
	}
	_ = result
}

func (q *Queue) maybeScaleUp() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) == 0 || q.workers >= q.opts.MaxWorkers {
		return
	}
	if time.Since(q.lastScale) < q.opts.ScaleCooldown {
		return
	}
	q.lastScale = time.Now()
	q.spawnWorker()
	q.log.Info("download queue scaled up: workers=%d queued=%d", q.workers, len(q.jobs))
}

func (q *Queue) maybeScale() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) > 0 {
		return
	}
	if q.workers <= q.opts.MinWorkers {
		return
	}
	if q.idleSince.IsZero() {
		q.idleSince = time.Now()
		return
	}
	if time.Since(q.idleSince) < q.opts.IdleScaleDown {
		return
	}
	q.workers--
	q.idleSince = time.Now()
	q.log.Info("download queue scaled down: workers=%d", q.workers)
}

// Stats returns the live queue metrics.
func (q *Queue) Stats() (queued, active, workers int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs), q.active, q.workers
}

// Close stops the queue and waits for the running jobs.
func (q *Queue) Close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.cond.Broadcast()
}

// Wait drains the queue, used by tests and by the graceful shutdown.
func (q *Queue) Wait(ctx context.Context) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		queued, active, _ := q.Stats()
		if queued == 0 && active == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// PendingForUser returns the number of queued jobs of a user.
func (q *Queue) PendingForUser(userID int64) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pending[userID]
}
