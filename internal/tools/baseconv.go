package tools

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/ilaziness/vela/internal/i18n"
)

// BaseConvertResult 进制转换结果
type BaseConvertResult struct {
	Success bool   `json:"success"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

// ConvertBase 在 2–36 进制之间转换
func (ts *Service) ConvertBase(input string, fromBase int, toBase int) BaseConvertResult {
	input = strings.TrimSpace(input)
	if input == "" {
		return BaseConvertResult{Success: false, Error: i18n.T("tool.err.input_empty")}
	}
	if fromBase < 2 || fromBase > 36 {
		return BaseConvertResult{Success: false, Error: i18n.T("tool.err.from_base")}
	}
	if toBase < 2 || toBase > 36 {
		return BaseConvertResult{Success: false, Error: i18n.T("tool.err.to_base")}
	}

	negative := false
	if strings.HasPrefix(input, "-") {
		negative = true
		input = input[1:]
	}
	lower := strings.ToLower(input)
	// 仅在对应进制下剥离常见前缀，避免把十进制 0x10 误解析成 10
	switch {
	case fromBase == 16 && strings.HasPrefix(lower, "0x"):
		input = input[2:]
	case fromBase == 2 && strings.HasPrefix(lower, "0b"):
		input = input[2:]
	}

	n := new(big.Int)
	if _, ok := n.SetString(input, fromBase); !ok {
		// en.json: "Cannot parse %s in base %s" — input then fromBase
		return BaseConvertResult{Success: false, Error: i18n.Tf("tool.err.base_parse", input, strconv.Itoa(fromBase))}
	}

	result := n.Text(toBase)
	if negative && n.Sign() != 0 {
		result = "-" + result
	}
	return BaseConvertResult{Success: true, Result: result}
}
