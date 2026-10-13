package database

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/transfer"
)

// SftpTransferRepository persists unfinished SFTP transfers.
type SftpTransferRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewSftpTransferRepository(db *sql.DB, logger *zap.Logger) *SftpTransferRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SftpTransferRepository{db: db, logger: logger}
}

func (r *SftpTransferRepository) Upsert(rec transfer.QueueRecord) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
		INSERT INTO sftp_transfers (id, owner_key, transfer_type, local_file, remote_file, total_size, status, error, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			owner_key=excluded.owner_key,
			transfer_type=excluded.transfer_type,
			local_file=excluded.local_file,
			remote_file=excluded.remote_file,
			total_size=excluded.total_size,
			status=excluded.status,
			error=excluded.error,
			updated_at=excluded.updated_at
	`, rec.ID, rec.OwnerKey, rec.TransferType, rec.LocalFile, rec.RemoteFile, rec.TotalSize, rec.Status, rec.Error, now)
	if err != nil {
		return fmt.Errorf("upsert sftp transfer: %w", err)
	}
	return nil
}

func (r *SftpTransferRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM sftp_transfers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete sftp transfer: %w", err)
	}
	return nil
}

func (r *SftpTransferRepository) Get(id string) (*transfer.QueueRecord, error) {
	row := r.db.QueryRow(`
		SELECT id, owner_key, transfer_type, local_file, remote_file, total_size, status, error
		FROM sftp_transfers WHERE id = ?`, id)
	var rec transfer.QueueRecord
	err := row.Scan(&rec.ID, &rec.OwnerKey, &rec.TransferType, &rec.LocalFile, &rec.RemoteFile, &rec.TotalSize, &rec.Status, &rec.Error)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sftp transfer: %w", err)
	}
	return &rec, nil
}

func (r *SftpTransferRepository) ListByOwner(ownerKey string) ([]transfer.QueueRecord, error) {
	rows, err := r.db.Query(`
		SELECT id, owner_key, transfer_type, local_file, remote_file, total_size, status, error
		FROM sftp_transfers WHERE owner_key = ? ORDER BY updated_at DESC`, ownerKey)
	if err != nil {
		return nil, fmt.Errorf("list sftp transfers: %w", err)
	}
	defer rows.Close()
	var out []transfer.QueueRecord
	for rows.Next() {
		var rec transfer.QueueRecord
		if err := rows.Scan(&rec.ID, &rec.OwnerKey, &rec.TransferType, &rec.LocalFile, &rec.RemoteFile, &rec.TotalSize, &rec.Status, &rec.Error); err != nil {
			return nil, fmt.Errorf("scan sftp transfer: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *SftpTransferRepository) SetStatus(id, status, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`UPDATE sftp_transfers SET status = ?, error = ?, updated_at = ? WHERE id = ?`,
		status, errMsg, now, id)
	if err != nil {
		return fmt.Errorf("set sftp transfer status: %w", err)
	}
	return nil
}
