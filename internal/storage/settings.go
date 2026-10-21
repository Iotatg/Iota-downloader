package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Iotatg/iota-downloader/internal/settings"
)

// GetSettings loads the preference set of a user, creating the default row on
// first access.
func (d *DB) GetSettings(ctx context.Context, userID int64) (settings.Set, error) {
	set, err := d.readSettings(ctx, userID)
	if err != nil {
		return settings.DefaultSet(), err
	}
	if set != nil {
		return *set, nil
	}
	if err := d.EnsureSettings(ctx, userID); err != nil {
		return settings.DefaultSet(), err
	}
	set, err = d.readSettings(ctx, userID)
	if err != nil {
		return settings.DefaultSet(), err
	}
	if set == nil {
		return settings.DefaultSet(), nil
	}
	return *set, nil
}

func (d *DB) readSettings(ctx context.Context, userID int64) (*settings.Set, error) {
	row := d.sql.QueryRowContext(ctx, `
		SELECT captions, delete_message, info_buttons, url_button, audio_button,
			file_button, video_quality, as_document, audio_format
		FROM settings WHERE user_id = $1`, userID)

	var set settings.Set
	err := row.Scan(&set.Captions, &set.DeleteMessage, &set.InfoButtons, &set.URLButton,
		&set.AudioButton, &set.FileButton, &set.VideoQuality, &set.AsDocument, &set.AudioFormat)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("read settings: %w", err)
	}
	return &set, nil
}

// EnsureSettings inserts the default preference row for a user.
func (d *DB) EnsureSettings(ctx context.Context, userID int64) error {
	defaults := settings.DefaultSet()
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO settings (user_id, captions, delete_message, info_buttons, url_button,
			audio_button, file_button, video_quality, as_document, audio_format)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id) DO NOTHING`,
		userID,
		defaults.Captions,
		defaults.DeleteMessage,
		defaults.InfoButtons,
		defaults.URLButton,
		defaults.AudioButton,
		defaults.FileButton,
		defaults.VideoQuality,
		defaults.AsDocument,
		defaults.AudioFormat)
	return err
}

// SetSetting updates a single preference of a user.
func (d *DB) SetSetting(ctx context.Context, userID int64, field, value string) error {
	column, ok := settingColumns[field]
	if !ok {
		return fmt.Errorf("unknown setting field %q", field)
	}
	if err := d.EnsureSettings(ctx, userID); err != nil {
		return err
	}
	query := fmt.Sprintf(`UPDATE settings SET %s = $2 WHERE user_id = $1`, column)
	_, err := d.sql.ExecContext(ctx, query, userID, value)
	return err
}

var settingColumns = map[string]string{
	settings.FieldCaptions:      "captions",
	settings.FieldDeleteMessage: "delete_message",
	settings.FieldInfoButtons:   "info_buttons",
	settings.FieldURLButton:     "url_button",
	settings.FieldAudioButton:   "audio_button",
	settings.FieldFileButton:    "file_button",
	settings.FieldVideoQuality:  "video_quality",
	settings.FieldAsDocument:    "as_document",
	settings.FieldAudioFormat:   "audio_format",
}

// GetFileID returns the cached Telegram file identifier of a media item.
func (d *DB) GetFileID(ctx context.Context, cacheKey string) (string, error) {
	var fileID string
	err := d.sql.QueryRowContext(ctx, `SELECT file_id FROM downloaded_files WHERE url = $1`, cacheKey).Scan(&fileID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("read file cache: %w", err)
	}
	return fileID, nil
}

// GetCachedFile returns the cached file identifier and its type.
func (d *DB) GetCachedFile(ctx context.Context, cacheKey string) (string, string, error) {
	var fileID, fileType string
	err := d.sql.QueryRowContext(ctx, `SELECT file_id, COALESCE(file_type, '') FROM downloaded_files WHERE url = $1`, cacheKey).
		Scan(&fileID, &fileType)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil
		}
		return "", "", fmt.Errorf("read file cache: %w", err)
	}
	return fileID, fileType, nil
}

// AddFile stores a Telegram file identifier for a cache key.
func (d *DB) AddFile(ctx context.Context, cacheKey, fileID, fileType string) error {
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO downloaded_files (url, file_id, file_type, date_added)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (url) DO UPDATE SET file_id = EXCLUDED.file_id, file_type = EXCLUDED.file_type, date_added = now()`,
		cacheKey, fileID, fileType)
	return err
}

// DeleteFile removes a cache entry.
func (d *DB) DeleteFile(ctx context.Context, cacheKey string) error {
	_, err := d.sql.ExecContext(ctx, `DELETE FROM downloaded_files WHERE url = $1`, cacheKey)
	return err
}

// CountCachedFiles returns the number of cached files.
func (d *DB) CountCachedFiles(ctx context.Context) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM downloaded_files`).Scan(&count)
	return count, err
}
