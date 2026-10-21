package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UpsertUser stores the identity of a user and returns the stored record.
func (d *DB) UpsertUser(ctx context.Context, userID int64, name, username, chatType, language string) (*User, error) {
	row := d.sql.QueryRowContext(ctx, `
		INSERT INTO users (user_id, user_name, user_username, chat_type, language, has_dm, status)
		VALUES ($1, $2, $3, $4, $5, false, 'active')
		ON CONFLICT (user_id) DO UPDATE SET
			user_name = COALESCE(EXCLUDED.user_name, users.user_name),
			user_username = COALESCE(EXCLUDED.user_username, users.user_username),
			chat_type = COALESCE(EXCLUDED.chat_type, users.chat_type),
			language = COALESCE(EXCLUDED.language, users.language)
		RETURNING user_id, COALESCE(user_name, ''), COALESCE(user_username, ''), COALESCE(chat_type, 'private'),
			COALESCE(language, ''), has_dm, COALESCE(status, 'active'), referred_by, COALESCE(source, '')`,
		userID, name, username, chatType, language)

	return scanUser(row)
}

// GetUser returns a single user record.
func (d *DB) GetUser(ctx context.Context, userID int64) (*User, error) {
	row := d.sql.QueryRowContext(ctx, `
		SELECT user_id, COALESCE(user_name, ''), COALESCE(user_username, ''), COALESCE(chat_type, 'private'),
			COALESCE(language, ''), has_dm, COALESCE(status, 'active'), referred_by, COALESCE(source, '')
		FROM users WHERE user_id = $1`, userID)
	return scanUser(row)
}

// SetUserStatus updates the activity status of a user.
func (d *DB) SetUserStatus(ctx context.Context, userID int64, status string) error {
	_, err := d.sql.ExecContext(ctx, `UPDATE users SET status = $2 WHERE user_id = $1`, userID, status)
	return err
}

// SetUserHasDM marks that the user opened a private chat with the bot.
func (d *DB) SetUserHasDM(ctx context.Context, userID int64, hasDM bool) error {
	_, err := d.sql.ExecContext(ctx, `UPDATE users SET has_dm = $2 WHERE user_id = $1`, userID, hasDM)
	return err
}

// SetUserReferral links a user to the inviter that referred them.
func (d *DB) SetUserReferral(ctx context.Context, userID, referredBy int64) error {
	_, err := d.sql.ExecContext(ctx, `UPDATE users SET referred_by = $2 WHERE user_id = $1`, userID, referredBy)
	return err
}

// SetUserSource stores the acquisition source of a user.
func (d *DB) SetUserSource(ctx context.Context, userID int64, source string) error {
	_, err := d.sql.ExecContext(ctx, `UPDATE users SET source = $2 WHERE user_id = $1`, userID, source)
	return err
}

// CountUsers returns the number of registered users.
func (d *DB) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// CountUsersByStatus returns the number of users with the given status.
func (d *DB) CountUsersByStatus(ctx context.Context, status string) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE status = $1`, status).Scan(&count)
	return count, err
}

// CountUsersWithDM returns the number of users that opened a private chat.
func (d *DB) CountUsersWithDM(ctx context.Context) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE has_dm = true`).Scan(&count)
	return count, err
}

// ListUserIDs returns every registered user identifier.
func (d *DB) ListUserIDs(ctx context.Context) ([]int64, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT user_id FROM users ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CountReferredUsers returns how many users were invited by the given user.
func (d *DB) CountReferredUsers(ctx context.Context, userID int64) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE referred_by = $1`, userID).Scan(&count)
	return count, err
}

// CountUsersBySource returns the number of users per acquisition source.
func (d *DB) CountUsersBySource(ctx context.Context) (map[string]int, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT COALESCE(source, ''), COUNT(*) FROM users GROUP BY source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var source string
		var count int
		if err := rows.Scan(&source, &count); err != nil {
			return nil, err
		}
		counts[source] = count
	}
	return counts, rows.Err()
}

func scanUser(row *sql.Row) (*User, error) {
	user := &User{}
	err := row.Scan(
		&user.UserID,
		&user.UserName,
		&user.UserUsername,
		&user.ChatType,
		&user.Language,
		&user.HasDM,
		&user.Status,
		&user.ReferredBy,
		&user.Source,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}

// TrackedGroupCount returns the number of groups known to the bot.
func (d *DB) TrackedGroupCount(ctx context.Context) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups`).Scan(&count)
	return count, err
}

