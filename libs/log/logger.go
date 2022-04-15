package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type CRLogger = *zap.SugaredLogger

func NewCRLogger(level string) CRLogger {
	var logLevel zapcore.Level

	switch level {
	case "DEBUG", "Debug", "debug":
		logLevel = zapcore.DebugLevel
	case "INFO", "Info", "info":
		logLevel = zapcore.InfoLevel
	case "ERROR", "Error", "error":
		logLevel = zapcore.ErrorLevel
	default:
		logLevel = zapcore.InfoLevel
	}

	var zc = zap.Config{
		Level:             zap.NewAtomicLevelAt(logLevel),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: false,
		Sampling:          nil,
		Encoding:          "console",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:     "message",
			LevelKey:       "level",
			TimeKey:        "time",
			NameKey:        "name",
			CallerKey:      "caller",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
			EncodeName:     zapcore.FullNameEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	core, err := zc.Build()
	if err != nil {
		panic(err)
	}
	return core.Sugar()
}