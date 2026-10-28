package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/ilaziness/vela/internal/i18n"
)

// JWTResult JWT 解码结果
type JWTResult struct {
	Success   bool   `json:"success"`
	Header    string `json:"header,omitempty"`
	Payload   string `json:"payload,omitempty"`
	Signature string `json:"signature,omitempty"`
	Error     string `json:"error,omitempty"`
}

const bearerPrefix = "bearer "

// DecodeJWT 本地拆分 JWT，不校验签名
func (ts *Service) DecodeJWT(token string) JWTResult {
	token = strings.TrimSpace(token)
	if token == "" {
		return JWTResult{Success: false, Error: i18n.T("tool.err.jwt_empty")}
	}

	if len(token) >= len(bearerPrefix) && strings.EqualFold(token[:len(bearerPrefix)], bearerPrefix) {
		token = strings.TrimSpace(token[len(bearerPrefix):])
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return JWTResult{Success: false, Error: i18n.T("tool.err.invalid_jwt_parts")}
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return JWTResult{Success: false, Error: i18n.T("tool.err.jwt_part_empty")}
	}

	header, err := decodeJWTPart(parts[0])
	if err != nil {
		return JWTResult{Success: false, Error: i18n.Tf("tool.err.jwt_header", err.Error())}
	}
	payload, err := decodeJWTPart(parts[1])
	if err != nil {
		return JWTResult{Success: false, Error: i18n.Tf("tool.err.jwt_payload", err.Error())}
	}

	return JWTResult{
		Success:   true,
		Header:    header,
		Payload:   payload,
		Signature: parts[2],
	}
}

func decodeJWTPart(part string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(part)
		if err != nil {
			return "", err
		}
	}
	var buf bytes.Buffer
	if !json.Valid(raw) {
		return string(raw), nil
	}
	_ = json.Indent(&buf, raw, "", "  ")
	if buf.Len() == 0 {
		return string(raw), nil
	}
	return buf.String(), nil
}
