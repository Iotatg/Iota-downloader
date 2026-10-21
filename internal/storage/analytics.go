package storage

import (
	"context"
	"fmt"
	"time"
)

// InsertAnalyticsEvents stores a batch of analytics events.
func (d *DB) InsertAnalyticsEvents(ctx context.Context, events []AnalyticsEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin analytics batch: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO analytics_events (user_id, chat_type, action_name, created_at)
		VALUES ($1, $2, $3, now())`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare analytics insert: %w", err)
	}
	defer stmt.Close()

	for _, event := range events {
		if _, err := stmt.ExecContext(ctx, event.UserID, event.ChatType, event.ActionName); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert analytics event: %w", err)
		}
	}
	return tx.Commit()
}

// AnalyticsBucket returns the aggregation bucket of a timestamp. The bucket is
// either the calendar day or the calendar month.
func AnalyticsBucket(ts time.Time, monthly bool) string {
	if monthly {
		return ts.Format("2006-01")
	}
	return ts.Format("2006-01-02")
}

// StatsSince aggregates the download history since the given moment.
//
// The first result contains the totals per bucket, the second one the totals
// per service inside the same period.
func (d *DB) StatsSince(ctx context.Context, since time.Time, monthly bool) ([]StatsBucket, []StatsBucket, error) {
	bucketExpr := "to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD')"
	if monthly {
		bucketExpr = "to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM')"
	}

	rows, err := d.sql.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*) AS total
		FROM download_history
		WHERE created_at >= $1
		GROUP BY bucket
		ORDER BY bucket`, bucketExpr), since)
	if err != nil {
		return nil, nil, fmt.Errorf("aggregate stats: %w", err)
	}
	defer rows.Close()

	var totals []StatsBucket
	for rows.Next() {
		var bucket StatsBucket
		if err := rows.Scan(&bucket.Bucket, &bucket.Total); err != nil {
			return nil, nil, err
		}
		totals = append(totals, bucket)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	rows, err = d.sql.QueryContext(ctx, `
		SELECT service, COUNT(*) AS total
		FROM download_history
		WHERE created_at >= $1
		GROUP BY service
		ORDER BY total DESC`, since)
	if err != nil {
		return nil, nil, fmt.Errorf("aggregate service stats: %w", err)
	}
	defer rows.Close()

	var services []StatsBucket
	for rows.Next() {
		var bucket StatsBucket
		if err := rows.Scan(&bucket.Service, &bucket.Total); err != nil {
			return nil, nil, err
		}
		services = append(services, bucket)
	}
	return totals, services, rows.Err()
}

// StatsSinceByService aggregates the download history per service and day.
//
// The result maps a service to its daily totals.
func (d *DB) StatsSinceByService(ctx context.Context, since time.Time) (map[string]map[string]int, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT service, to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS bucket, COUNT(*) AS total
		FROM download_history
		WHERE created_at >= $1
		GROUP BY service, bucket
		ORDER BY bucket`, since)
	if err != nil {
		return nil, fmt.Errorf("aggregate service stats by day: %w", err)
	}
	defer rows.Close()

	result := map[string]map[string]int{}
	for rows.Next() {
		var (
			service string
			bucket  string
			total   int
		)
		if err := rows.Scan(&service, &bucket, &total); err != nil {
			return nil, err
		}
		if _, ok := result[service]; !ok {
			result[service] = map[string]int{}
		}
		result[service][bucket] = total
	}
	return result, rows.Err()
}

// CountAnalyticsEvents returns the total number of stored analytics events.
func (d *DB) CountAnalyticsEvents(ctx context.Context) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM analytics_events`).Scan(&count)
	return count, err
}

// CountAnalyticsActions returns how often an action was recorded.
func (d *DB) CountAnalyticsActions(ctx context.Context, action string) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM analytics_events WHERE action_name = $1`, action).Scan(&count)
	return count, err
}

// TopActionsSince returns the most recorded actions of a period.
func (d *DB) TopActionsSince(ctx context.Context, since time.Time) ([]StatsBucket, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT action_name, COUNT(*) FROM analytics_events
		WHERE created_at >= $1 GROUP BY action_name ORDER BY 2 DESC LIMIT 20`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []StatsBucket
	for rows.Next() {
		var bucket StatsBucket
		if err := rows.Scan(&bucket.Service, &bucket.Total); err != nil {
			return nil, err
		}
		result = append(result, bucket)
	}
	return result, rows.Err()
}
