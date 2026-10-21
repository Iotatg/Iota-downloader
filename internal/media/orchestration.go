package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Iotatg/iota-downloader/internal/download"
	"github.com/Iotatg/iota-downloader/internal/logger"
	"github.com/Iotatg/iota-downloader/internal/runtime"
	"github.com/Iotatg/iota-downloader/internal/storage"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// DefaultGroupConcurrency is the number of album items that are downloaded at
// the same time.
const DefaultGroupConcurrency = 4

// Flow performs the cache aware delivery of a single media item, an album or
// an audio track.
type Flow struct {
	DB          *storage.DB
	Stats       *runtime.Stats
	Dedupe      *runtime.Dedupe
	Log         *logger.Logger
	MaxFileSize int64
}

// SingleOptions describes a single media delivery.
type SingleOptions struct {
	CacheKey      string
	CacheFileType string
	Source        string
	Kind          string

	UpdateStatus   func(ctx context.Context, text string) error
	SendChatAction func(ctx context.Context, action string) error
	Download       func(ctx context.Context) (*download.Metrics, error)
	Send           func(ctx context.Context, path string) (*tg.Message, error)
	SendCached     func(ctx context.Context, fileID string) (*tg.Message, error)
	Cleanup        func(path string)
	DeleteStatus   func(ctx context.Context) error

	OnMissingMedia    func(ctx context.Context) error
	OnRateLimit       func(ctx context.Context, retryAfter time.Duration) error
	OnQueueBusy       func(ctx context.Context, position int) error
	OnTooLarge        func(ctx context.Context) error
	OnUnexpectedError func(ctx context.Context, err error) error
	Record            func(ctx context.Context, status string, err error)
}

// RunSingle delivers a single media item, reusing the cached file when it is
// available.
func (f *Flow) RunSingle(ctx context.Context, opts SingleOptions) (*tg.Message, error) {
	if opts.Cleanup == nil {
		opts.Cleanup = func(path string) { _ = os.Remove(path) }
	}

	fileID, _, err := f.DB.GetCachedFile(ctx, opts.CacheKey)
	if err != nil {
		f.logError("read cache", err)
	}
	if fileID != "" {
		f.Stats.RecordCacheHit()
		sent, sendErr := opts.SendCached(ctx, fileID)
		if sendErr == nil {
			record(ctx, opts.Record, storage.StatusCached, nil)
			return sent, nil
		}
		f.logError("send cached file", sendErr)
		f.Stats.RecordCacheMiss()
		if isStaleReference(sendErr) {
			// The cache holds a file id of the Bot API, such a file cannot
			// be sent over MTProto, the entry is dropped.
			if err := f.DB.DeleteFile(ctx, opts.CacheKey); err != nil {
				f.logError("drop the stale cache entry", err)
			}
		}
	} else {
		f.Stats.RecordCacheMiss()
	}

	leader, release := f.Dedupe.Begin(opts.CacheKey)
	if !leader {
		return nil, nil
	}
	defer release()

	if opts.UpdateStatus != nil {
		_ = opts.UpdateStatus(ctx, "⏳")
	}
	if opts.SendChatAction != nil {
		_ = opts.SendChatAction(ctx, "upload_document")
	}

	metrics, downloadErr := opts.Download(ctx)
	if downloadErr != nil {
		record(ctx, opts.Record, storage.StatusError, downloadErr)
		f.handleError(ctx, opts, downloadErr)
		return nil, downloadErr
	}
	if metrics == nil {
		record(ctx, opts.Record, storage.StatusError, nil)
		if opts.OnMissingMedia != nil {
			_ = opts.OnMissingMedia(ctx)
		}
		return nil, errors.New("no media downloaded")
	}

	if f.MaxFileSize > 0 && metrics.Size >= f.MaxFileSize {
		opts.Cleanup(metrics.Path)
		record(ctx, opts.Record, storage.StatusError, nil)
		if opts.OnTooLarge != nil {
			_ = opts.OnTooLarge(ctx)
		}
		return nil, errors.New("media exceeds the size limit")
	}

	sent, sendErr := opts.Send(ctx, metrics.Path)
	if sendErr != nil {
		opts.Cleanup(metrics.Path)
		record(ctx, opts.Record, storage.StatusError, sendErr)
		f.handleError(ctx, opts, sendErr)
		return nil, sendErr
	}

	if extracted := ExtractFileID(sent, opts.Kind); extracted != "" {
		if err := f.DB.AddFile(ctx, opts.CacheKey, extracted, opts.CacheFileType); err != nil {
			f.logError("store cache entry", err)
		}
	}
	opts.Cleanup(metrics.Path)
	record(ctx, opts.Record, storage.StatusSuccess, nil)
	return sent, nil
}

