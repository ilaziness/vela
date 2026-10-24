package services

import (
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/sshkey"
)

type SSHKeyInfo = sshkey.Info
type SSHKeyPublic = sshkey.PublicResult

type KeyService struct {
	app    *application.App
	core   *sshkey.Service
	logger *zap.Logger
}

func NewKeyService(app *application.App, core *sshkey.Service, logger *zap.Logger) *KeyService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &KeyService{app: app, core: core, logger: logger}
}

func (s *KeyService) List() ([]SSHKeyInfo, error) {
	return s.core.List()
}

func (s *KeyService) GenerateAndSave(name, algorithm, comment string) (SSHKeyInfo, error) {
	return s.core.GenerateAndSave(name, algorithm, comment)
}

func (s *KeyService) Delete(id string) error {
	return s.core.Delete(id)
}

// Export 把已保存的私钥写到用户选择的路径。取消对话框时 written 为 false。
func (s *KeyService) Export(id, passphrase string) (bool, error) {
	info, err := s.core.Get(id)
	if err != nil {
		return false, err
	}
	pem, err := s.core.PrivatePEM(id)
	if err != nil {
		return false, err
	}
	path, ok, err := s.pickPath(sshkey.DefaultFileName(info.Algorithm))
	if err != nil || !ok {
		return false, err
	}
	if err := sshkey.WritePrivateKeyFile(path, pem, info.Comment, passphrase); err != nil {
		s.logger.Error("export ssh key failed", zap.String("id", id), zap.Error(err))
		return false, err
	}
	return true, nil
}

// GenerateAndExport 生成密钥并只写入用户选择的文件，不入库。取消对话框时 written 为 false。
func (s *KeyService) GenerateAndExport(algorithm, comment, passphrase string) (SSHKeyPublic, bool, error) {
	path, ok, err := s.pickPath(sshkey.DefaultFileName(strings.TrimSpace(algorithm)))
	if err != nil || !ok {
		return SSHKeyPublic{}, false, err
	}
	pub, privatePEM, err := sshkey.Generate(algorithm, comment)
	if err != nil {
		return SSHKeyPublic{}, false, err
	}
	if err := sshkey.WritePrivateKeyFile(path, privatePEM, pub.Comment, passphrase); err != nil {
		s.logger.Error("export generated ssh key failed", zap.Error(err))
		return SSHKeyPublic{}, false, err
	}
	return pub, true, nil
}

func (s *KeyService) pickPath(filename string) (string, bool, error) {
	path, err := promptSaveFile(s.app, "导出私钥", filename)
	if dialogCancelled(err) || path == "" {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}
