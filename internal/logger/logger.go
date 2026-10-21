// Package logger provides the structured logging used by the Iota Coder bot.
//
// Four sinks are maintained:
//
//	logs/bot_log.log     human readable application log
//	logs/error_log.log   errors only
//	logs/events_log.jsonl structured business events
//	logs/perf_log.jsonl  performance samples
package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type contextKey string

const (
	serviceKey   contextKey = "service"
	flowKey      contextKey = "flow"
	requestIDKey contextKey = "request_id"
)

// Fields is a set of structured key/value pairs attached to a log record.
type Fields = map[string]any

// Logger writes human readable and structured records to disk.
type Logger struct {
	mu sync.Mutex

	infoFile  *os.File
	errorFile *os.File
	eventFile *os.File
	perfFile  *os.File

	debug bool
}

// Options configures the logger.
type Options struct {
	Dir       string
	Debug     bool
	Timestamp func() time.Time
}

// New opens the log files and returns a ready to use logger.
func New(opts Options) (*Logger, error) {
	if opts.Dir == "" {
		opts.Dir = "logs"
	}
	if opts.Timestamp == nil {
		opts.Timestamp = time.Now
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	info, err := openLog(filepath.Join(opts.Dir, "bot_log.log"))
	if err != nil {
		return nil, err
	}
	errLog, err := openLog(filepath.Join(opts.Dir, "error_log.log"))
	if err != nil {
		return nil, err
	}
	events, err := openLog(filepath.Join(opts.Dir, "events_log.jsonl"))
	if err != nil {
		return nil, err
	}
	perf, err := openLog(filepath.Join(opts.Dir, "perf_log.jsonl"))
	if err != nil {
		return nil, err
	}

	return &Logger{
		infoFile:  info,
		errorFile: errLog,
		eventFile: events,
		perfFile:  perf,
		debug:     opts.Debug,
	}, nil
}

func openLog(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", path, err)
	}
	return file, nil
}

// Close flushes and closes every sink.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var firstErr error
	for _, file := range []*os.File{l.infoFile, l.errorFile, l.eventFile, l.perfFile} {
		if file == nil {
			continue
		}
		if err := file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (l *Logger) write(level, format string, args ...any) {
	if l == nil || l.infoFile == nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), level, message)

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.infoFile, line)
	if level == "ERROR" {
		_, _ = io.WriteString(l.errorFile, line)
	}
}

// Debug logs at debug level when debug mode is enabled.
func (l *Logger) Debug(format string, args ...any) {
	if l == nil || !l.debug {
		return
	}
	l.write("DEBUG", format, args...)
}

// Info logs an informational message.
func (l *Logger) Info(format string, args ...any) { l.write("INFO", format, args...) }

// Warning logs a warning message.
func (l *Logger) Warning(format string, args ...any) { l.write("WARNING", format, args...) }

// Warn is an alias of Warning.
func (l *Logger) Warn(format string, args ...any) { l.write("WARNING", format, args...) }

// Error logs an error message.
func (l *Logger) Error(format string, args ...any) { l.write("ERROR", format, args...) }

// Event writes a structured business event.
func (l *Logger) Event(name string, fields Fields) {
	if l == nil || l.eventFile == nil {
		return
	}
	record := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"event": name,
	}
	for key, value := range fields {
		record[key] = value
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.eventFile, string(payload)+"\n")
}

// Perf writes a structured performance sample.
func (l *Logger) Perf(name string, durationMS float64, fields Fields) {
	if l == nil || l.perfFile == nil {
		return
	}
	record := map[string]any{
		"ts":          time.Now().UTC().Format(time.RFC3339Nano),
		"perf":        name,
		"duration_ms": durationMS,
	}
	for key, value := range fields {
		record[key] = value
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.perfFile, string(payload)+"\n")
}

// WithService returns a context tagged with a service name.
func WithService(ctx context.Context, service string) context.Context {
	return context.WithValue(ctx, serviceKey, service)
}

// WithFlow returns a context tagged with a flow identifier.
func WithFlow(ctx context.Context, flow string) context.Context {
	return context.WithValue(ctx, flowKey, flow)
}

// WithRequestID returns a context tagged with a request identifier.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// ContextFields extracts the logging context fields of a context.
func ContextFields(ctx context.Context) Fields {
	fields := Fields{}
	if ctx == nil {
		return fields
	}
	if value, ok := ctx.Value(serviceKey).(string); ok && value != "" {
		fields["service"] = value
	}
	if value, ok := ctx.Value(flowKey).(string); ok && value != "" {
		fields["flow"] = value
	}
	if value, ok := ctx.Value(requestIDKey).(string); ok && value != "" {
		fields["request_id"] = value
	}
	return fields
}
