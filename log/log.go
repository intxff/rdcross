package log

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var defaultLogger *zap.Logger

type Log struct {
	Level string `yaml:"level"`
	Path  string `yaml:"path"`
}

func init() {
	var zc = zap.Config{
		Level:             zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:       false,
		DisableCaller:     true,
		DisableStacktrace: true,
		Sampling:          nil,
		Encoding:          "json",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:     "message",
			LevelKey:       "level",
			TimeKey:        "time",
			NameKey:        "name",
			CallerKey:      "caller",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
			EncodeName:     zapcore.FullNameEncoder,
		},
		OutputPaths:      []string{"stdout", "./log.json"},
		ErrorOutputPaths: []string{"stderr"},
	}

	var err error
	defaultLogger, err = zc.Build()
	if err != nil {
		panic(fmt.Sprintf("[LOGGER] ERROR: %v\n", err))
	}
}

func UpdateLogger(l *Log) {
    defaultLogger.Sync()

	var (
		logLevel zap.AtomicLevel
		stack    bool
	)
	switch l.Level {
	case "debug":
		logLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
		stack = true
	case "info":
		logLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
		stack = false
	case "error":
		logLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
		stack = false
	default:
		logLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	}

	var zc = zap.Config{
		Level:             logLevel,
		Development:       false,
		DisableCaller:     true,
		DisableStacktrace: stack,
		Sampling:          nil,
		Encoding:          "json",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:     "message",
			LevelKey:       "level",
			TimeKey:        "time",
			NameKey:        "name",
			CallerKey:      "caller",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
			EncodeName:     zapcore.FullNameEncoder,
		},
		OutputPaths:      []string{"stdout", l.Path},
		ErrorOutputPaths: []string{"stderr"},
	}

	var err error
    newLogger, err := zc.Build()
	if err != nil {
		panic(fmt.Sprintf("[LOGGER] ERROR: %v\n", err))
	}
    defaultLogger = newLogger
}

func CloseLogger() {
	defaultLogger.Sync()
}
func Error(s string, f ...zap.Field) {
	defaultLogger.Error(s, f...)
}
func Info(s string, f ...zap.Field) {
	defaultLogger.Info(s, f...)
}
func Debug(s string, f ...zap.Field) {
	defaultLogger.Debug(s, f...)
}
func Panic(s string, f ...zap.Field) {
    defaultLogger.Panic(s, f...)
}
func Fatal(s string, f ...zap.Field) {
    defaultLogger.Fatal(s, f...)
}
