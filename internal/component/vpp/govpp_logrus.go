package vpp

import (
	"context"
	"io"
	"log/slog"
	"sort"

	"github.com/sirupsen/logrus"
	"go.fd.io/govpp/adapter/socketclient"
	"go.fd.io/govpp/adapter/statsclient"
	"go.fd.io/govpp/core"
)

type govppLogrusHook struct {
	logger    *slog.Logger
	baseAttrs []any
}

func setGovppLoggers(logger *slog.Logger) {
	if logger == nil {
		return
	}

	core.SetLogger(newGovppLogrusLogger(logger, "logger", "govpp/core"))
	socketclient.SetLogger(newGovppLogrusLogger(logger, "logger", "govpp/socketclient"))
	statsclient.Log = newGovppLogrusLogger(logger, "logger", "govpp/statsclient")
	statsclient.Debug = logger.Enabled(context.Background(), slog.LevelDebug)
}

func newGovppLogrusLogger(logger *slog.Logger, baseAttrs ...any) *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	l.SetLevel(govppLogrusLevel(logger))
	l.AddHook(govppLogrusHook{logger: logger, baseAttrs: append([]any(nil), baseAttrs...)})
	return l
}

func govppLogrusLevel(logger *slog.Logger) logrus.Level {
	if logger == nil {
		return logrus.PanicLevel
	}

	ctx := context.Background()
	switch {
	case logger.Enabled(ctx, slog.LevelDebug):
		return logrus.DebugLevel
	case logger.Enabled(ctx, slog.LevelInfo):
		return logrus.InfoLevel
	case logger.Enabled(ctx, slog.LevelWarn):
		return logrus.WarnLevel
	case logger.Enabled(ctx, slog.LevelError):
		return logrus.ErrorLevel
	default:
		return logrus.PanicLevel
	}
}

func (h govppLogrusHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h govppLogrusHook) Fire(entry *logrus.Entry) error {
	if h.logger == nil {
		return nil
	}

	args := make([]any, 0, len(h.baseAttrs)+len(entry.Data)*2)
	for i := 0; i+1 < len(h.baseAttrs); i += 2 {
		key, ok := h.baseAttrs[i].(string)
		if !ok {
			continue
		}
		if _, exists := entry.Data[key]; exists {
			continue
		}
		args = append(args, key, h.baseAttrs[i+1])
	}

	keys := make([]string, 0, len(entry.Data))
	for key := range entry.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, key, entry.Data[key])
	}

	h.logger.Log(context.Background(), slogLevelFromLogrus(entry.Level), entry.Message, args...)
	return nil
}

func slogLevelFromLogrus(level logrus.Level) slog.Level {
	switch level {
	case logrus.TraceLevel, logrus.DebugLevel:
		return slog.LevelDebug
	case logrus.InfoLevel:
		return slog.LevelInfo
	case logrus.WarnLevel:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}
