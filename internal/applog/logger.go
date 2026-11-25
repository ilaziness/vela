// Package applog provides process logging used by the desktop app.
package applog

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/ilaziness/vela/internal/buildinfo"
)

var (
	global   atomic.Pointer[zap.Logger]
	globalMu sync.Mutex
)

// SetDefault installs l as the process-wide logger returned by L.
func SetDefault(l *zap.Logger) {
	if l == nil {
		l = zap.NewNop()
	}
	global.Store(l)
}

// L returns the process-wide logger, lazily creating it via New on first
// call so packages can log without an injected *zap.Logger.
func L() *zap.Logger {
	if l := global.Load(); l != nil {
		return l
	}
	globalMu.Lock()
	defer globalMu.Unlock()
	if l := global.Load(); l != nil {
		return l
	}
	return New()
}

// New creates the application logger (file + console in non-release) and
// installs it as the process-wide logger returned by L.
func New() *zap.Logger {
	level := zap.NewAtomicLevelAt(zap.DebugLevel)
	if buildinfo.IsRelease() {
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	logger, err := newLogger("logs/vela.log", level)
	if err != nil {
		logger = zap.NewNop()
	} else {
		logger.Info("Logger initialized",
			zap.String("mode", buildinfo.Mode),
			zap.String("logPath", "logs/vela.log"),
			zap.String("logLevel", level.String()),
		)
	}
	SetDefault(logger)
	return logger
}

func newLogger(logPath string, level zap.AtomicLevel) (*zap.Logger, error) {
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	writer, err := rotatelogs.New(
		logPath+".%Y-%m",
		rotatelogs.WithLinkName(logPath),
		rotatelogs.WithRotationTime(24*time.Hour*30),
		rotatelogs.WithMaxAge(365*24*time.Hour),
	)
	if err != nil {
		return nil, err
	}
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	fileCore := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(writer), level)
	consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.AddSync(os.Stdout), level)
	var core zapcore.Core
	if buildinfo.IsRelease() {
		core = fileCore
	} else {
		core = zapcore.NewTee(fileCore, consoleCore)
	}
	if buildinfo.IsRelease() {
		return zap.New(core), nil
	}
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}
