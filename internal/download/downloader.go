package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// Errors returned by the downloader.
var (
	// ErrDownload is the generic download failure.
	ErrDownload = errors.New("download failed")
	// ErrTooLarge signals that the file exceeds the configured limit.
	ErrTooLarge = errors.New("file is too large")
)

// TooLargeError carries the size information of a rejected download.
type TooLargeError struct {
	Size    int64
	MaxSize int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("%s: %d bytes exceed the limit of %d bytes", ErrTooLarge, e.Size, e.MaxSize)
}

// Unwrap allows errors.Is checks against ErrTooLarge.
func (e *TooLargeError) Unwrap() error { return ErrTooLarge }

// Metrics describes a finished download.
type Metrics struct {
	URL           string
	Path          string
	Size          int64
	Elapsed       time.Duration
	UsedMultipart bool
	Resumed       bool
}

// Progress describes the state of a running download.
type Progress struct {
	DownloadedBytes int64
	TotalBytes      int64
	Elapsed         time.Duration
	SpeedBPS        float64
	ETASeconds      float64
	Done            bool
}

// ProgressFunc receives download progress updates.
type ProgressFunc func(Progress)

// Options are the per-download options.
type Options struct {
	Headers     map[string]string
	SkipIfExist bool
	MaxSize     int64
	Source      string
	OnProgress  ProgressFunc
}

// Config tunes the downloader behaviour.
type Config struct {
	ChunkSize          int64
	MultipartThreshold int64
	MaxWorkers         int
	HeadTimeout        time.Duration
	StreamTimeout      time.Duration
	ClientTimeout      time.Duration
	MaxRetries         int
	RetryBackoff       time.Duration
	AllowResume        bool
	TempSuffix         string
}

// DefaultConfig returns the shared download configuration.
func DefaultConfig() Config {
	return Config{
		ChunkSize:          1 << 20,
		MultipartThreshold: 12 << 20,
		MaxWorkers:         6,
		HeadTimeout:        3 * time.Second,
		StreamTimeout:      60 * time.Second,
		ClientTimeout:      3 * time.Minute,
		MaxRetries:         3,
		RetryBackoff:       750 * time.Millisecond,
		AllowResume:        true,
		TempSuffix:         ".part",
	}
}

// Downloader downloads files with resume, multipart and retry support.
type Downloader struct {
	outputDir string
	config    Config
	client    *http.Client
	log       Logger

	mu       sync.Mutex
	inFlight map[string]*downloadHandle
}

type downloadHandle struct {
	done  chan struct{}
	bytes int64
	path  string
	err   error
}

// NewDownloader builds a downloader writing into outputDir.
func NewDownloader(outputDir string, config Config, log Logger) *Downloader {
	if config.ChunkSize <= 0 {
		config = DefaultConfig()
	}
	return &Downloader{
		outputDir: outputDir,
		config:    config,
		client:    httpx.NewClient(config.ClientTimeout, config.MaxWorkers*4),
		log:       log,
		inFlight:  map[string]*downloadHandle{},
	}
}

// OutputDir returns the directory the downloader writes into.
func (d *Downloader) OutputDir() string { return d.outputDir }

// Download fetches a URL into the output directory.
func (d *Downloader) Download(ctx context.Context, rawURL, filename string, opts Options) (*Metrics, error) {
	target, err := d.resolvePath(filename)
	if err != nil {
		return nil, err
	}

	if opts.SkipIfExist {
		if info, statErr := os.Stat(target); statErr == nil && info.Size() > 0 {
			return &Metrics{URL: rawURL, Path: target, Size: info.Size()}, nil
		}
	}

	key := target + "::" + rawURL
	d.mu.Lock()
	if handle, ok := d.inFlight[key]; ok {
		d.mu.Unlock()
		<-handle.done
		if handle.err != nil {
			return nil, handle.err
		}
		return &Metrics{URL: rawURL, Path: handle.path, Size: handle.bytes}, nil
	}
	handle := &downloadHandle{done: make(chan struct{})}
	d.inFlight[key] = handle
	d.mu.Unlock()

	metrics, downloadErr := d.downloadExclusive(ctx, rawURL, target, opts)

	d.mu.Lock()
	handle.err = downloadErr
	handle.path = target
	if metrics != nil {
		handle.bytes = metrics.Size
	}
	delete(d.inFlight, key)
	d.mu.Unlock()
	close(handle.done)

	return metrics, downloadErr
}

