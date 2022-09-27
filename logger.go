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
	commonLogger    *log.Logger
	commonLogFilter *logutils.LevelFilter
)

func init() {
	commonLogger = newLogger()
	commonLogFilter = newLogFilter(os.Stderr, "WARNING")
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
	return log.New(io.Discard, "", log.LstdFlags|log.Lmicroseconds)
}

func Log(f string, v ...interface{}) {
	commonLogger.Printf(f, v...)
}

func (d *App) Log(f string, v ...interface{}) {
	d.logger.Printf(d.Name()+" "+f, v...)
}

func (d *App) LogJSON(v interface{}) {
	b, _ := json.Marshal(v)
	d.logger.Println(string(b))
}
