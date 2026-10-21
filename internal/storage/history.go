package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// HistoryFilter narrows down a download history query.
type HistoryFilter struct {
	Service string
	Status  string
	Limit   int
	Offset  int
}

// RecordDownload appends an entry to the download history.
func (d *DB) RecordDownload(ctx context.Context, record *DownloadRecord) error {
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO download_history (user_id, chat_id, chat_type, service, url, title,
			file_type, file_id, file_size_bytes, duration_seconds, status, error_message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())`,
		record.UserID,
		record.ChatID,
		record.ChatType,
		record.Service,
		record.URL,
		record.Title,
		record.FileType,
		record.FileID,
		record.FileSizeBytes,
		record.DurationSeconds,
		record.Status,
		record.ErrorMessage)
	return err
}

// CountHistory returns the number of history entries matching a filter.
func (d *DB) CountHistory(ctx context.Context, filter HistoryFilter) (int, error) {
	query := `SELECT COUNT(*) FROM download_history WHERE 1 = 1`
	args := []any{}
	if filter.Service != "" && filter.Service != "all" {
		query += ` AND service = $` + fmt.Sprint(len(args)+1)
		args = append(args, filter.Service)
	}
	if filter.Status != "" && filter.Status != "all" {
		query += ` AND status = $` + fmt.Sprint(len(args)+1)
		args = append(args, filter.Status)
	}

	var count int
	err := d.sql.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// ListHistory returns the download history entries matching a filter.
func (d *DB) ListHistory(ctx context.Context, filter HistoryFilter) ([]*DownloadRecord, error) {
	query := `
		SELECT h.id, h.user_id, h.chat_id, COALESCE(h.chat_type, ''), h.service, h.url,
			COALESCE(h.title, ''), COALESCE(h.file_type, ''), COALESCE(h.file_id, ''),
			h.file_size_bytes, h.duration_seconds, h.status, COALESCE(h.error_message, ''), h.created_at,
			COALESCE(u.user_username, ''), COALESCE(u.user_name, '')
		FROM download_history h
		LEFT JOIN users u ON u.user_id = h.user_id
		WHERE 1 = 1`
	args := []any{}
	if filter.Service != "" && filter.Service != "all" {
		query += ` AND h.service = $` + fmt.Sprint(len(args)+1)
		args = append(args, filter.Service)
	}
	if filter.Status != "" && filter.Status != "all" {
		query += ` AND h.status = $` + fmt.Sprint(len(args)+1)
		args = append(args, filter.Status)
	}
	query += ` ORDER BY h.created_at DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ` + fmt.Sprint(filter.Limit)
	}
	if filter.Offset > 0 {
		query += ` OFFSET ` + fmt.Sprint(filter.Offset)
	}

	rows, err := d.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read download history: %w", err)
	}
	defer rows.Close()

	var records []*DownloadRecord
	for rows.Next() {
		record := &DownloadRecord{}
		if err := rows.Scan(
			&record.ID, &record.UserID, &record.ChatID, &record.ChatType, &record.Service,
			&record.URL, &record.Title, &record.FileType, &record.FileID,
			&record.FileSizeBytes, &record.DurationSeconds, &record.Status, &record.ErrorMessage,
			&record.CreatedAt, &record.Username, &record.UserDisplayName,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// CountDownloadsByService returns the number of downloads per service.
func (d *DB) CountDownloadsByService(ctx context.Context) (map[string]int, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT service, COUNT(*) FROM download_history GROUP BY service`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var service string
		var count int
		if err := rows.Scan(&service, &count); err != nil {
			return nil, err
		}
		counts[service] = count
	}
	return counts, rows.Err()
}

// CountDownloadsByStatus returns the number of downloads per status.
func (d *DB) CountDownloadsByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT status, COUNT(*) FROM download_history GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

// LastDownload returns the timestamp of the most recent download.
func (d *DB) LastDownload(ctx context.Context) (time.Time, bool, error) {
	var createdAt time.Time
	err := d.sql.QueryRowContext(ctx, `SELECT created_at FROM download_history ORDER BY created_at DESC LIMIT 1`).Scan(&createdAt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return createdAt, true, nil
}
