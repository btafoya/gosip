package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/btafoya/gosip/internal/models"
)

var ErrMessageNotFound = errors.New("message not found")

// MessageRepository handles database operations for SMS/MMS messages
type MessageRepository struct {
	db *sql.DB
}

// NewMessageRepository creates a new MessageRepository
func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create inserts a new message
func (r *MessageRepository) Create(ctx context.Context, msg *models.Message) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO messages (message_sid, direction, from_number, to_number, did_id, body, media_urls, status, created_at, is_read)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.MessageSID, msg.Direction, msg.FromNumber, msg.ToNumber, msg.DIDID, msg.Body, msg.MediaURLs, msg.Status, time.Now(), msg.IsRead)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	msg.ID = id
	return nil
}

// GetByID retrieves a message by ID
func (r *MessageRepository) GetByID(ctx context.Context, id int64) (*models.Message, error) {
	msg := &models.Message{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, message_sid, direction, from_number, to_number, did_id, body, media_urls, status, created_at, is_read
		FROM messages WHERE id = ?
	`, id).Scan(&msg.ID, &msg.MessageSID, &msg.Direction, &msg.FromNumber, &msg.ToNumber, &msg.DIDID, &msg.Body, &msg.MediaURLs, &msg.Status, &msg.CreatedAt, &msg.IsRead)
	if err == sql.ErrNoRows {
		return nil, ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// GetByMessageSID retrieves a message by Twilio Message SID
func (r *MessageRepository) GetByMessageSID(ctx context.Context, messageSID string) (*models.Message, error) {
	msg := &models.Message{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, message_sid, direction, from_number, to_number, did_id, body, media_urls, status, created_at, is_read
		FROM messages WHERE message_sid = ?
	`, messageSID).Scan(&msg.ID, &msg.MessageSID, &msg.Direction, &msg.FromNumber, &msg.ToNumber, &msg.DIDID, &msg.Body, &msg.MediaURLs, &msg.Status, &msg.CreatedAt, &msg.IsRead)
	if err == sql.ErrNoRows {
		return nil, ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// Update updates an existing message
func (r *MessageRepository) Update(ctx context.Context, msg *models.Message) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE messages SET message_sid = ?, direction = ?, from_number = ?, to_number = ?,
		did_id = ?, body = ?, media_urls = ?, status = ?, is_read = ?
		WHERE id = ?
	`, msg.MessageSID, msg.Direction, msg.FromNumber, msg.ToNumber, msg.DIDID, msg.Body, msg.MediaURLs, msg.Status, msg.IsRead, msg.ID)
	return err
}

// UpdateStatus updates the status of a message
func (r *MessageRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE messages SET status = ? WHERE id = ?`, status, id)
	return err
}

// Delete removes a message
func (r *MessageRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM messages WHERE id = ?`, id)
	return err
}

// MarkAsRead marks a message as read
func (r *MessageRepository) MarkAsRead(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE messages SET is_read = 1 WHERE id = ?`, id)
	return err
}

// List returns messages with pagination
func (r *MessageRepository) List(ctx context.Context, limit, offset int) ([]*models.Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, message_sid, direction, from_number, to_number, did_id, body, media_urls, status, created_at, is_read
		FROM messages ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		if err := rows.Scan(&msg.ID, &msg.MessageSID, &msg.Direction, &msg.FromNumber, &msg.ToNumber, &msg.DIDID, &msg.Body, &msg.MediaURLs, &msg.Status, &msg.CreatedAt, &msg.IsRead); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// ListByDID returns messages for a specific DID
func (r *MessageRepository) ListByDID(ctx context.Context, didID int64, limit, offset int) ([]*models.Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, message_sid, direction, from_number, to_number, did_id, body, media_urls, status, created_at, is_read
		FROM messages WHERE did_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, didID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		if err := rows.Scan(&msg.ID, &msg.MessageSID, &msg.Direction, &msg.FromNumber, &msg.ToNumber, &msg.DIDID, &msg.Body, &msg.MediaURLs, &msg.Status, &msg.CreatedAt, &msg.IsRead); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// GetConversation returns messages between a DID and a specific phone number (threaded view)
func (r *MessageRepository) GetConversation(ctx context.Context, didID int64, phoneNumber string, limit, offset int) ([]*models.Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, message_sid, direction, from_number, to_number, did_id, body, media_urls, status, created_at, is_read
		FROM messages
		WHERE did_id = ? AND (from_number = ? OR to_number = ?)
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, didID, phoneNumber, phoneNumber, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		if err := rows.Scan(&msg.ID, &msg.MessageSID, &msg.Direction, &msg.FromNumber, &msg.ToNumber, &msg.DIDID, &msg.Body, &msg.MediaURLs, &msg.Status, &msg.CreatedAt, &msg.IsRead); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// ListUnread returns unread messages
func (r *MessageRepository) ListUnread(ctx context.Context) ([]*models.Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, message_sid, direction, from_number, to_number, did_id, body, media_urls, status, created_at, is_read
		FROM messages WHERE is_read = 0 ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		if err := rows.Scan(&msg.ID, &msg.MessageSID, &msg.Direction, &msg.FromNumber, &msg.ToNumber, &msg.DIDID, &msg.Body, &msg.MediaURLs, &msg.Status, &msg.CreatedAt, &msg.IsRead); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// CountUnread returns the count of unread messages
func (r *MessageRepository) CountUnread(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE is_read = 0`).Scan(&count)
	return count, err
}

// Count returns the total number of messages
func (r *MessageRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&count)
	return count, err
}

// GetConversationSummaries returns a summary of conversations (latest message per conversation)
func (r *MessageRepository) GetConversationSummaries(ctx context.Context, didID int64) ([]map[string]interface{}, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			CASE WHEN direction = 'inbound' THEN from_number ELSE to_number END as phone_number,
			MAX(created_at) as last_message_at,
			COUNT(*) as message_count,
			SUM(CASE WHEN is_read = 0 AND direction = 'inbound' THEN 1 ELSE 0 END) as unread_count
		FROM messages
		WHERE did_id = ?
		GROUP BY CASE WHEN direction = 'inbound' THEN from_number ELSE to_number END
		ORDER BY last_message_at DESC
	`, didID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []map[string]interface{}
	for rows.Next() {
		var phoneNumber string
		var lastMessageAt time.Time
		var messageCount, unreadCount int
		if err := rows.Scan(&phoneNumber, &lastMessageAt, &messageCount, &unreadCount); err != nil {
			return nil, err
		}
		summaries = append(summaries, map[string]interface{}{
			"phone_number":    phoneNumber,
			"last_message_at": lastMessageAt,
			"message_count":   messageCount,
			"unread_count":    unreadCount,
		})
	}
	return summaries, rows.Err()
}
