package sshkey

import (
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/secret"
	"github.com/ilaziness/vela/internal/usererr"
	"github.com/ilaziness/vela/internal/utils"
)

// Info 是密钥列表项，不含私钥。
type Info struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Algorithm   string `json:"algorithm"`
	Comment     string `json:"comment"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   string `json:"created_at"`
}

type PasswordFn func(reason string) (string, error)

type Service struct {
	logger    *zap.Logger
	db        *database.Database
	password  PasswordFn
	onBadPass func()
}

func New(logger *zap.Logger, db *database.Database, password PasswordFn, onBadPass func()) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{logger: logger, db: db, password: password, onBadPass: onBadPass}
}

func (s *Service) List() ([]Info, error) {
	rows, err := s.db.SSHKeyRepo.List()
	if err != nil {
		s.logger.Error("list ssh keys failed", zap.Error(err))
		return nil, err
	}
	out := make([]Info, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInfo(row))
	}
	return out, nil
}

func (s *Service) GenerateAndSave(name, algorithm, comment string) (Info, error) {
	pub, privatePEM, err := Generate(algorithm, comment)
	if err != nil {
		return Info{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = pub.Algorithm + " " + time.Now().Format("2006-01-02 15:04")
	}
	encrypted, err := s.encrypt(privatePEM)
	if err != nil {
		return Info{}, err
	}
	row := &database.SSHKeyDB{
		ID:         utils.GenerateRandomID(),
		Name:       name,
		Algorithm:  pub.Algorithm,
		Comment:    pub.Comment,
		PublicKey:  pub.PublicKey,
		PrivateKey: encrypted,
		CreatedAt:  time.Now(),
	}
	if err := s.db.SSHKeyRepo.Insert(row); err != nil {
		s.logger.Error("save ssh key failed", zap.Error(err))
		return Info{}, err
	}
	s.logger.Debug("ssh key generated", zap.String("id", row.ID), zap.String("algorithm", row.Algorithm))
	return toInfo(row), nil
}

// PrivatePEM 解密已保存的私钥，供拨号和导出使用。
func (s *Service) PrivatePEM(id string) (string, error) {
	row, err := s.db.SSHKeyRepo.Get(id)
	if err != nil {
		return "", err
	}
	plain, err := s.decrypt(row.PrivateKey)
	if err != nil {
		s.logger.Error("decrypt ssh key failed", zap.String("id", id), zap.Error(err))
		return "", err
	}
	return plain, nil
}

func (s *Service) Get(id string) (Info, error) {
	row, err := s.db.SSHKeyRepo.Get(id)
	if err != nil {
		return Info{}, err
	}
	return toInfo(row), nil
}

func (s *Service) Delete(id string) error {
	if strings.TrimSpace(id) == "" {
		return usererr.Coded("sshkey.not_found", "SSH key not found")
	}
	count, err := s.db.BookmarkRepo.CountBySSHKeyID(id)
	if err != nil {
		s.logger.Error("count ssh key bookmarks failed", zap.String("id", id), zap.Error(err))
		return err
	}
	if count > 0 {
		return usererr.Coded("sshkey.in_use", "%s bookmark(s) still reference this key; unlink them first", strconv.Itoa(count))
	}
	if err := s.db.SSHKeyRepo.Delete(id); err != nil {
		s.logger.Error("delete ssh key failed", zap.String("id", id), zap.Error(err))
		return err
	}
	return nil
}

func (s *Service) encrypt(plain string) (string, error) {
	password, err := s.masterPassword(i18n.T("password.encrypt_ssh_key"))
	if err != nil {
		return "", err
	}
	encrypted, err := secret.Encrypt(password, plain)
	if err != nil {
		s.logger.Error("encrypt ssh key failed", zap.Error(err))
		return "", usererr.Coded("sshkey.encrypt_failed", "Failed to encrypt private key: %s", err.Error())
	}
	return encrypted, nil
}

func (s *Service) decrypt(encrypted string) (string, error) {
	password, err := s.masterPassword(i18n.T("password.decrypt_ssh_key"))
	if err != nil {
		return "", err
	}
	plain, err := secret.Decrypt(password, encrypted)
	if err != nil {
		if s.onBadPass != nil {
			s.onBadPass()
		}
		return "", usererr.Coded("sshkey.decrypt_failed", "Failed to decrypt private key")
	}
	return plain, nil
}

func (s *Service) masterPassword(reason string) (string, error) {
	if s.password == nil {
		return "", usererr.Coded("error.password_not_entered", "Password not entered")
	}
	password, err := s.password(reason)
	if err != nil {
		return "", err
	}
	if password == "" {
		return "", usererr.Coded("error.password_not_entered", "Password not entered")
	}
	return password, nil
}

func toInfo(row *database.SSHKeyDB) Info {
	fp, err := FingerprintOfAuthorized(row.PublicKey)
	if err != nil {
		fp = ""
	}
	return Info{
		ID:          row.ID,
		Name:        row.Name,
		Algorithm:   row.Algorithm,
		Comment:     row.Comment,
		PublicKey:   row.PublicKey,
		Fingerprint: fp,
		CreatedAt:   row.CreatedAt.Format("2006-01-02 15:04"),
	}
}
