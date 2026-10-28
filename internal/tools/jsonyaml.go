package tools

import (
	"bytes"
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ilaziness/vela/internal/i18n"
)

// FormatResult JSON/YAML 处理结果
type FormatResult struct {
	Success bool   `json:"success"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

// FormatJSONYAML 处理 JSON/YAML：format | minify | validate | json2yaml | yaml2json
func (ts *Service) FormatJSONYAML(action string, input string) FormatResult {
	input = strings.TrimSpace(input)
	if input == "" {
		return FormatResult{Success: false, Error: i18n.T("tool.err.input_empty")}
	}

	switch strings.ToLower(action) {
	case "format":
		return formatJSON(input)
	case "minify":
		return minifyJSON(input)
	case "validate":
		return validateJSON(input)
	case "json2yaml":
		return jsonToYAML(input)
	case "yaml2json":
		return yamlToJSON(input)
	default:
		return FormatResult{Success: false, Error: i18n.Tf("tool.err.unsupported_action", action)}
	}
}

func formatJSON(input string) FormatResult {
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return FormatResult{Success: false, Error: i18n.Tf("tool.err.json_parse", err.Error())}
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return FormatResult{Success: false, Error: i18n.Tf("tool.err.json_format", err.Error())}
	}
	return FormatResult{Success: true, Result: string(out)}
}

func minifyJSON(input string) FormatResult {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(input)); err != nil {
		return FormatResult{Success: false, Error: i18n.Tf("tool.err.json_minify", err.Error())}
	}
	return FormatResult{Success: true, Result: buf.String()}
}

func validateJSON(input string) FormatResult {
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return FormatResult{Success: false, Error: i18n.Tf("tool.err.invalid_json", err.Error())}
	}
	return FormatResult{Success: true, Result: "Valid JSON"}
}

func jsonToYAML(input string) FormatResult {
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return FormatResult{Success: false, Error: i18n.Tf("tool.err.json_parse", err.Error())}
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return FormatResult{Success: false, Error: i18n.Tf("tool.err.yaml_convert", err.Error())}
	}
	return FormatResult{Success: true, Result: string(out)}
}

func yamlToJSON(input string) FormatResult {
	var v any
	if err := yaml.Unmarshal([]byte(input), &v); err != nil {
		return FormatResult{Success: false, Error: i18n.Tf("tool.err.yaml_parse", err.Error())}
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return FormatResult{Success: false, Error: i18n.Tf("tool.err.json_convert", err.Error())}
	}
	return FormatResult{Success: true, Result: string(out)}
}