func (f *Flow) handleError(ctx context.Context, opts SingleOptions, err error) {
	var tooLarge *download.TooLargeError
	var apiError *tg.APIError
	switch {
	case errors.As(err, &tooLarge):
		if opts.OnTooLarge != nil {
			_ = opts.OnTooLarge(ctx)
		}
	case errors.As(err, &apiError) && apiError.IsTooLarge():
		if opts.OnTooLarge != nil {
			_ = opts.OnTooLarge(ctx)
		}
	case errors.Is(err, download.ErrQueueBusy):
		if opts.OnQueueBusy != nil {
			_ = opts.OnQueueBusy(ctx, 0)
		}
	case errors.Is(err, download.ErrRateLimited):
		if opts.OnRateLimit != nil {
			_ = opts.OnRateLimit(ctx, 0)
		}
	default:
		if opts.OnUnexpectedError != nil {
			_ = opts.OnUnexpectedError(ctx, err)
		}
	}
}

// Recorder stores the outcome of a delivery.
type Recorder func(ctx context.Context, status string, err error)

func record(ctx context.Context, recorder Recorder, status string, err error) {
	if recorder == nil {
		return
	}
	recorder(ctx, status, err)
}

func (f *Flow) logError(message string, err error) {
	if f.Log != nil {
		f.Log.Error("%s: %v", message, err)
	}
}

// GroupItem is a single entry of an album.
type GroupItem struct {
	Index    int
	Kind     string
	CacheKey string
	URL      string
	Download func(ctx context.Context) (*download.Metrics, error)
}

// GroupOptions describes an album delivery.
type GroupOptions struct {
	Items        []GroupItem
	Source       string
	UpdateStatus func(ctx context.Context, text string) error
	SendEntries  func(ctx context.Context, entries []GroupEntry) error
	OnEmpty      func(ctx context.Context) error
	DeleteStatus func(ctx context.Context) error
	Cleanup      func(path string)
	Record       func(ctx context.Context, status string, err error)
	// Concurrency is the number of items that are downloaded at the same time,
	// zero uses the default of the flow.
	Concurrency int
}

// GroupEntry is a resolved album entry.
type GroupEntry struct {
	Index    int
	Kind     string
	CacheKey string
	FileID   string
	Path     string
	Cached   bool
}

// RunGroup downloads and delivers an album.
func (f *Flow) RunGroup(ctx context.Context, opts GroupOptions) error {
	if opts.Cleanup == nil {
		opts.Cleanup = func(path string) { _ = os.Remove(path) }
	}

	entries := f.downloadGroupItems(ctx, opts)

	if len(entries) == 0 {
		if opts.OnEmpty != nil {
			_ = opts.OnEmpty(ctx)
		}
		return errors.New("album is empty")
	}

	if err := opts.SendEntries(ctx, entries); err != nil {
		for _, entry := range entries {
			if entry.Path != "" {
				opts.Cleanup(entry.Path)
			}
		}
		record(ctx, opts.Record, storage.StatusError, err)
		return err
	}

	for _, entry := range entries {
		if entry.Path != "" {
			opts.Cleanup(entry.Path)
		}
	}
	record(ctx, opts.Record, storage.StatusSuccess, nil)
	return nil
}

