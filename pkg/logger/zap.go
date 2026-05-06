package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newZapLogger(namespace string) *zap.Logger {
	return newZapLoggerWithLevel(namespace, "info")
}

func newZapLoggerWithLevel(namespace, levelStr string) *zap.Logger {
	stdout := zapcore.AddSync(os.Stdout)

	level := zap.NewAtomicLevelAt(parseLevel(levelStr))

	developmentCfg := zap.NewDevelopmentEncoderConfig()
	developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)

	core := zapcore.NewTee(zapcore.NewCore(consoleEncoder, stdout, level))

	return zap.New(core)
}

func parseLevel(s string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zap.DebugLevel
	case "warn", "warning":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}
