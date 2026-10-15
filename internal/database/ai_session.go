package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"uuid"
)

// AISession represents an AI chat session
type AISession struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message UI status values (persisted for history replay; ignored by Genkit).
const (
	MessageStatusCancelled = "cancelled"
	MessageStatusDenied    = "denied"
)

// AIMessage represents a message in an AI chat session
type AIMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // Genkit: user | model | tool | system
	Content   string    `json:"content"`
	Parts     string    `json:"parts"`  // JSON []*genkit Part
	Status    string    `json:"status"` // "" | cancelled | denied
	Timestamp time.Time `json:"timestamp"`
}

// AISessionRepository provides data access operations for AI sessions and messages
type AISessionRepository interface {
	CreateSession(ctx context.Context, session *AISession) error
	GetSession(ctx context.Context, id string) (*AISession, error)
	ListSessions(ctx context.Context, limit int) ([]*AISession, error)
	UpdateSession(ctx context.Context, session *AISession) error
	DeleteSession(ctx context.Context, id string) error
	CreateMessage(ctx context.Context, msg *AIMessage) error
	CreateMessages(ctx context.Context, msgs []*AIMessage) error
	ListMessages(ctx context.Context, sessionID string) ([]*AIMessage, error)
	UpdateMessageStatus(ctx context.Context, id, status string) error
	UpdateMessageStatuses(ctx context.Context, ids []string, status string) error
}

// SQLiteAISessionRepository implements AISessionRepository using SQLite
type SQLiteAISessionRepository struct {
	db *sql.DB
}

// NewSQLiteAISessionRepository creates a new SQLite repository
func NewSQLiteAISessionRepository(db *sql.DB) AISessionRepository {
	return &SQLiteAISessionRepository{db: db}
}

// CreateSession creates a new session
func (r *SQLiteAISessionRepository) CreateSession(ctx context.Context, session *AISession) error {
	session.ID = uuid.New().String()
	now := time.Now()
	session.CreatedAt = now
	session.UpdatedAt = now

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ai_sessions (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		session.ID, session.Title, session.CreatedAt.Unix(), session.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("create session failed: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID
func (r *SQLiteAISessionRepository) GetSession(ctx context.Context, id string) (*AISession, error) {
	var session AISession
	var createdAt, updatedAt int64

	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, created_at, updated_at FROM ai_sessions WHERE id = ?`,
		id).Scan(&session.ID, &session.Title, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get session failed: %w", err)
	}

	session.CreatedAt = time.Unix(createdAt, 0)
	session.UpdatedAt = time.Unix(updatedAt, 0)
	return &session, nil
}

// ListSessions lists sessions ordered by updated_at DESC
func (r *SQLiteAISessionRepository) ListSessions(ctx context.Context, limit int) ([]*AISession, error) {
	query := `SELECT id, title, created_at, updated_at FROM ai_sessions ORDER BY updated_at DESC`
	var (
		rows *sql.Rows
		err  error
	)
	if limit > 0 {
		rows, err = r.db.QueryContext(ctx, query+` LIMIT ?`, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("list sessions failed: %w", err)
	}
	defer rows.Close()

	var sessions []*AISession
	for rows.Next() {
		var session AISession
		var createdAt, updatedAt int64
		if err := rows.Scan(&session.ID, &session.Title, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan session failed: %w", err)
		}
		session.CreatedAt = time.Unix(createdAt, 0)
		session.UpdatedAt = time.Unix(updatedAt, 0)
		sessions = append(sessions, &session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sessions failed: %w", err)
	}

	return sessions, nil
}

// UpdateSession updates a session
func (r *SQLiteAISessionRepository) UpdateSession(ctx context.Context, session *AISession) error {
	session.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx,
		`UPDATE ai_sessions SET title = ?, updated_at = ? WHERE id = ?`,
		session.Title, session.UpdatedAt.Unix(), session.ID)
	if err != nil {
		return fmt.Errorf("update session failed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// DeleteSession deletes a session (CASCADE delete handles messages)
func (r *SQLiteAISessionRepository) DeleteSession(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM ai_sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session failed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// CreateMessage creates a new message
func (r *SQLiteAISessionRepository) CreateMessage(ctx context.Context, msg *AIMessage) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}
	return r.CreateMessages(ctx, []*AIMessage{msg})
}

// CreateMessages inserts messages in one transaction so a tool round is never stored half-written.
func (r *SQLiteAISessionRepository) CreateMessages(ctx context.Context, msgs []*AIMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin message tx failed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if msg.ID == "" {
			msg.ID = uuid.New().String()
		}
		if msg.Timestamp.IsZero() {
			msg.Timestamp = now
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO ai_messages (id, session_id, role, content, parts, status, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			msg.ID, msg.SessionID, msg.Role, msg.Content, msg.Parts, msg.Status, msg.Timestamp.Unix()); err != nil {
			return fmt.Errorf("create message failed: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit messages failed: %w", err)
	}
	return nil
}

// ListMessages lists messages for a session ordered by timestamp ASC
func (r *SQLiteAISessionRepository) ListMessages(ctx context.Context, sessionID string) ([]*AIMessage, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, session_id, role, content, parts, status, timestamp FROM ai_messages WHERE session_id = ? ORDER BY timestamp ASC, rowid ASC`,
		sessionID)
	if err != nil {
		return nil, fmt.Errorf("list messages failed: %w", err)
	}
	defer rows.Close()

	var messages []*AIMessage
	for rows.Next() {
		var msg AIMessage
		var timestamp int64
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.Parts, &msg.Status, &timestamp); err != nil {
			return nil, fmt.Errorf("scan message failed: %w", err)
		}
		msg.Timestamp = time.Unix(timestamp, 0)
		messages = append(messages, &msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list messages failed: %w", err)
	}

	return messages, nil
}

// UpdateMessageStatus sets status for a single message by id.
func (r *SQLiteAISessionRepository) UpdateMessageStatus(ctx context.Context, id, status string) error {
	if id == "" {
		return fmt.Errorf("message id is required")
	}
	return r.UpdateMessageStatuses(ctx, []string{id}, status)
}

// UpdateMessageStatuses sets the same status for multiple message ids.
func (r *SQLiteAISessionRepository) UpdateMessageStatuses(ctx context.Context, ids []string, status string) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin status tx failed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `UPDATE ai_messages SET status = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare status update failed: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, status, id); err != nil {
			return fmt.Errorf("update message status failed: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit status update failed: %w", err)
	}
	return nil
}
