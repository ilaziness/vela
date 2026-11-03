package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// net/http-style status in errors looks like ": 503 Service Unavailable" (space after colon).
// That avoids matching URL ports such as ":443".
var httpResponseStatus = regexp.MustCompile(`:\s([45]\d{2})\b`)

// PublicGenerateError is a short message for the chat UI. Full detail belongs in logs.
func PublicGenerateError(err error) string {
	if err == nil {
		return "generate failed"
	}
	if isToolArgsParseError(err) {
		return "工具参数 JSON 无效（命令过长或引号未转义），请改用更短的单条命令"
	}
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "AI chat failed: ")
	msg = strings.TrimPrefix(msg, "generate error: ")
	msg = strings.TrimPrefix(msg, "stream error: ")
	if api := formatAPIError(msg); api != "" {
		return api
	}
	if strings.Contains(strings.ToLower(msg), "unexpected end of json") {
		return "生成失败：模型输出无法解析"
	}
	return truncPublic(msg, 240)
}

// formatAPIError turns provider HTTP failures into a short user-facing API error.
func formatAPIError(msg string) string {
	status := httpStatusFromError(msg)
	detail := providerErrorDetail(msg)
	if status == "" {
		return ""
	}
	if detail != "" {
		return fmt.Sprintf("API 错误 (%s): %s", status, detail)
	}
	if i := strings.LastIndex(msg, ": "); i >= 0 && i+2 < len(msg) {
		tail := strings.TrimSpace(msg[i+2:])
		if strings.HasPrefix(tail, status) {
			if j := strings.Index(tail, "{"); j > 0 {
				tail = strings.TrimSpace(tail[:j])
			}
			if tail != "" {
				return "API 错误: " + truncPublic(tail, 200)
			}
		}
	}
	return fmt.Sprintf("API 错误 (%s)", status)
}

func httpStatusFromError(msg string) string {
	cut := msg
	if i := strings.Index(msg, "{"); i >= 0 {
		cut = msg[:i]
	}
	matches := httpResponseStatus.FindAllStringSubmatch(cut, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1][1]
}

func providerErrorDetail(msg string) string {
	i := strings.Index(msg, "{")
	if i < 0 {
		return ""
	}
	var body struct {
		Message string `json:"message"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(msg[i:]), &body); err != nil {
		return ""
	}
	if m := strings.TrimSpace(body.Message); m != "" {
		return truncPublic(m, 200)
	}
	if body.Error != nil {
		if m := strings.TrimSpace(body.Error.Message); m != "" {
			return truncPublic(m, 200)
		}
	}
	return ""
}

func truncPublic(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
