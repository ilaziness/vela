package sshkey

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ilaziness/vela/internal/usererr"
)

const (
	AlgoEd25519   = "ed25519"
	AlgoEcdsaP256 = "ecdsa-p256"
	AlgoRsa4096   = "rsa-4096"
)

// PublicResult 是可以交给前端的公钥信息，不含私钥。
type PublicResult struct {
	Algorithm   string `json:"algorithm"`
	Comment     string `json:"comment"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
}

// Generate 生成 OpenSSH 密钥对。privatePEM 为未加口令的 OpenSSH 私钥。
func Generate(algorithm, comment string) (PublicResult, string, error) {
	algorithm = strings.TrimSpace(algorithm)
	comment = sanitizeComment(comment)
	key, err := newPrivateKey(algorithm)
	if err != nil {
		return PublicResult{}, "", err
	}
	signer, err := cryptossh.NewSignerFromKey(key)
	if err != nil {
		return PublicResult{}, "", fmt.Errorf("create signer failed: %w", err)
	}
	block, err := cryptossh.MarshalPrivateKey(key, comment)
	if err != nil {
		return PublicResult{}, "", fmt.Errorf("encode private key failed: %w", err)
	}
	pub := PublicResult{
		Algorithm:   algorithm,
		Comment:     comment,
		PublicKey:   authorizedLine(signer.PublicKey(), comment),
		Fingerprint: cryptossh.FingerprintSHA256(signer.PublicKey()),
	}
	return pub, string(pem.EncodeToMemory(block)), nil
}

func newPrivateKey(algorithm string) (any, error) {
	switch algorithm {
	case AlgoEd25519:
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ed25519 key failed: %w", err)
		}
		return priv, nil
	case AlgoEcdsaP256:
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ecdsa key failed: %w", err)
		}
		return priv, nil
	case AlgoRsa4096:
		priv, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, fmt.Errorf("generate rsa key failed: %w", err)
		}
		return priv, nil
	default:
		return nil, usererr.Coded("sshkey.unsupported_algo", "Unsupported key algorithm")
	}
}

func authorizedLine(pub cryptossh.PublicKey, comment string) string {
	line := strings.TrimSpace(string(cryptossh.MarshalAuthorizedKey(pub)))
	if comment != "" {
		line += " " + comment
	}
	return line + "\n"
}

func sanitizeComment(comment string) string {
	comment = strings.TrimSpace(comment)
	comment = strings.ReplaceAll(comment, "\r", " ")
	comment = strings.ReplaceAll(comment, "\n", " ")
	return strings.Join(strings.Fields(comment), " ")
}

// FingerprintOfAuthorized 从 authorized_keys 行计算 SHA256 指纹。
func FingerprintOfAuthorized(line string) (string, error) {
	pub, _, _, _, err := cryptossh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return "", fmt.Errorf("parse public key failed: %w", err)
	}
	return cryptossh.FingerprintSHA256(pub), nil
}

// DefaultFileName 是导出私钥时的建议文件名。
func DefaultFileName(algorithm string) string {
	switch algorithm {
	case AlgoEd25519:
		return "id_ed25519"
	case AlgoEcdsaP256:
		return "id_ecdsa"
	case AlgoRsa4096:
		return "id_rsa"
	default:
		return "id_ssh"
	}
}

// WritePrivateKeyFile 把 OpenSSH 私钥写到 path。passphrase 为空时原样写入，否则重新加密。
func WritePrivateKeyFile(path, privatePEM, comment, passphrase string) error {
	data := []byte(privatePEM)
	if passphrase != "" {
		raw, err := cryptossh.ParseRawPrivateKey(data)
		if err != nil {
			return fmt.Errorf("parse private key failed: %w", err)
		}
		block, err := cryptossh.MarshalPrivateKeyWithPassphrase(raw, sanitizeComment(comment), []byte(passphrase))
		if err != nil {
			return fmt.Errorf("encrypt private key failed: %w", err)
		}
		data = pem.EncodeToMemory(block)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write private key failed: %w", err)
	}
	return nil
}
