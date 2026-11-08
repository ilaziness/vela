package ai

import (
	"testing"

	"github.com/firebase/genkit/go/plugins/ollama"
	"github.com/openai/openai-go"
	"google.golang.org/genai"
)

func TestIsOpenAIReasoningModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model string
		want  bool
	}{
		{"gpt-4o", false},
		{"o1", true},
		{"o1-mini", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"gpt-5", true},
		{"openai/o3-mini", true},
		{"O1-PREVIEW", true},
	}
	for _, tc := range cases {
		if got := isOpenAIReasoningModel(tc.model); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.model, got, tc.want)
		}
	}
}

func TestBuildGenConfigOpenAIReasoningUsesMaxCompletionTokens(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Provider:    ProviderOpenAI,
		Model:       "o3-mini",
		Temperature: 0.7,
		MaxTokens:   1024,
	}
	got, ok := cfg.buildGenConfig().(*openai.ChatCompletionNewParams)
	if !ok || got == nil {
		t.Fatalf("unexpected type %T", cfg.buildGenConfig())
	}
	if got.ReasoningEffort != openai.ReasoningEffortMedium {
		t.Fatalf("ReasoningEffort=%#v", got.ReasoningEffort)
	}
	if !got.MaxCompletionTokens.Valid() || got.MaxCompletionTokens.Or(0) != 1024 {
		t.Fatalf("MaxCompletionTokens=%#v", got.MaxCompletionTokens)
	}
	if got.MaxTokens.Valid() {
		t.Fatalf("MaxTokens should be unset, got %#v", got.MaxTokens)
	}
	if got.Temperature.Valid() {
		t.Fatalf("Temperature should be unset for reasoning models, got %#v", got.Temperature)
	}
}

func TestBuildGenConfigOpenAINonReasoningKeepsTemperature(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Provider:    ProviderOpenAI,
		Model:       "gpt-4o",
		Temperature: 0.3,
		MaxTokens:   256,
	}
	got, ok := cfg.buildGenConfig().(*openai.ChatCompletionNewParams)
	if !ok || got == nil {
		t.Fatalf("unexpected type %T", cfg.buildGenConfig())
	}
	if got.ReasoningEffort != "" {
		t.Fatalf("ReasoningEffort should be unset, got %#v", got.ReasoningEffort)
	}
	if !got.Temperature.Valid() || got.Temperature.Or(0) != 0.3 {
		t.Fatalf("Temperature=%#v", got.Temperature)
	}
	if !got.MaxTokens.Valid() || got.MaxTokens.Or(0) != 256 {
		t.Fatalf("MaxTokens=%#v", got.MaxTokens)
	}
	if got.MaxCompletionTokens.Valid() {
		t.Fatalf("MaxCompletionTokens should be unset, got %#v", got.MaxCompletionTokens)
	}
}

func TestBuildGenConfigOpenAICompatibleSkipsReasoningEffort(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Provider:  ProviderOpenAICompatible,
		Model:     "gpt-4o",
		MaxTokens: 512,
	}
	got, ok := cfg.buildGenConfig().(*openai.ChatCompletionNewParams)
	if !ok || got == nil {
		t.Fatalf("unexpected type %T", cfg.buildGenConfig())
	}
	if got.ReasoningEffort != "" {
		t.Fatalf("ReasoningEffort should be unset for compatible, got %#v", got.ReasoningEffort)
	}
	if !got.MaxTokens.Valid() || got.MaxTokens.Or(0) != 512 {
		t.Fatalf("MaxTokens=%#v", got.MaxTokens)
	}
	if got.Temperature.Valid() {
		t.Fatalf("Temperature=0 should be unset for provider default, got %#v", got.Temperature)
	}
}

func TestBuildGenConfigTemperatureZeroOmitsParam(t *testing.T) {
	t.Parallel()

	openaiCfg := (&Config{Provider: ProviderOpenAI, Model: "gpt-4o"}).buildGenConfig().(*openai.ChatCompletionNewParams)
	if openaiCfg.Temperature.Valid() {
		t.Fatalf("OpenAI Temperature should be unset, got %#v", openaiCfg.Temperature)
	}

	ollamaCfg := (&Config{Provider: ProviderOllama, Model: "llama3.2"}).buildGenConfig().(*ollama.GenerateContentConfig)
	if ollamaCfg.Temperature != nil {
		t.Fatalf("Ollama Temperature should be unset, got %#v", ollamaCfg.Temperature)
	}

	googleCfg := (&Config{Provider: ProviderGoogle, Model: "gemini-2.0-flash"}).buildGenConfig().(*genai.GenerateContentConfig)
	if googleCfg.Temperature != nil {
		t.Fatalf("Google Temperature should be unset, got %#v", googleCfg.Temperature)
	}
}

