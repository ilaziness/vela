package services

import "github.com/ilaziness/vela/internal/tools"

type Tool = tools.Tool
type PortCheckResult = tools.PortCheckResult
type EncodeResponse = tools.EncodeResponse
type RegexMatchResult = tools.RegexMatchResult
type HashResult = tools.HashResult
type TimestampResult = tools.TimestampResult
type FormatResult = tools.FormatResult
type CronResult = tools.CronResult
type CIDRResult = tools.CIDRResult
type JWTResult = tools.JWTResult
type RandomResult = tools.RandomResult
type BaseConvertResult = tools.BaseConvertResult
type DiffResult = tools.DiffResult
type DiffLine = tools.DiffLine
type ChmodResult = tools.ChmodResult

type ToolService struct {
	windows *Windows
	core    *tools.Service
}

func NewToolService(windows *Windows) *ToolService {
	return &ToolService{windows: windows, core: tools.New()}
}

func (ts *ToolService) ShowWindow()      { ts.windows.ShowTool() }
func (ts *ToolService) CloseWindow()     { ts.windows.CloseTool() }
func (ts *ToolService) GetTools() []Tool { return ts.core.GetTools() }
func (ts *ToolService) CheckPort(host string, port int) PortCheckResult {
	return ts.core.CheckPort(host, port)
}
func (ts *ToolService) Encode(toolType, input string) EncodeResponse {
	return ts.core.Encode(toolType, input)
}
func (ts *ToolService) Decode(toolType, input string) EncodeResponse {
	return ts.core.Decode(toolType, input)
}
func (ts *ToolService) RegexMatch(pattern, text, flags string) RegexMatchResult {
	return ts.core.RegexMatch(pattern, text, flags)
}
func (ts *ToolService) CalculateHash(input, algorithm string) HashResult {
	return ts.core.CalculateHash(input, algorithm)
}
func (ts *ToolService) ConvertTimestamp(input string, toTimestamp bool) TimestampResult {
	return ts.core.ConvertTimestamp(input, toTimestamp)
}
func (ts *ToolService) FormatJSONYAML(action, input string) FormatResult {
	return ts.core.FormatJSONYAML(action, input)
}
func (ts *ToolService) ParseCron(expr string, count int) CronResult {
	return ts.core.ParseCron(expr, count)
}
func (ts *ToolService) CalculateCIDR(cidr, checkIP string) CIDRResult {
	return ts.core.CalculateCIDR(cidr, checkIP)
}
func (ts *ToolService) DecodeJWT(token string) JWTResult {
	return ts.core.DecodeJWT(token)
}
func (ts *ToolService) GenerateRandom(kind string, length int, charset, customCharset string) RandomResult {
	return ts.core.GenerateRandom(kind, length, charset, customCharset)
}
func (ts *ToolService) ConvertBase(input string, fromBase, toBase int) BaseConvertResult {
	return ts.core.ConvertBase(input, fromBase, toBase)
}
func (ts *ToolService) DiffText(left, right string) DiffResult {
	return ts.core.DiffText(left, right)
}
func (ts *ToolService) ConvertChmod(input, direction string) ChmodResult {
	return ts.core.ConvertChmod(input, direction)
}
