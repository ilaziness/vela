package services

import (
	"os"
	"path/filepath"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/ilaziness/vela/internal/buildinfo"
)

type LogService struct {
	logger *zap.Logger
	level  zap.AtomicLevel
}

func NewLogService() *LogService {
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
	return &LogService{logger: logger, level: level}
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

func (ls *LogService) Logger() *zap.Logger { return ls.logger }

func (ls *LogService) Debug(msg string) { ls.logger.Debug(msg) }
func (ls *LogService) Info(msg string)  { ls.logger.Info(msg) }
func (ls *LogService) Warn(msg string)  { ls.logger.Warn(msg) }
func (ls *LogService) Error(msg string) { ls.logger.Error(msg) }