func TestBuildGenConfigOpenAICompatibleReasoningUsesMaxCompletionTokens(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Provider:    ProviderOpenAICompatible,
		Model:       "o3-mini",
		Temperature: 0.9,
		MaxTokens:   800,
	}
	got, ok := cfg.buildGenConfig().(*openai.ChatCompletionNewParams)
	if !ok || got == nil {
		t.Fatalf("unexpected type %T", cfg.buildGenConfig())
	}
	if got.ReasoningEffort != "" {
		t.Fatalf("ReasoningEffort should be unset for compatible, got %#v", got.ReasoningEffort)
	}
	if !got.MaxCompletionTokens.Valid() || got.MaxCompletionTokens.Or(0) != 800 {
		t.Fatalf("MaxCompletionTokens=%#v", got.MaxCompletionTokens)
	}
	if got.MaxTokens.Valid() {
		t.Fatalf("MaxTokens should be unset, got %#v", got.MaxTokens)
	}
	if got.Temperature.Valid() {
		t.Fatalf("Temperature should be unset, got %#v", got.Temperature)
	}
}

func TestOllamaThinkOption(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model   string
		wantNil bool
		effort  bool // ThinkEffort vs ThinkEnabled
	}{
		{"llama3.2", true, false},
		{"qwen2.5:7b", true, false},
		{"deepseek-r1", false, false},
		{"deepseek-r1:14b", false, false},
		{"qwq:32b", false, false},
		{"magistral", false, false},
		{"gpt-oss:20b", false, true},
		{"GPT-OSS-120B", false, true},
	}
	for _, tc := range cases {
		got := ollamaThinkOption(tc.model)
		if tc.wantNil {
			if got != nil {
				t.Fatalf("%q: want nil Think, got %#v", tc.model, got)
			}
			continue
		}
		if got == nil || !got.IsEnabled() {
			t.Fatalf("%q: want enabled Think, got %#v", tc.model, got)
		}
		b, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("%q: marshal: %v", tc.model, err)
		}
		if tc.effort {
			if string(b) != `"medium"` {
				t.Fatalf("%q: want ThinkEffort medium, got %s", tc.model, b)
			}
		} else if string(b) != "true" {
			t.Fatalf("%q: want ThinkEnabled true, got %s", tc.model, b)
		}
	}
}

func TestBuildGenConfigOllamaSkipsThinkByDefault(t *testing.T) {
	t.Parallel()
	cfg := &Config{Provider: ProviderOllama, Model: "llama3.2"}
	got, ok := cfg.buildGenConfig().(*ollama.GenerateContentConfig)
	if !ok || got == nil {
		t.Fatalf("unexpected type %T", cfg.buildGenConfig())
	}
	if got.Think != nil {
		t.Fatalf("Think should be unset for llama3.2, got %#v", got.Think)
	}
}

func TestIsGoogleThinkingModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model string
		want  bool
	}{
		{"gemini-1.5-flash", false},
		{"gemini-2.0-flash", false},
		{"gemini-2.5-flash", true},
		{"gemini-2.5-pro", true},
		{"googleai/gemini-2.5-flash-lite", true},
		{"gemini-3-pro-preview", true},
	}
	for _, tc := range cases {
		if got := isGoogleThinkingModel(tc.model); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.model, got, tc.want)
		}
	}
}

func TestBuildGenConfigGoogleThinkingOnlyWhenSupported(t *testing.T) {
	t.Parallel()
	plain := (&Config{Provider: ProviderGoogle, Model: "gemini-2.0-flash"}).buildGenConfig().(*genai.GenerateContentConfig)
	if plain.ThinkingConfig != nil {
		t.Fatalf("ThinkingConfig should be unset for 2.0, got %#v", plain.ThinkingConfig)
	}
	thinking := (&Config{Provider: ProviderGoogle, Model: "gemini-2.5-flash"}).buildGenConfig().(*genai.GenerateContentConfig)
	if thinking.ThinkingConfig == nil || !thinking.ThinkingConfig.IncludeThoughts {
		t.Fatalf("ThinkingConfig=%#v", thinking.ThinkingConfig)
	}
}

func TestEffectiveMaxAgentTurns(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, DefaultMaxAgentTurns},
		{-1, DefaultMaxAgentTurns},
		{30, DefaultMaxAgentTurns},
		{DefaultMaxAgentTurns, DefaultMaxAgentTurns},
		{DefaultMaxAgentTurns + 10, DefaultMaxAgentTurns + 10},
	}
	for _, tc := range cases {
		cfg := &Config{MaxAgentTurns: tc.in}
		if got := cfg.EffectiveMaxAgentTurns(); got != tc.want {
			t.Fatalf("MaxAgentTurns=%d: got %d, want %d", tc.in, got, tc.want)
		}
	}
	if got := (*Config)(nil).EffectiveMaxAgentTurns(); got != DefaultMaxAgentTurns {
		t.Fatalf("nil config: got %d, want %d", got, DefaultMaxAgentTurns)
	}
}
