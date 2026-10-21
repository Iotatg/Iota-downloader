package runtime

import (
	"sync"
	"time"
)

// SourceStats holds the download counters of a single source platform.
type SourceStats struct {
	Jobs       int
	Bytes      int64
	TotalMS    float64
	QueueWait  []float64
	Processing []float64
}

// Stats collects the live runtime metrics shown by the admin panel.
type Stats struct {
	mu sync.Mutex

	startedAt time.Time

	totalDownloads int
	videos         int
	audio          int
	other          int
	totalBytes     int64

	cacheHits   int
	cacheMisses int

	analyticsDrops      int
	lastAnalyticsDropAt time.Time

	sources map[string]*SourceStats

	lastDownloadAt time.Time
}

// NewStats creates the runtime statistics collector.
func NewStats() *Stats {
	return &Stats{
		startedAt: time.Now(),
		sources:   map[string]*SourceStats{},
	}
}

// RecordDownload accounts a finished download.
func (s *Stats) RecordDownload(source, kind string, size int64, elapsed time.Duration, queueWait time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.totalDownloads++
	s.totalBytes += size
	switch kind {
	case "video":
		s.videos++
	case "audio":
		s.audio++
	default:
		s.other++
	}
	s.lastDownloadAt = time.Now()

	entry, ok := s.sources[source]
	if !ok {
		entry = &SourceStats{}
		s.sources[source] = entry
	}
	entry.Jobs++
	entry.Bytes += size
	entry.TotalMS += float64(elapsed.Milliseconds())
	entry.QueueWait = append(entry.QueueWait, float64(queueWait.Milliseconds()))
	entry.Processing = append(entry.Processing, float64(elapsed.Milliseconds()))
	if len(entry.QueueWait) > 500 {
		entry.QueueWait = entry.QueueWait[len(entry.QueueWait)-500:]
	}
	if len(entry.Processing) > 500 {
		entry.Processing = entry.Processing[len(entry.Processing)-500:]
	}
}

// RecordCacheHit accounts a cache hit.
func (s *Stats) RecordCacheHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheHits++
}

// RecordCacheMiss accounts a cache miss.
func (s *Stats) RecordCacheMiss() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheMisses++
}

// RecordAnalyticsDrop accounts a dropped analytics event.
func (s *Stats) RecordAnalyticsDrop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.analyticsDrops++
	s.lastAnalyticsDropAt = time.Now()
}

// Snapshot is an immutable copy of the runtime statistics.
type Snapshot struct {
	Uptime          time.Duration
	TotalDownloads  int
	Videos          int
	Audio           int
	Other           int
	TotalBytes      int64
	CacheHits       int
	CacheMisses     int
	AnalyticsDrops  int
	LastDropAge     time.Duration
	LastDownloadAge time.Duration
	HasLastDownload bool
	Sources         map[string]SourceStats
}

// Snapshot returns a copy of the current statistics.
func (s *Stats) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	sources := make(map[string]SourceStats, len(s.sources))
	for name, entry := range s.sources {
		copied := *entry
		copied.QueueWait = append([]float64(nil), entry.QueueWait...)
		copied.Processing = append([]float64(nil), entry.Processing...)
		sources[name] = copied
	}

	snapshot := Snapshot{
		Uptime:         time.Since(s.startedAt),
		TotalDownloads: s.totalDownloads,
		Videos:         s.videos,
		Audio:          s.audio,
		Other:          s.other,
		TotalBytes:     s.totalBytes,
		CacheHits:      s.cacheHits,
		CacheMisses:    s.cacheMisses,
		AnalyticsDrops: s.analyticsDrops,
		Sources:        sources,
	}
	if !s.lastAnalyticsDropAt.IsZero() {
		snapshot.LastDropAge = time.Since(s.lastAnalyticsDropAt)
	}
	if !s.lastDownloadAt.IsZero() {
		snapshot.LastDownloadAge = time.Since(s.lastDownloadAt)
		snapshot.HasLastDownload = true
	}
	return snapshot
}

// CacheHitRate returns the ratio of cache hits in percent.
func (s Snapshot) CacheHitRate() float64 {
	total := s.CacheHits + s.CacheMisses
	if total == 0 {
		return 0
	}
	return float64(s.CacheHits) / float64(total) * 100
}

// Percentile returns the p-th percentile of the given sample.
func Percentile(samples []float64, p float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]float64(nil), samples...)
	for i := 1; i < len(sorted); i++ {
		value := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > value {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = value
	}
	index := int(float64(len(sorted)-1) * p / 100)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}
