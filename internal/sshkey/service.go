package sshkey

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/secret"
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
		return fmt.Errorf("密钥不存在")
	}
	count, err := s.db.BookmarkRepo.CountBySSHKeyID(id)
	if err != nil {
		s.logger.Error("count ssh key bookmarks failed", zap.String("id", id), zap.Error(err))
		return err
	}
	if count > 0 {
		return fmt.Errorf("有 %d 个书签正在使用此密钥，请先解除引用", count)
	}
	if err := s.db.SSHKeyRepo.Delete(id); err != nil {
		s.logger.Error("delete ssh key failed", zap.String("id", id), zap.Error(err))
		return err
	}
	return nil
}

func (s *Service) encrypt(plain string) (string, error) {
	password, err := s.masterPassword("需要密码来加密 SSH 私钥")
	if err != nil {
		return "", err
	}
	encrypted, err := secret.Encrypt(password, plain)
	if err != nil {
		s.logger.Error("encrypt ssh key failed", zap.Error(err))
		return "", fmt.Errorf("加密私钥失败: %w", err)
	}
	return encrypted, nil
}

func (s *Service) decrypt(encrypted string) (string, error) {
	password, err := s.masterPassword("需要密码来解密 SSH 私钥")
	if err != nil {
		return "", err
	}
	plain, err := secret.Decrypt(password, encrypted)
	if err != nil {
		if s.onBadPass != nil {
			s.onBadPass()
		}
		return "", fmt.Errorf("解密私钥失败")
	}
	return plain, nil
}

func (s *Service) masterPassword(reason string) (string, error) {
	if s.password == nil {
		return "", fmt.Errorf("password not entered")
	}
	password, err := s.password(reason)
	if err != nil {
		return "", err
	}
	if password == "" {
		return "", fmt.Errorf("password not entered")
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
