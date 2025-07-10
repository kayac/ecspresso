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
		HandlerOptions: slog.HandlerOptions{
			Level: logLevel,
		},
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

func LogInfo(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	commonLogger.Info(msg)
}

func LogWarn(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	commonLogger.Warn(msg)
}

func LogError(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	commonLogger.Error(msg)
}

func (d *App) LogDebug(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	d.logger.Debug(msg)
}

func (d *App) LogInfo(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	d.logger.Info(msg)
}

func (d *App) LogWarn(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	d.logger.Warn(msg)
}

func (d *App) LogError(f string, v ...any) {
	msg := fmt.Sprintf(f, v...)
	d.logger.Error(msg)
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
