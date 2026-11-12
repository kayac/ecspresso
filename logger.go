package ecspresso

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/fujiwara/sloghandler"
)

var (
	logLevel           = new(slog.LevelVar)
	logFormat          string
	commonLogger       = newLogger(os.Stderr)
	slogHandlerOptions = &sloghandler.HandlerOptions{
		Color: true,
		Level: logLevel,
	}
)

const (
	logFormatText = "text"
	logFormatJSON = "json"
)

func setLogFormat(format string) {
	changed := format != logFormat
	logFormat = format
	if changed {
		commonLogger = newLogger(os.Stderr)
	}
}

func newLogger(w io.Writer) *slog.Logger {
	switch logFormat {
	case logFormatJSON:
		return slog.New(slog.NewJSONHandler(w, &slogHandlerOptions.HandlerOptions))
	case logFormatText, "":
		return slog.New(sloghandler.NewLogHandler(w, slogHandlerOptions))
	default:
		panic("unknown log format " + logFormat)
	}
}

func LogDebug(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	commonLogger.Debug(msg)
}

func LogInfo(msg string, args ...any) {
	commonLogger.Info(msg, args...)
}

func LogWarn(msg string, args ...any) {
	commonLogger.Warn(msg, args...)
}

func LogError(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	commonLogger.Error(msg)
}

func (d *App) LogDebug(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	d.logger.Debug(msg)
}

func (d *App) LogInfo(msg string, args ...any) {
	d.logger.Info(msg, args...)
}

func (d *App) LogWarn(msg string, args ...any) {
	d.logger.Warn(msg, args...)
}

func (d *App) LogError(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	d.logger.Error(msg)
}

// withDryRun appends "dry_run":true to args only when dryRun is true.
// Usage: d.LogInfo("msg", withDryRun(opt.DryRun, "key", val)...)
func withDryRun(dryRun bool, args ...any) []any {
	if dryRun {
		return append(args, "dry_run", true)
	}
	return args
}

func (d *App) LogJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		d.logger.Warn("failed to marshal json", "error", err.Error())
		return
	}
	if logLevel.Level() == slog.LevelDebug {
		// Print JSON in debug level only
		fmt.Fprintln(os.Stderr, string(b))
	}
}
