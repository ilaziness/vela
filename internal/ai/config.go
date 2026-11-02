package ai

import (
	"strings"

	"github.com/firebase/genkit/go/plugins/ollama"
	"github.com/openai/openai-go"
	"google.golang.org/genai"

	"github.com/ilaziness/vela/internal/ai/tool"
)

var (
	defaultTemperature = 0.5
)

const (
	// DefaultMaxAgentTurns 单次请求中模型↔工具交互的最大轮次，超出后终止生成。
	DefaultMaxAgentTurns = 50
)

// FileSystemConfig gates local file tools via an allowlist.
type FileSystemConfig struct {
	AllowedDirs []string `json:"allowed_dirs" toml:"allowed_dirs"`
}

// WebSearchConfig stores the selected search provider and encrypted API key.
type WebSearchConfig struct {
	Provider        tool.SearchProvider `json:"provider" toml:"provider"`
	APIKey          string              `json:"api_key,omitempty" toml:"api_key,omitempty"`
	APIKeyEncrypted string              `json:"api_key_encrypted,omitempty" toml:"api_key_encrypted,omitempty"`
}

// Config AI配置（配置文件与引擎共用）
type Config struct {
	Enabled         bool             `json:"enabled" toml:"enabled"`
	Provider        Provider         `json:"provider" toml:"provider"`
	Model           string           `json:"model" toml:"model"`
	APIKey          string           `json:"api_key,omitempty" toml:"api_key,omitempty"`
	APIKeyEncrypted string           `json:"api_key_encrypted,omitempty" toml:"api_key_encrypted,omitempty"`
	Endpoint        string           `json:"endpoint" toml:"endpoint"`
	Temperature     float64          `json:"temperature" toml:"temperature"`
	MaxTokens       int              `json:"max_tokens" toml:"max_tokens"`
	EchoSSHCommands bool             `json:"echo_ssh_commands" toml:"echo_ssh_commands"`
	MaxAgentTurns   int              `json:"max_agent_turns" toml:"max_agent_turns"`
	Filesystem      FileSystemConfig `json:"filesystem" toml:"filesystem"`
	WebSearch       WebSearchConfig  `json:"web_search" toml:"web_search"`
}

// EffectiveMaxAgentTurns 返回生效轮次：未配置或低于默认时用 DefaultMaxAgentTurns，允许配置更大值。
func (c *Config) EffectiveMaxAgentTurns() int {
	if c == nil || c.MaxAgentTurns < DefaultMaxAgentTurns {
		return DefaultMaxAgentTurns
	}
	return c.MaxAgentTurns
}

// EffectiveAllowedDirs returns existing absolute allowlist directories only.
func (c *Config) EffectiveAllowedDirs() []string {
	if c == nil {
		return nil
	}
	return tool.SanitizeAllowedDirs(c.Filesystem.AllowedDirs)
}

// isOpenAIReasoningModel reports models that use the Responses/completions
// reasoning API shape (o-series / gpt-5), where MaxTokens is rejected.
func isOpenAIReasoningModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	m = strings.TrimPrefix(m, "openai/")
	switch {
	case strings.HasPrefix(m, "o1"),
		strings.HasPrefix(m, "o3"),
		strings.HasPrefix(m, "o4"),
		strings.HasPrefix(m, "gpt-5"):
		return true
	default:
		return false
	}
}

// ollamaThinkOption returns a Think setting only for models that support it.
// Non-thinking models (e.g. llama3.2) reject think:true with HTTP 400.
func ollamaThinkOption(model string) *ollama.ThinkOption {
	m := strings.ToLower(strings.TrimSpace(model))
	base, _, _ := strings.Cut(m, ":")
	switch {
	case strings.Contains(base, "gpt-oss"):
		return ollama.ThinkEffort("medium")
	case strings.Contains(base, "deepseek-r1"),
		strings.HasPrefix(base, "qwq"),
		strings.HasPrefix(base, "magistral"):
		return ollama.ThinkEnabled(true)
	default:
		return nil
	}
}

// isGoogleThinkingModel reports Gemini models that accept ThinkingConfig.
// Older models (1.5 / 2.0 / flash-lite class) reject includeThoughts.
func isGoogleThinkingModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	m = strings.TrimPrefix(m, "googleai/")
	m = strings.TrimPrefix(m, "google/")
	switch {
	case strings.Contains(m, "gemini-2.5"),
		strings.Contains(m, "gemini-3"),
		strings.Contains(m, "thinking"):
		return true
	default:
		return false
	}
}

// buildGenConfig 构建各供应商的原生配置结构
func (c *Config) buildGenConfig() any {
	switch c.Provider {
	case ProviderOllama:
		cfg := &ollama.GenerateContentConfig{
			Think:       ollamaThinkOption(c.Model),
			Temperature: &defaultTemperature,
		}
		if c.Temperature > 0 {
			cfg.Temperature = new(c.Temperature)
		}
		if c.MaxTokens > 0 {
			cfg.NumPredict = new(c.MaxTokens)
		}
		return cfg

	case ProviderOpenAI, ProviderOpenAICompatible:
		cfg := &openai.ChatCompletionNewParams{}
		// o-series / gpt-5 reject temperature and max_tokens; use MaxCompletionTokens.
		// ReasoningEffort is official OpenAI only — many openai_compatible gateways reject it.
		if isOpenAIReasoningModel(c.Model) {
			if c.Provider == ProviderOpenAI {
				cfg.ReasoningEffort = openai.ReasoningEffortMedium
			}
			if c.MaxTokens > 0 {
				cfg.MaxCompletionTokens = openai.Int(int64(c.MaxTokens))
			}
			return cfg
		}
		cfg.Temperature = openai.Float(defaultTemperature)
		if c.Temperature > 0 {
			cfg.Temperature = openai.Float(c.Temperature)
		}
		if c.MaxTokens > 0 {
			cfg.MaxTokens = openai.Int(int64(c.MaxTokens))
		}
		return cfg
	case ProviderGoogle:
		cfg := &genai.GenerateContentConfig{
			Temperature: genai.Ptr(float32(0.5)),
		}
		if isGoogleThinkingModel(c.Model) {
			cfg.ThinkingConfig = &genai.ThinkingConfig{
				IncludeThoughts: true,
			}
		}
		if c.Temperature > 0 {
			cfg.Temperature = genai.Ptr(float32(c.Temperature))
		}
		if c.MaxTokens > 0 {
			cfg.MaxOutputTokens = int32(c.MaxTokens)
		}
		return cfg
	}
	return nil
}