// downloadGroupItems fetches the items of an album.
//
// The items are downloaded by a small pool of workers, an album of ten photos
// does not wait for the tenth photo to start the first one. The order of the
// album is kept, the entries are returned in the order of the items.
func (f *Flow) downloadGroupItems(ctx context.Context, opts GroupOptions) []GroupEntry {
	entries := make([]GroupEntry, len(opts.Items))
	results := make([]bool, len(opts.Items))

	workers := opts.Concurrency
	if workers <= 0 {
		workers = DefaultGroupConcurrency
	}
	if workers > len(opts.Items) {
		workers = len(opts.Items)
	}
	if workers < 1 {
		workers = 1
	}

	var (
		mu      sync.Mutex
		next    int
		counter sync.WaitGroup
	)

	for worker := 0; worker < workers; worker++ {
		counter.Add(1)
		go func() {
			defer counter.Done()
			for {
				mu.Lock()
				index := next
				next++
				mu.Unlock()
				if index >= len(opts.Items) {
					return
				}

				entry, ok := f.downloadGroupItem(ctx, opts.Items[index])
				mu.Lock()
				entries[index] = entry
				results[index] = ok
				mu.Unlock()
			}
		}()
	}
	counter.Wait()

	ready := make([]GroupEntry, 0, len(entries))
	for index, entry := range entries {
		if !results[index] {
			continue
		}
		ready = append(ready, entry)
	}
	return ready
}

// downloadGroupItem fetches a single item of an album.
func (f *Flow) downloadGroupItem(ctx context.Context, item GroupItem) (GroupEntry, bool) {
	fileID, _, err := f.DB.GetCachedFile(ctx, item.CacheKey)
	if err != nil {
		f.logError("read cache", err)
	}
	if fileID != "" {
		f.Stats.RecordCacheHit()
		return GroupEntry{
			Index: item.Index, Kind: item.Kind, CacheKey: item.CacheKey, FileID: fileID, Cached: true,
		}, true
	}

	f.Stats.RecordCacheMiss()
	metrics, downloadErr := item.Download(ctx)
	if downloadErr != nil || metrics == nil {
		f.logError("download album item", downloadErr)
		return GroupEntry{}, false
	}
	return GroupEntry{
		Index: item.Index, Kind: item.Kind, CacheKey: item.CacheKey, Path: metrics.Path, Cached: false,
	}, true
}

// AudioOptions describes an audio delivery.
type AudioOptions struct {
	CacheKey      string
	CacheFileType string
	Source        string
	Title         string
	Performer     string

	DownloadAudio  func(ctx context.Context) (*download.Metrics, error)
	Prepare        func(ctx context.Context, path string) (string, error)
	Send           func(ctx context.Context, path string, duration int) (*tg.Message, error)
	SendCached     func(ctx context.Context, fileID string) (*tg.Message, error)
	Cleanup        func(path string)
	OnMissingAudio func(ctx context.Context) error
	OnTooLarge     func(ctx context.Context) error
	Record         func(ctx context.Context, status string, err error)
}

// RunAudio delivers an audio track.
func (f *Flow) RunAudio(ctx context.Context, opts AudioOptions) (*tg.Message, error) {
	if opts.Cleanup == nil {
		opts.Cleanup = func(path string) { _ = os.Remove(path) }
	}

	fileID, _, err := f.DB.GetCachedFile(ctx, opts.CacheKey)
	if err != nil {
		f.logError("read cache", err)
	}
	if fileID != "" {
		f.Stats.RecordCacheHit()
		sent, sendErr := opts.SendCached(ctx, fileID)
		if sendErr == nil {
			record(ctx, opts.Record, storage.StatusCached, nil)
			return sent, nil
		}
		f.logError("send cached audio", sendErr)
	} else {
		f.Stats.RecordCacheMiss()
	}

	metrics, downloadErr := opts.DownloadAudio(ctx)
	if downloadErr != nil {
		record(ctx, opts.Record, storage.StatusError, downloadErr)
		f.handleError(ctx, SingleOptions{
			OnTooLarge:        opts.OnTooLarge,
			OnUnexpectedError: func(context.Context, error) error { return nil },
		}, downloadErr)
		return nil, downloadErr
	}
	if metrics == nil {
		record(ctx, opts.Record, storage.StatusError, nil)
		if opts.OnMissingAudio != nil {
			_ = opts.OnMissingAudio(ctx)
		}
		return nil, errors.New("no audio downloaded")
	}

	if f.MaxFileSize > 0 && metrics.Size >= f.MaxFileSize {
		opts.Cleanup(metrics.Path)
		record(ctx, opts.Record, storage.StatusError, nil)
		if opts.OnTooLarge != nil {
			_ = opts.OnTooLarge(ctx)
		}
		return nil, errors.New("audio exceeds the size limit")
	}

	path := metrics.Path
	if opts.Prepare != nil {
		prepared, err := opts.Prepare(ctx, path)
		if err != nil {
			f.logError("prepare audio", err)
		} else if prepared != "" {
			path = prepared
		}
	}

	duration, _ := ProbeAudioDuration(ctx, path)
	sent, sendErr := opts.Send(ctx, path, duration)
	if sendErr != nil {
		opts.Cleanup(path)
		opts.Cleanup(metrics.Path)
		record(ctx, opts.Record, storage.StatusError, sendErr)

		var apiError *tg.APIError
		if errors.As(sendErr, &apiError) && apiError.IsTooLarge() && opts.OnTooLarge != nil {
			_ = opts.OnTooLarge(ctx)
		}
		return nil, sendErr
	}

	if extracted := ExtractFileID(sent, "audio"); extracted != "" {
		if err := f.DB.AddFile(ctx, opts.CacheKey, extracted, opts.CacheFileType); err != nil {
			f.logError("store audio cache entry", err)
		}
	}
	opts.Cleanup(path)
	opts.Cleanup(metrics.Path)
	record(ctx, opts.Record, storage.StatusSuccess, nil)
	return sent, nil
}

