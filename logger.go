package ecspresso

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/fujiwara/logutils"
)

var (
	commonLogger *log.Logger
)

func init() {
	commonLogger = newLogger()
	commonLogger.SetOutput(newLogFilter(os.Stderr, "INFO"))
}

func newLogFilter(w io.Writer, minLevel string) *logutils.LevelFilter {
	return &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"DEBUG", "INFO", "WARNING", "ERROR"},
		ModifierFuncs: []logutils.ModifierFunc{
			nil, // DEBUG
			nil, // default
			logutils.Color(color.FgYellow),
			logutils.Color(color.FgRed),
		},
		MinLevel: logutils.LogLevel(minLevel),
		Writer:   w,
	}
}

func newLogger() *log.Logger {
	return log.New(io.Discard, "", log.LstdFlags)
}

func Log(f string, v ...interface{}) {
	commonLogger.Printf(f, v...)
}

func (d *App) Log(f string, v ...interface{}) {
	d.logger.Printf(d.Name()+" "+f, v...)
}

func (d *App) LogJSON(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		d.logger.Printf("[WARNING] failed to marshal json: %s", err)
		return
	}
	d.logger.Println("[INFO] " + string(b))
}
