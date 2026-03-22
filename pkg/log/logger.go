package log

import (
	"io"
	"log"
	"strings"
)

// Logger interface compatible with hclog.Logger for plugin system integration.
type Logger interface {
	// Core logging methods
	Trace(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})

	// Level check methods
	IsTrace() bool
	IsDebug() bool
	IsInfo() bool
	IsWarn() bool
	IsError() bool

	// Logging with explicit level
	Log(level LogLevel, msg string, args ...interface{})

	// Context methods
	ImpliedArgs() []interface{}
	With(args ...interface{}) Logger

	// Name methods
	Name() string
	Named(name string) Logger
	ResetNamed(name string) Logger

	// Level methods
	SetLevel(level LogLevel)
	GetLevel() LogLevel

	// Standard library compatibility
	StandardLogger(opts *StandardLoggerOptions) *log.Logger
	StandardWriter(opts *StandardLoggerOptions) io.Writer
}

type LogLevel int

const (
	Trace LogLevel = iota
	Debug
	Info
	Warn
	Error
)

func (l LogLevel) String() string {
	switch l {
	case Trace:
		return "TRACE"
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func parse(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "TRACE":
		return Trace
	case "DEBUG":
		return Debug
	case "INFO":
		return Info
	case "WARN":
		return Warn
	case "ERROR":
		return Error
	default:
		panic("Invalid log_level defined...")
	}
}
