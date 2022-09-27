package ecspresso

import (
	"encoding/json"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/fujiwara/logutils"
)

var (
	commonLogger    = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	commonLogFilter = &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"DEBUG", "INFO", "NOTICE", "WARNING", "ERROR"},
		ModifierFuncs: []logutils.ModifierFunc{
			nil, // default
			logutils.Color(color.FgCyan),
			logutils.Color(color.FgGreen),
			logutils.Color(color.FgYellow),
			logutils.Color(color.FgRed),
		},
		MinLevel: logutils.LogLevel("NOTICE"),
		Writer:   os.Stderr,
	}
)

func init() {
	commonLogger = NewLogger()
}

func NewLogger() *log.Logger {
	logger := log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	logger.SetOutput(commonLogFilter)
	return logger
}

func Log(f string, v ...interface{}) {
	commonLogger.Printf(f, v...)
}

func (d *App) Log(f string, v ...interface{}) {
	d.logger.Printf(d.Name()+" "+f, v...)
}

func (d *App) DebugLog(f string, v ...interface{}) {
	if !d.Debug {
		return
	}
	d.Log(f, v...)
}

func (d *App) LogJSON(v interface{}) {
	b, _ := json.Marshal(v)
	d.logger.Println(string(b))
}