// ExtractFileID reads the Telegram file identifier out of a sent message.
func ExtractFileID(message *tg.Message, kind string) string {
	if message == nil {
		return ""
	}
	switch kind {
	case "video":
		if message.Video != nil {
			return message.Video.FileID
		}
		if message.Document != nil {
			return message.Document.FileID
		}
	case "photo":
		if len(message.Photo) > 0 {
			return message.Photo[len(message.Photo)-1].FileID
		}
		if message.Document != nil {
			return message.Document.FileID
		}
	case "audio":
		if message.Audio != nil {
			return message.Audio.FileID
		}
		if message.Document != nil {
			return message.Document.FileID
		}
	}
	if message.Document != nil {
		return message.Document.FileID
	}
	if message.Video != nil {
		return message.Video.FileID
	}
	if len(message.Photo) > 0 {
		return message.Photo[len(message.Photo)-1].FileID
	}
	if message.Audio != nil {
		return message.Audio.FileID
	}
	return ""
}

// audioExtensions lists the containers a download can produce.
var audioExtensions = []string{".mp3", ".m4a", ".flac", ".opus", ".webm", ".wav", ".aac", ".ogg", ".m4b"}

// CleanupPath removes a downloaded file and its leftovers.
//
// The conversion, the tagging and the cover embedding write sibling files, so
// the whole family is removed.
func CleanupPath(path string) {
	if path == "" {
		return
	}
	directory := filepath.Dir(path)
	base := filepath.Base(path)
	extension := filepath.Ext(base)
	stem := strings.TrimSuffix(base, extension)

	_ = os.Remove(path)
	_ = os.Remove(path + ".part")
	_ = os.Remove(filepath.Join(directory, "tagged-"+base))
	_ = os.Remove(filepath.Join(directory, stem+".cover"+extension))

	stems := []string{stem}
	plain := strings.TrimSuffix(strings.TrimPrefix(stem, "tagged-"), ".cover")
	if plain != stem {
		stems = append(stems, plain)
	}
	if plain != "" {
		stems = append(stems, "tagged-"+plain)
	}
	for _, name := range stems {
		for _, candidate := range audioExtensions {
			_ = os.Remove(filepath.Join(directory, name+candidate))
			_ = os.Remove(filepath.Join(directory, name+".cover"+candidate))
		}
	}
}

// CleanupThumbnail removes a prepared thumbnail and its source image.
func CleanupThumbnail(path string) {
	if path == "" {
		return
	}
	directory := filepath.Dir(path)
	_ = os.Remove(path)

	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "cover") {
			continue
		}
		_ = os.Remove(filepath.Join(directory, entry.Name()))
	}
	_ = os.Remove(directory)
}

// DescribeError renders an error for the user facing messages.
func DescribeError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(err)
}

// isStaleReference reports whether the error comes from a cached file that
// belongs to the Bot API.
func isStaleReference(err error) bool {
	var apiError *tg.APIError
	return errors.As(err, &apiError) && apiError.IsStaleReference()
}
