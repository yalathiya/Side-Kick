package logging

import (
	"log"
	"os"
)

// Logger is a simple structured logger for Sidekick.
type Logger struct {
	info  *log.Logger
	error *log.Logger
}

// New creates a new Logger.
func New() *Logger {
	return &Logger{
		info:  log.New(os.Stdout, "[SIDEKICK] ", log.LstdFlags),
		error: log.New(os.Stderr, "[SIDEKICK][ERROR] ", log.LstdFlags),
	}
}

func (l *Logger) Info(format string, args ...any) {
	l.info.Printf(format, args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.error.Printf(format, args...)
}
