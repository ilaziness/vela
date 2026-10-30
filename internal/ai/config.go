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

// buildGenConfig 构建各供应商的原生配置结构
func (c *Config) buildGenConfig() any {
	switch c.Provider {
	case ProviderOllama:
		cfg := &ollama.GenerateContentConfig{
			Think:       ollama.ThinkEnabled(true),
			Temperature: &defaultTemperature,
		}
		if strings.Contains(c.Model, "GPT") {
			cfg.Think = ollama.ThinkEffort("medium")
		}
		if c.Temperature > 0 {
			cfg.Temperature = new(c.Temperature)
		}
		if c.MaxTokens > 0 {
			cfg.NumPredict = new(c.MaxTokens)
		}
		return cfg

	case ProviderOpenAI, ProviderOpenAICompatible:
		cfg := &openai.ChatCompletionNewParams{
			Temperature:     openai.Float(defaultTemperature),
			ReasoningEffort: openai.ReasoningEffortMedium,
		}
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
			ThinkingConfig: &genai.ThinkingConfig{
				IncludeThoughts: true,
			},
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
