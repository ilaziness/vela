package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

const sshKeySelectCols = `id, name, algorithm, comment, public_key, private_key, created_at`

// SSHKeyDB 应用内 SSH 密钥。PrivateKey 是主密码加密后的 PEM。
type SSHKeyDB struct {
	ID         string
	Name       string
	Algorithm  string
	Comment    string
	PublicKey  string
	PrivateKey string
	CreatedAt  time.Time
}

type SSHKeyRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewSSHKeyRepository(db *sql.DB, logger *zap.Logger) *SSHKeyRepository {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SSHKeyRepository{db: db, logger: logger}
}

func scanSSHKey(scanner interface{ Scan(dest ...any) error }) (*SSHKeyDB, error) {
	var k SSHKeyDB
	if err := scanner.Scan(&k.ID, &k.Name, &k.Algorithm, &k.Comment, &k.PublicKey, &k.PrivateKey, &k.CreatedAt); err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *SSHKeyRepository) List() ([]*SSHKeyDB, error) {
	rows, err := r.db.Query(`SELECT ` + sshKeySelectCols + ` FROM ssh_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf(errQuery, "ssh keys", err)
	}
	defer rows.Close()

	keys := make([]*SSHKeyDB, 0)
	for rows.Next() {
		k, err := scanSSHKey(rows)
		if err != nil {
			r.logger.Error("scan ssh key failed", zap.Error(err))
			continue
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(errQuery, "ssh keys", err)
	}
	return keys, nil
}

func (r *SSHKeyRepository) Get(id string) (*SSHKeyDB, error) {
	k, err := scanSSHKey(r.db.QueryRow(`SELECT `+sshKeySelectCols+` FROM ssh_keys WHERE id = ?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf(errQuery, "ssh key", err)
	}
	return k, nil
}

func (r *SSHKeyRepository) Insert(key *SSHKeyDB) error {
	_, err := r.db.Exec(
		`INSERT INTO ssh_keys (id, name, algorithm, comment, public_key, private_key, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.Name, key.Algorithm, key.Comment, key.PublicKey, key.PrivateKey, key.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf(errInsertQuery, "ssh key", err)
	}
	r.logger.Debug("ssh key inserted", zap.String("id", key.ID), zap.String("algorithm", key.Algorithm))
	return nil
}

func (r *SSHKeyRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM ssh_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf(errDeleteQuery, "ssh key", err)
	}
	r.logger.Debug("ssh key deleted", zap.String("id", id))
	return nil
}
