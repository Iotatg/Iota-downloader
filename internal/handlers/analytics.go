package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/Iotatg/iota-downloader/internal/storage"
)

// AnalyticsRecorder batches analytics events and writes them to the database.
type AnalyticsRecorder struct {
	db    *storage.DB
	log   AnalyticsLogger
	queue []storage.AnalyticsEvent
	mu    sync.Mutex
	stop  chan struct{}
	once  sync.Once
}

// AnalyticsLogger is the logging surface of the recorder.
type AnalyticsLogger interface {
	Warning(format string, args ...any)
	Error(format string, args ...any)
}

// NewAnalyticsRecorder builds a recorder that flushes every interval.
func NewAnalyticsRecorder(db *storage.DB, log AnalyticsLogger, interval time.Duration) *AnalyticsRecorder {
	recorder := &AnalyticsRecorder{db: db, log: log, stop: make(chan struct{})}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go recorder.loop(interval)
	return recorder
}

// Record stores an event for the next flush.
func (r *AnalyticsRecorder) Record(action string, userID int64, chatType string) {
	if r == nil || r.db == nil {
		return
	}
	event := storage.AnalyticsEvent{
		ActionName: action,
		UserID:     userID,
		ChatType:   chatType,
		CreatedAt:  time.Now().UTC(),
	}

	r.mu.Lock()
	r.queue = append(r.queue, event)
	pending := len(r.queue)
	r.mu.Unlock()

	if pending >= 100 {
		r.Flush(context.Background())
	}
}

// loop flushes the buffered events periodically.
func (r *AnalyticsRecorder) loop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			r.Flush(context.Background())
			return
		case <-ticker.C:
			r.Flush(context.Background())
		}
	}
}

// Flush writes the buffered events to the database.
func (r *AnalyticsRecorder) Flush(ctx context.Context) {
	if r == nil || r.db == nil {
		return
	}
	r.mu.Lock()
	if len(r.queue) == 0 {
		r.mu.Unlock()
		return
	}
	events := r.queue
	r.queue = nil
	r.mu.Unlock()

	if err := r.db.InsertAnalyticsEvents(ctx, events); err != nil && r.log != nil {
		r.log.Error("write analytics events: %v", err)
	}
}

// Close stops the recorder and writes the remaining events.
func (r *AnalyticsRecorder) Close() {
	if r == nil {
		return
	}
	r.once.Do(func() { close(r.stop) })
}
