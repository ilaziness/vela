package database

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SSHTunnelDB is one persisted tunnel rule bound to a bookmark.
// tunnel_type is "local" | "remote" | "dynamic"; remote_port is only
// meaningful for "remote" rules.
type SSHTunnelDB struct {
	ID         string
	BookmarkID string
	TunnelType string
	LocalAddr  string
	RemoteAddr string
	RemotePort int
	CreatedAt  string
	UpdatedAt  string
}

// SSHTunnelRepository persists bookmark-scoped tunnel rules.
type SSHTunnelRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewSSHTunnelRepository(db *sql.DB, logger *zap.Logger) *SSHTunnelRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SSHTunnelRepository{db: db, logger: logger}
}

func (r *SSHTunnelRepository) Insert(rec *SSHTunnelDB) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
		INSERT INTO ssh_tunnels (id, bookmark_id, tunnel_type, local_addr, remote_addr, remote_port, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.BookmarkID, rec.TunnelType, rec.LocalAddr, rec.RemoteAddr, rec.RemotePort, now, now)
	if err != nil {
		return fmt.Errorf("insert ssh tunnel: %w", err)
	}
	return nil
}

func (r *SSHTunnelRepository) ListByBookmark(bookmarkID string) ([]SSHTunnelDB, error) {
	rows, err := r.db.Query(`
		SELECT id, bookmark_id, tunnel_type, local_addr, remote_addr, remote_port, created_at, updated_at
		FROM ssh_tunnels WHERE bookmark_id = ? ORDER BY created_at, id`, bookmarkID)
	if err != nil {
		return nil, fmt.Errorf("list ssh tunnels: %w", err)
	}
	defer rows.Close()
	var out []SSHTunnelDB
	for rows.Next() {
		var rec SSHTunnelDB
		if err := rows.Scan(&rec.ID, &rec.BookmarkID, &rec.TunnelType, &rec.LocalAddr,
			&rec.RemoteAddr, &rec.RemotePort, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ssh tunnel: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Delete removes one rule scoped to its bookmark.
func (r *SSHTunnelRepository) Delete(id, bookmarkID string) error {
	_, err := r.db.Exec(`DELETE FROM ssh_tunnels WHERE id = ? AND bookmark_id = ?`, id, bookmarkID)
	if err != nil {
		return fmt.Errorf("delete ssh tunnel: %w", err)
	}
	return nil
}

// DeleteByBookmark removes every rule bound to a bookmark (bookmark deleted).
func (r *SSHTunnelRepository) DeleteByBookmark(bookmarkID string) error {
	_, err := r.db.Exec(`DELETE FROM ssh_tunnels WHERE bookmark_id = ?`, bookmarkID)
	if err != nil {
		return fmt.Errorf("delete ssh tunnels by bookmark: %w", err)
	}
	return nil
}

// DeleteByBookmarkGroup removes every rule bound to bookmarks in a group.
// Call it before the bookmarks are deleted; the subquery reads their rows.
func (r *SSHTunnelRepository) DeleteByBookmarkGroup(groupName string) error {
	query := `DELETE FROM ssh_tunnels
			  WHERE bookmark_id IN (
				  SELECT b.bookmark_id FROM bookmarks b
				  INNER JOIN bookmark_groups g ON b.group_id = g.id
				  WHERE g.name = ?)`
	if _, err := r.db.Exec(query, groupName); err != nil {
		return fmt.Errorf("delete ssh tunnels by group: %w", err)
	}
	return nil
}
