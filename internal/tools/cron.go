package tools

import (
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/ilaziness/vela/internal/i18n"
)

// CronResult Cron 解析结果
type CronResult struct {
	Success   bool     `json:"success"`
	NextTimes []string `json:"nextTimes,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// ParseCron 解析 Cron 表达式，返回接下来 count 次执行时间
func (ts *Service) ParseCron(expr string, count int) CronResult {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return CronResult{Success: false, Error: i18n.T("tool.err.cron_empty")}
	}
	if count <= 0 {
		count = 5
	}
	if count > 20 {
		count = 20
	}

	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(expr)
	if err != nil {
		return CronResult{
			Success: false,
			Error:   i18n.Tf("tool.err.cron_invalid", err.Error()),
		}
	}

	next := time.Now()
	times := make([]string, 0, count)
	for i := 0; i < count; i++ {
		next = schedule.Next(next)
		times = append(times, next.Format("2006-01-02 15:04:05"))
	}

	return CronResult{
		Success:   true,
		NextTimes: times,
	}
}