func (d *Downloader) downloadExclusive(ctx context.Context, rawURL, target string, opts Options) (*Metrics, error) {
	var lastErr error
	attempts := d.config.MaxRetries
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		metrics, err := d.downloadOnce(ctx, rawURL, target, opts)
		if err == nil {
			return metrics, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
		if attempt == attempts {
			break
		}
		backoff := time.Duration(float64(d.config.RetryBackoff) * float64(attempt))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return nil, fmt.Errorf("%w: %v", ErrDownload, lastErr)
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var tooLarge *TooLargeError
	if errors.As(err, &tooLarge) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

func (d *Downloader) downloadOnce(ctx context.Context, rawURL, target string, opts Options) (*Metrics, error) {
	started := time.Now()
	headers := httpx.MergeHeaders(opts.Headers)

	size, ranges, probeErr := d.probe(ctx, rawURL, headers)
	switch {
	case probeErr == nil:
	case errors.Is(probeErr, errNoLength):
	case errors.Is(probeErr, context.DeadlineExceeded):
		// The probe only collects the size of the file, a slow host must not
		// stop the download.
		size, ranges = 0, false
	default:
		return nil, probeErr
	}
	if opts.MaxSize > 0 && size > opts.MaxSize {
		return nil, &TooLargeError{Size: size, MaxSize: opts.MaxSize}
	}

	tempPath := target + d.config.TempSuffix
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, fmt.Errorf("create download directory: %w", err)
	}

	var (
		written int64
		resumed bool
	)

	if d.config.AllowResume {
		if info, err := os.Stat(tempPath); err == nil && size > 0 && info.Size() < size {
			written = info.Size()
			resumed = true
		} else if err == nil {
			_ = os.Remove(tempPath)
		}
	}

	multipart := ranges && size >= d.config.MultipartThreshold && !resumed
	chunkSize := int64(0)
	if multipart {
		parts := d.config.MaxWorkers
		if parts < 2 {
			parts = 2
		}
		chunkSize = size / int64(parts)
		if chunkSize < d.config.ChunkSize {
			multipart = false
			chunkSize = 0
		}
	}

	if multipart {
		total, err := d.downloadMultipart(ctx, rawURL, tempPath, size, chunkSize, headers, opts)
		if err != nil {
			return nil, err
		}
		written = total
	} else {
		total, err := d.downloadStream(ctx, rawURL, tempPath, written, headers, opts, size)
		if err != nil {
			return nil, err
		}
		written = total
	}

	if err := os.Rename(tempPath, target); err != nil {
		return nil, fmt.Errorf("finalise download: %w", err)
	}

	if opts.MaxSize > 0 && written > opts.MaxSize {
		_ = os.Remove(target)
		return nil, &TooLargeError{Size: written, MaxSize: opts.MaxSize}
	}

	return &Metrics{
		URL:           rawURL,
		Path:          target,
		Size:          written,
		Elapsed:       time.Since(started),
		UsedMultipart: multipart,
		Resumed:       resumed,
	}, nil
}

var errNoLength = errors.New("content length unknown")

func (d *Downloader) probe(ctx context.Context, rawURL string, headers map[string]string) (int64, bool, error) {
	probeCtx, cancel := context.WithTimeout(ctx, d.config.HeadTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(probeCtx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, false, fmt.Errorf("build head request: %w", err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := d.client.Do(request)
	if err == nil {
		defer response.Body.Close()
		if response.StatusCode >= 400 {
			return 0, false, fmt.Errorf("%w: head request returned %d", ErrDownload, response.StatusCode)
		}
		length := response.ContentLength
		if length < 0 {
			length = parseContentLength(response.Header.Get("Content-Length"))
		}
		acceptsRanges := strings.Contains(strings.ToLower(response.Header.Get("Accept-Ranges")), "bytes")
		if length <= 0 {
			return 0, false, errNoLength
		}
		return length, acceptsRanges, nil
	}

	// Some hosts reject HEAD requests, fall back to a ranged GET probe.
	probeCtx2, cancel2 := context.WithTimeout(ctx, d.config.HeadTimeout)
	defer cancel2()
	getRequest, err := http.NewRequestWithContext(probeCtx2, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, false, fmt.Errorf("build probe request: %w", err)
	}
	getRequest.Header.Set("Range", "bytes=0-0")
	for key, value := range headers {
		getRequest.Header.Set(key, value)
	}
	getResponse, err := d.client.Do(getRequest)
	if err != nil {
		return 0, false, fmt.Errorf("probe %s: %w", rawURL, err)
	}
	defer getResponse.Body.Close()
	if getResponse.StatusCode >= 400 {
		return 0, false, fmt.Errorf("%w: probe returned %d", ErrDownload, getResponse.StatusCode)
	}
	if getResponse.StatusCode == http.StatusPartialContent {
		if total := parseContentRangeTotal(getResponse.Header.Get("Content-Range")); total > 0 {
			return total, true, nil
		}
	}
	length := parseContentLength(getResponse.Header.Get("Content-Length"))
	if length <= 0 {
		return 0, false, errNoLength
	}
	return length, false, nil
}

func parseContentLength(value string) int64 {
	var parsed int64
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return 0
	}
	return parsed
}

func parseContentRangeTotal(value string) int64 {
	index := strings.LastIndex(value, "/")
	if index < 0 {
		return 0
	}
	return parseContentLength(value[index+1:])
}

func (d *Downloader) downloadStream(ctx context.Context, rawURL, tempPath string, offset int64, headers map[string]string, opts Options, total int64) (int64, error) {
	activity := &atomic.Int64{}
	activity.Store(time.Now().UnixNano())

	requestCtx, cancel := d.watchStall(ctx, activity)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, fmt.Errorf("build download request: %w", err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if offset > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	response, err := d.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("download %s: %w", rawURL, err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return 0, fmt.Errorf("%w: server returned %d", ErrDownload, response.StatusCode)
	}
	if offset > 0 && response.StatusCode != http.StatusPartialContent {
		offset = 0
	}

	flags := os.O_CREATE | os.O_WRONLY
	if offset > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(tempPath, flags, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", tempPath, err)
	}
	defer file.Close()

	writer := &progressWriter{
		file:     file,
		offset:   offset,
		total:    total,
		started:  time.Now(),
		callback: opts.OnProgress,
		activity: activity,
	}
	if _, err := io.Copy(writer, response.Body); err != nil {
		return writer.written, fmt.Errorf("copy body: %w", err)
	}
	if err := writer.finish(); err != nil {
		return writer.written, err
	}
	return writer.written, nil
}

func (d *Downloader) downloadMultipart(ctx context.Context, rawURL, tempPath string, size, chunkSize int64, headers map[string]string, opts Options) (int64, error) {
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", tempPath, err)
	}
	defer file.Close()

	if err := file.Truncate(size); err != nil {
		return 0, fmt.Errorf("allocate %s: %w", tempPath, err)
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		written  int64
	)

	partCount := (size + chunkSize - 1) / chunkSize
	for index := int64(0); index < partCount; index++ {
		start := index * chunkSize
		end := start + chunkSize - 1
		if end > size-1 {
			end = size - 1
		}

		wg.Add(1)
		go func(partIndex, from, to int64) {
			defer wg.Done()
			count, partErr := d.downloadPart(ctx, rawURL, tempPath, from, to, headers)
			mu.Lock()
			written += count
			if partErr != nil && firstErr == nil {
				firstErr = partErr
			}
			mu.Unlock()
		}(index, start, end)
	}
	wg.Wait()

	if firstErr != nil {
		return written, firstErr
	}
	if opts.OnProgress != nil {
		opts.OnProgress(Progress{DownloadedBytes: written, TotalBytes: size, Done: true})
	}
	return written, nil
}

func (d *Downloader) downloadPart(ctx context.Context, rawURL, tempPath string, from, to int64, headers map[string]string) (int64, error) {
	activity := &atomic.Int64{}
	activity.Store(time.Now().UnixNano())

	requestCtx, cancel := d.watchStall(ctx, activity)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, fmt.Errorf("build range request: %w", err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", from, to))

	response, err := d.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("download range %d-%d: %w", from, to, err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return 0, fmt.Errorf("%w: range request returned %d", ErrDownload, response.StatusCode)
	}

	file, err := os.OpenFile(tempPath, os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", tempPath, err)
	}
	defer file.Close()

	var written int64
	buffer := make([]byte, 256*1024)
	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			activity.Store(time.Now().UnixNano())
			if _, writeErr := file.WriteAt(buffer[:n], from+written); writeErr != nil {
				return written, fmt.Errorf("write range: %w", writeErr)
			}
			written += int64(n)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return written, fmt.Errorf("read range: %w", readErr)
		}
	}
	return written, nil
}

// watchStall returns a context that is cancelled when the download stops
// sending data.
//
// A host that generates a file on demand answers late, the watchdog only
// counts the silence between two packets.
func (d *Downloader) watchStall(ctx context.Context, activity *atomic.Int64) (context.Context, context.CancelFunc) {
	requestCtx, cancel := context.WithCancel(ctx)
	if d.config.StreamTimeout <= 0 {
		return requestCtx, cancel
	}

	go func() {
		interval := d.config.StreamTimeout / 2
		if interval < time.Second {
			interval = time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-requestCtx.Done():
				return
			case <-ticker.C:
				last := time.Unix(0, activity.Load())
				if time.Since(last) >= d.config.StreamTimeout {
					cancel()
					return
				}
			}
		}
	}()

	return requestCtx, cancel
}

type progressWriter struct {
	file       *os.File
	offset     int64
	written    int64
	total      int64
	started    time.Time
	callback   ProgressFunc
	lastNotify time.Time
	activity   *atomic.Int64
}

func (w *progressWriter) Write(data []byte) (int, error) {
	n, err := w.file.Write(data)
	w.written += int64(n)
	if n > 0 && w.activity != nil {
		w.activity.Store(time.Now().UnixNano())
	}
	w.notify(false)
	return n, err
}

func (w *progressWriter) notify(done bool) {
	if w.callback == nil {
		return
	}
	now := time.Now()
	if !done && now.Sub(w.lastNotify) < 900*time.Millisecond {
		return
	}
	w.lastNotify = now

	elapsed := now.Sub(w.started)
	speed := 0.0
	if elapsed > 0 {
		speed = float64(w.written) / elapsed.Seconds()
	}
	eta := 0.0
	if speed > 0 && w.total > 0 {
		remaining := float64(w.total - w.offset - w.written)
		if remaining > 0 {
			eta = remaining / speed
		}
	}
	w.callback(Progress{
		DownloadedBytes: w.offset + w.written,
		TotalBytes:      w.total,
		Elapsed:         elapsed,
		SpeedBPS:        speed,
		ETASeconds:      eta,
		Done:            done,
	})
}

func (w *progressWriter) finish() error {
	w.notify(true)
	return nil
}

// resolvePath builds an absolute path inside the output directory and rejects
// attempts to escape it.
func (d *Downloader) resolvePath(filename string) (string, error) {
	cleaned := filepath.Clean(filename)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("%w: absolute paths are not allowed", ErrDownload)
	}
	target := filepath.Join(d.outputDir, cleaned)
	root, err := filepath.Abs(d.outputDir)
	if err != nil {
		return "", fmt.Errorf("resolve output dir: %w", err)
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve target: %w", err)
	}
	if absolute != root && !strings.HasPrefix(absolute, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: path escapes the output directory", ErrDownload)
	}
	return absolute, nil
}

// Cleanup removes a downloaded file.
func Cleanup(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

// Priority maps a size hint onto a queue priority.
func Priority(sizeHint int64) int {
	switch {
	case sizeHint > 0 && sizeHint <= 25*1024*1024:
		return 10
	case sizeHint > 0 && sizeHint <= 120*1024*1024:
		return 25
	case sizeHint > 0:
		return 50
	default:
		return 40
	}
}