// TrackedGroupMemberCount returns the number of tracked group members.
func (d *DB) TrackedGroupMemberCount(ctx context.Context) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_members`).Scan(&count)
	return count, err
}

// UpsertGroup stores or refreshes a group record.
func (d *DB) UpsertGroup(ctx context.Context, group *Group) error {
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO groups (id, title, username, chat_type, status, member_count, last_thread_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())
		ON CONFLICT (id) DO UPDATE SET
			title = COALESCE(EXCLUDED.title, groups.title),
			username = COALESCE(EXCLUDED.username, groups.username),
			chat_type = COALESCE(EXCLUDED.chat_type, groups.chat_type),
			status = EXCLUDED.status,
			member_count = EXCLUDED.member_count,
			last_thread_id = EXCLUDED.last_thread_id,
			updated_at = now()`,
		group.ID, group.Title, group.Username, group.ChatType, group.Status, group.MemberCount, group.LastThreadID)
	return err
}

// SetGroupStatus updates the status of a tracked group.
func (d *DB) SetGroupStatus(ctx context.Context, groupID int64, status string) error {
	_, err := d.sql.ExecContext(ctx, `UPDATE groups SET status = $2, updated_at = now() WHERE id = $1`, groupID, status)
	return err
}

// SetGroupMemberCount updates the cached member count of a group.
func (d *DB) SetGroupMemberCount(ctx context.Context, groupID, count int64) error {
	_, err := d.sql.ExecContext(ctx, `UPDATE groups SET member_count = $2, updated_at = now() WHERE id = $1`, groupID, count)
	return err
}

// ListGroups returns every tracked group.
func (d *DB) ListGroups(ctx context.Context) ([]*Group, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, COALESCE(title, ''), COALESCE(username, ''), COALESCE(chat_type, ''),
			status, member_count, last_thread_id, created_at, updated_at
		FROM groups ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		group := &Group{}
		if err := rows.Scan(&group.ID, &group.Title, &group.Username, &group.ChatType,
			&group.Status, &group.MemberCount, &group.LastThreadID, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

// ListGroupsByStatus returns the tracked groups with the given status.
func (d *DB) ListGroupsByStatus(ctx context.Context, status string) ([]*Group, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, COALESCE(title, ''), COALESCE(username, ''), COALESCE(chat_type, ''),
			status, member_count, last_thread_id, created_at, updated_at
		FROM groups WHERE status = $1 ORDER BY id`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		group := &Group{}
		if err := rows.Scan(&group.ID, &group.Title, &group.Username, &group.ChatType,
			&group.Status, &group.MemberCount, &group.LastThreadID, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

// TrackGroupMember records that a user was seen inside a group.
func (d *DB) TrackGroupMember(ctx context.Context, groupID, userID int64) error {
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO group_members (group_id, user_id, last_seen_at)
		VALUES ($1, $2, now())
		ON CONFLICT (group_id, user_id) DO UPDATE SET last_seen_at = now()`, groupID, userID)
	return err
}

// CountGroupMembers returns how many members are tracked for a group.
func (d *DB) CountGroupMembers(ctx context.Context, groupID int64) (int, error) {
	var count int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_members WHERE group_id = $1`, groupID).Scan(&count)
	return count, err
}

// KnownChat looks up a chat in the tracked groups table.
func (d *DB) KnownChat(ctx context.Context, chatID int64) (*Group, error) {
	group := &Group{}
	err := d.sql.QueryRowContext(ctx, `
		SELECT id, COALESCE(title, ''), COALESCE(username, ''), COALESCE(chat_type, ''),
			status, member_count, last_thread_id, created_at, updated_at
		FROM groups WHERE id = $1`, chatID).Scan(
		&group.ID, &group.Title, &group.Username, &group.ChatType,
		&group.Status, &group.MemberCount, &group.LastThreadID, &group.CreatedAt, &group.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return group, nil
}

// TouchUser refreshes the last activity timestamp of a user record.
func (d *DB) TouchUser(ctx context.Context, userID int64) error {
	_, err := d.sql.ExecContext(ctx, `UPDATE users SET status = 'active' WHERE user_id = $1`, userID)
	return err
}

// nowUTC returns the current time in UTC, used by the timestamp helpers.
func nowUTC() time.Time { return time.Now().UTC() }
