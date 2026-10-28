package tools

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"uuid"

	"github.com/ilaziness/vela/internal/i18n"
)

// RandomResult 随机生成结果
type RandomResult struct {
	Success bool   `json:"success"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

const (
	charsetAlphaNum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charsetAlpha    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	charsetNumeric  = "0123456789"
	charsetHex      = "0123456789abcdef"
	charsetBase64   = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
)

// GenerateRandom 生成 UUID 或随机串
// kind: uuid-v4 | uuid-v7 | string
// charset: alphanum | alpha | numeric | hex | base64 | custom（customCharset 生效）
func (ts *Service) GenerateRandom(kind string, length int, charset string, customCharset string) RandomResult {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "uuid-v4":
		return RandomResult{Success: true, Result: uuid.NewV4().String()}
	case "uuid-v7":
		return RandomResult{Success: true, Result: uuid.NewV7().String()}
	case "string":
		return generateRandomString(length, charset, customCharset)
	default:
		return RandomResult{Success: false, Error: i18n.Tf("tool.err.unsupported_kind", kind)}
	}
}

func generateRandomString(length int, charsetName string, customCharset string) RandomResult {
	if length <= 0 {
		length = 16
	}
	if length > 1024 {
		return RandomResult{Success: false, Error: i18n.T("tool.err.length_max")}
	}

	charset, err := resolveCharset(charsetName, customCharset)
	if err != nil {
		return RandomResult{Success: false, Error: err.Error()}
	}

	var b strings.Builder
	b.Grow(length)
	charsetLen := big.NewInt(int64(len(charset)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return RandomResult{Success: false, Error: i18n.Tf("tool.err.random_failed", err.Error())}
		}
		b.WriteByte(charset[n.Int64()])
	}
	return RandomResult{Success: true, Result: b.String()}
}

func resolveCharset(name string, custom string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "alphanum", "":
		return charsetAlphaNum, nil
	case "alpha":
		return charsetAlpha, nil
	case "numeric":
		return charsetNumeric, nil
	case "hex":
		return charsetHex, nil
	case "base64":
		return charsetBase64, nil
	case "custom":
		if custom == "" {
			return "", errors.New("custom charset cannot be empty")
		}
		return custom, nil
	default:
		return "", errors.New("unsupported charset: " + name)
	}
}
