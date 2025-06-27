package gobonsai

import (
	"log"
	"os"

	"github.com/fatih/color"
)

type Logger struct {
	name     string
	errorLog *log.Logger
	infoLog  *log.Logger
	fatalLog *log.Logger
	debugLog *log.Logger
	traceLog *log.Logger
	warnLog  *log.Logger
}

func NewLogger(name string) *Logger {
	makeLogger := func(prefix string, c *color.Color, w *os.File) *log.Logger {
		return log.New(w, c.Sprint(prefix+name+" "), log.LstdFlags)
	}
	return &Logger{
		name:     name,
		errorLog: makeLogger("[ERROR] ", color.New(color.FgRed), os.Stdout),
		infoLog:  makeLogger("[INFO] ", color.New(color.FgGreen), os.Stdout),
		fatalLog: makeLogger("[FATAL] ", color.New(color.FgHiRed), os.Stderr),
		debugLog: makeLogger("[DEBUG] ", color.New(color.FgBlue), os.Stdout),
		traceLog: makeLogger("[TRACE] ", color.New(color.FgCyan), os.Stdout),
		warnLog:  makeLogger("[WARN] ", color.New(color.FgYellow), os.Stdout),
	}
}

func (l *Logger) Error(v ...interface{}) {
	if Debug {
		l.errorLog.Println(v...)
	}
}
func (l *Logger) Info(v ...interface{}) {
	if Debug {
		l.infoLog.Println(v...)
	}
}
func (l *Logger) Fatal(v ...interface{}) {
	if Debug {
		l.fatalLog.Println(v...)
		os.Exit(1)
	}
}
func (l *Logger) Debug(v ...interface{}) {
	if Debug {
		l.debugLog.Println(v...)
	}
}
func (l *Logger) Trace(v ...interface{}) {
	if Debug {
		l.traceLog.Println(v...)
	}
}
func (l *Logger) Warn(v ...interface{}) {
	if Debug {
		l.warnLog.Println(v...)
	}
}
