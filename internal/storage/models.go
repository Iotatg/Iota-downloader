package storage

import "time"

// User is a row of the users table.
type User struct {
	UserID       int64
	UserName     string
	UserUsername string
	ChatType     string
	Language     string
	HasDM        bool
	Status       string
	ReferredBy   *int64
	Source       string
}

// Group is a row of the groups table.
type Group struct {
	ID           int64
	Title        string
	Username     string
	ChatType     string
	Status       string
	MemberCount  int64
	LastThreadID *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DownloadRecord is a row of the download_history table.
type DownloadRecord struct {
	ID              int64
	UserID          int64
	ChatID          *int64
	ChatType        string
	Service         string
	URL             string
	Title           string
	FileType        string
	FileID          string
	FileSizeBytes   *int64
	DurationSeconds *float64
	Status          string
	ErrorMessage    string
	CreatedAt       time.Time
	Username        string
	UserDisplayName string
}

// AnalyticsEvent is a row of the analytics_events table.
type AnalyticsEvent struct {
	ID         int64
	UserID     int64
	ChatType   string
	ActionName string
	CreatedAt  time.Time
}

// StatsBucket is a single bucket of the statistics aggregation.
type StatsBucket struct {
	Bucket  string
	Total   int
	Service string
}

// ChatCounts holds the aggregated chat statistics used by the admin panel.
type ChatCounts struct {
	TotalChats    int
	PrivateChats  int
	GroupChats    int
	ActiveChats   int
	InactiveChats int
	BannedChats   int
}

// DownloadStatus values used by the history table.
const (
	StatusSuccess = "success"
	StatusCached  = "cached"
	StatusError   = "error"
)
