package tools

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"

	"github.com/ilaziness/vela/internal/i18n"
)

// Tool 工具定义
type Tool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
}

// PortCheckResult 端口检测结果
type PortCheckResult struct {
	Success      bool   `json:"success"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ResponseTime int64  `json:"responseTime"` // 毫秒
	Error        string `json:"error,omitempty"`
}

// EncodeResponse 编码响应
type EncodeResponse struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// Match 正则匹配结果
type Match struct {
	Text   string   `json:"text"`
	Index  int      `json:"index"`
	Groups []string `json:"groups"`
}

// RegexMatchResult 正则匹配结果
type RegexMatchResult struct {
	Matches []Match `json:"matches"`
	Count   int     `json:"count"`
	Error   string  `json:"error,omitempty"`
}

// Service 工具箱核心逻辑（无包级可变状态）
type Service struct{}

// New 创建工具服务实例
func New() *Service {
	return &Service{}
}

// GetTools 获取可用工具列表（按常用程度排序）
func (ts *Service) GetTools() []Tool {
	return []Tool{
		{
			ID:          "json-yaml",
			Name:        i18n.T("tool.json_yaml.name"),
			Description: i18n.T("tool.json_yaml.desc"),
			Icon:        "DataObject",
			Category:    "dev",
		},
		{
			ID:          "timestamp",
			Name:        i18n.T("tool.timestamp.name"),
			Description: i18n.T("tool.timestamp.desc"),
			Icon:        "AccessTime",
			Category:    "dev",
		},
		{
			ID:          "encoder",
			Name:        i18n.T("tool.encoder.name"),
			Description: i18n.T("tool.encoder.desc"),
			Icon:        "Code",
			Category:    "encoding",
		},
		{
			ID:          "hash",
			Name:        i18n.T("tool.hash.name"),
			Description: i18n.T("tool.hash.desc"),
			Icon:        "Fingerprint",
			Category:    "security",
		},
		{
			ID:          "regex",
			Name:        i18n.T("tool.regex.name"),
			Description: i18n.T("tool.regex.desc"),
			Icon:        "RegularExpression",
			Category:    "dev",
		},
		{
			ID:          "cron",
			Name:        i18n.T("tool.cron.name"),
			Description: i18n.T("tool.cron.desc"),
			Icon:        "Schedule",
			Category:    "ops",
		},
		{
			ID:          "cidr",
			Name:        i18n.T("tool.cidr.name"),
			Description: i18n.T("tool.cidr.desc"),
			Icon:        "Lan",
			Category:    "network",
		},
		{
			ID:          "port-check",
			Name:        i18n.T("tool.port_check.name"),
			Description: i18n.T("tool.port_check.desc"),
			Icon:        "NetworkCheck",
			Category:    "network",
		},
		{
			ID:          "jwt",
			Name:        i18n.T("tool.jwt.name"),
			Description: i18n.T("tool.jwt.desc"),
			Icon:        "Key",
			Category:    "security",
		},
		{
			ID:          "random",
			Name:        i18n.T("tool.random.name"),
			Description: i18n.T("tool.random.desc"),
			Icon:        "Casino",
			Category:    "dev",
		},
		{
			ID:          "base-convert",
			Name:        i18n.T("tool.base_convert.name"),
			Description: i18n.T("tool.base_convert.desc"),
			Icon:        "Pin",
			Category:    "dev",
		},
		{
			ID:          "text-diff",
			Name:        i18n.T("tool.text_diff.name"),
			Description: i18n.T("tool.text_diff.desc"),
			Icon:        "Compare",
			Category:    "dev",
		},
		{
			ID:          "chmod",
			Name:        i18n.T("tool.chmod.name"),
			Description: i18n.T("tool.chmod.desc"),
			Icon:        "Lock",
			Category:    "ops",
		},
	}
}

// CheckPort 检测端口连通性
func (ts *Service) CheckPort(host string, port int) PortCheckResult {
	if host == "" {
		return PortCheckResult{
			Success: false,
			Host:    host,
			Port:    port,
			Error:   i18n.T("tool.err.host_empty"),
		}
	}
	if port < 1 || port > 65535 {
		return PortCheckResult{
			Success: false,
			Host:    host,
			Port:    port,
			Error:   i18n.T("tool.err.port_range"),
		}
	}

	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return PortCheckResult{
			Success:      false,
			Host:         host,
			Port:         port,
			ResponseTime: elapsed,
			Error:        err.Error(),
		}
	}
	defer conn.Close()

	return PortCheckResult{
		Success:      true,
		Host:         host,
		Port:         port,
		ResponseTime: elapsed,
	}
}

// Encode 编码
func (ts *Service) Encode(toolType string, input string) EncodeResponse {
	var result string

	switch toolType {
	case "base64":
		result = base64.StdEncoding.EncodeToString([]byte(input))
	case "url":
		result = url.QueryEscape(input)
	case "html":
		result = html.EscapeString(input)
	default:
		return EncodeResponse{
			Error: i18n.Tf("tool.err.unsupported_encode", toolType),
		}
	}

	return EncodeResponse{Result: result}
}

// Decode 解码
func (ts *Service) Decode(toolType string, input string) EncodeResponse {
	var result string
	var err error

	switch toolType {
	case "base64":
		var decoded []byte
		decoded, err = base64.StdEncoding.DecodeString(input)
		if err != nil {
			return EncodeResponse{Error: i18n.Tf("tool.err.base64_decode", err.Error())}
		}
		result = string(decoded)
	case "url":
		result, err = url.QueryUnescape(input)
		if err != nil {
			return EncodeResponse{Error: i18n.Tf("tool.err.url_decode", err.Error())}
		}
	case "html":
		result = html.UnescapeString(input)
	default:
		return EncodeResponse{
			Error: i18n.Tf("tool.err.unsupported_decode", toolType),
		}
	}

	return EncodeResponse{Result: result}
}

// HashResult 哈希计算结果
type HashResult struct {
	Success   bool   `json:"success"`
	Input     string `json:"input"`
	Algorithm string `json:"algorithm"` // 哈希算法
	Result    string `json:"result"`    // 哈希值
	Error     string `json:"error,omitempty"`
}

// TimestampResult 时间戳转换结果
type TimestampResult struct {
	Success   bool   `json:"success"`
	Timestamp int64  `json:"timestamp,omitempty"` // Unix时间戳（秒）
	Datetime  string `json:"datetime,omitempty"`  // 日期时间字符串
	Error     string `json:"error,omitempty"`
}

// RegexMatch 正则匹配
func (ts *Service) RegexMatch(pattern string, text string, flags string) RegexMatchResult {
	if pattern == "" {
		return RegexMatchResult{Matches: []Match{}, Count: 0}
	}

	// 构建正则标志
	flagMap := map[rune]bool{
		'i': false, // 忽略大小写
		'm': false, // 多行模式
		'g': false, // 全局匹配
	}
	for _, f := range flags {
		if _, ok := flagMap[f]; ok {
			flagMap[f] = true
		}
	}

	// 构建 Go 正则标志
	goFlags := ""
	if flagMap['i'] {
		goFlags += "(?i)"
	}
	if flagMap['m'] {
		goFlags += "(?m)"
	}

	// 编译正则表达式
	fullPattern := goFlags + pattern
	re, err := regexp.Compile(fullPattern)
	if err != nil {
		return RegexMatchResult{
			Error: i18n.Tf("tool.err.regex_compile", err.Error()),
		}
	}

	// 查找所有匹配
	matches := re.FindAllStringSubmatchIndex(text, -1)
	if matches == nil {
		return RegexMatchResult{Matches: []Match{}, Count: 0}
	}

	result := make([]Match, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			start, end := match[0], match[1]
			matchedText := text[start:end]

			// 提取捕获组
			groups := make([]string, 0)
			for i := 2; i < len(match); i += 2 {
				if i+1 < len(match) {
					groupStart, groupEnd := match[i], match[i+1]
					if groupStart >= 0 && groupEnd >= 0 {
						groups = append(groups, text[groupStart:groupEnd])
					} else {
						groups = append(groups, "")
					}
				}
			}

			result = append(result, Match{
				Text:   matchedText,
				Index:  start,
				Groups: groups,
			})
		}
	}

	return RegexMatchResult{
		Matches: result,
		Count:   len(result),
	}
}

// CalculateHash 计算哈希值
func (ts *Service) CalculateHash(input string, algorithm string) HashResult {
	if input == "" {
		return HashResult{
			Success: false,
			Input:   input,
			Error:   i18n.T("tool.err.input_empty"),
		}
	}
	if algorithm == "" {
		algorithm = "md5"
	}

	var result string

	switch strings.ToLower(algorithm) {
	case "md5":
		hash := md5.Sum([]byte(input))
		result = hex.EncodeToString(hash[:])
	case "sha1":
		hash := sha1.Sum([]byte(input))
		result = hex.EncodeToString(hash[:])
	case "sha256":
		hash := sha256.Sum256([]byte(input))
		result = hex.EncodeToString(hash[:])
	case "sha512":
		hash := sha512.Sum512([]byte(input))
		result = hex.EncodeToString(hash[:])
	default:
		return HashResult{
			Success: false,
			Input:   input,
			Error:   i18n.Tf("tool.err.unsupported_hash", algorithm),
		}
	}

	return HashResult{
		Success:   true,
		Input:     input,
		Algorithm: strings.ToLower(algorithm),
		Result:    result,
	}
}

// ConvertTimestamp 转换时间戳
func (ts *Service) ConvertTimestamp(input string, toTimestamp bool) TimestampResult {
	if input == "" {
		return TimestampResult{
			Success: false,
			Error:   i18n.T("tool.err.input_empty"),
		}
	}

	if toTimestamp {
		// datetime-local 等本地时间格式用 ParseInLocation；带时区的用 Parse
		localLayouts := []string{
			"2006-01-02T15:04:05",
			"2006-01-02T15:04",
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
			"2006-01-02",
		}
		zonedLayouts := []string{
			time.RFC3339,
			time.RFC1123,
			time.RFC822,
			time.ANSIC,
			time.UnixDate,
			time.RubyDate,
		}

		var t time.Time
		var err error
		parsed := false
		for _, layout := range localLayouts {
			t, err = time.ParseInLocation(layout, input, time.Local)
			if err == nil {
				parsed = true
				break
			}
		}
		if !parsed {
			for _, layout := range zonedLayouts {
				t, err = time.Parse(layout, input)
				if err == nil {
					parsed = true
					break
				}
			}
		}
		if !parsed {
			return TimestampResult{
				Success: false,
				Error:   i18n.T("tool.err.datetime_parse"),
			}
		}

		return TimestampResult{
			Success:   true,
			Timestamp: t.Unix(),
		}
	}

	timestamp, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64)
	if err != nil {
		return TimestampResult{
			Success: false,
			Error:   i18n.Tf("tool.err.invalid_timestamp", err.Error()),
		}
	}

	// 13 位左右按毫秒处理（常见 JS / 日志时间戳）
	sec, nsec := timestamp, int64(0)
	if timestamp > 1_000_000_000_000 { // > ~2001-09 的毫秒量级
		sec = timestamp / 1000
		nsec = (timestamp % 1000) * int64(time.Millisecond)
	}

	t := time.Unix(sec, nsec)
	return TimestampResult{
		Success:  true,
		Datetime: t.In(time.Local).Format("2006-01-02 15:04:05"),
	}
}
