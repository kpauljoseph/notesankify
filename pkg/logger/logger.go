package logger

import (
	"io"
	"log"
	"os"
)

type LogLevel int

const (
	LevelInfo LogLevel = iota
	LevelDebug
	LevelTrace
)

type Logger struct {
	*log.Logger
	level     LogLevel
	isVerbose bool
}

type Option func(*Logger)

func WithOutput(w io.Writer) Option {
	return func(l *Logger) {
		l.Logger = log.New(w, l.Logger.Prefix(), l.Logger.Flags())
	}
}

func WithPrefix(prefix string) Option {
	return func(l *Logger) {
		l.Logger = log.New(l.Logger.Writer(), prefix, l.Logger.Flags())
	}
}

func WithFlags(flags int) Option {
	return func(l *Logger) {
		l.Logger = log.New(l.Logger.Writer(), l.Logger.Prefix(), flags)
	}
}

func New(options ...Option) *Logger {
	l := &Logger{
		Logger:    log.New(os.Stdout, "", log.LstdFlags),
		level:     LevelInfo,
		isVerbose: false,
	}

	for _, opt := range options {
		opt(l)
	}

	return l
}

func (l *Logger) SetVerbose(verbose bool) {
	l.isVerbose = verbose
}

func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.printf(LevelInfo, format, args...)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.isVerbose {
		l.printf(LevelDebug, format, args...)
	}
}

func (l *Logger) Trace(format string, args ...interface{}) {
	if l.level >= LevelTrace {
		l.printf(LevelTrace, format, args...)
	}
}

func (l *Logger) printf(level LogLevel, format string, args ...interface{}) {
	var prefix string
	switch level {
	case LevelInfo:
		prefix = "INFO: "
	case LevelDebug:
		prefix = "DEBUG: "
	case LevelTrace:
		prefix = "TRACE: "
	}
	l.Logger.Printf(prefix+format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.Logger.Fatalf("FATAL: "+format, args...)
}
