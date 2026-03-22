package log

import (
	"io"
	"log"

	"github.com/hashicorp/go-hclog"
)

// Ensure hclogAdapter implements hclog.Logger.
var _ hclog.Logger = (*hclogAdapter)(nil)

// hclogAdapter wraps our Logger to implement hclog.Logger.
type hclogAdapter struct {
	Logger
}

// NewHclogAdapter creates an hclog.Logger from our Logger.
func NewHclogAdapter(logger Logger) hclog.Logger {
	return &hclogAdapter{Logger: logger}
}

func (a *hclogAdapter) Trace(msg string, args ...interface{}) {
	a.Logger.Trace(msg, args...)
}

func (a *hclogAdapter) Debug(msg string, args ...interface{}) {
	a.Logger.Debug(msg, args...)
}

func (a *hclogAdapter) Info(msg string, args ...interface{}) {
	a.Logger.Info(msg, args...)
}

func (a *hclogAdapter) Warn(msg string, args ...interface{}) {
	a.Logger.Warn(msg, args...)
}

func (a *hclogAdapter) Error(msg string, args ...interface{}) {
	a.Logger.Error(msg, args...)
}

func (a *hclogAdapter) IsTrace() bool {
	return a.Logger.IsTrace()
}

func (a *hclogAdapter) IsDebug() bool {
	return a.Logger.IsDebug()
}

func (a *hclogAdapter) IsInfo() bool {
	return a.Logger.IsInfo()
}

func (a *hclogAdapter) IsWarn() bool {
	return a.Logger.IsWarn()
}

func (a *hclogAdapter) IsError() bool {
	return a.Logger.IsError()
}

func (a *hclogAdapter) ImpliedArgs() []interface{} {
	return a.Logger.ImpliedArgs()
}

func (a *hclogAdapter) With(args ...interface{}) hclog.Logger {
	return &hclogAdapter{Logger: a.Logger.With(args...)}
}

func (a *hclogAdapter) Name() string {
	return a.Logger.Name()
}

func (a *hclogAdapter) Named(name string) hclog.Logger {
	return &hclogAdapter{Logger: a.Logger.Named(name)}
}

func (a *hclogAdapter) ResetNamed(name string) hclog.Logger {
	return &hclogAdapter{Logger: a.Logger.ResetNamed(name)}
}

func (a *hclogAdapter) SetLevel(level hclog.Level) {
	switch level {
	case hclog.Trace:
		a.Logger.SetLevel(Trace)
	case hclog.Debug:
		a.Logger.SetLevel(Debug)
	case hclog.Info:
		a.Logger.SetLevel(Info)
	case hclog.Warn:
		a.Logger.SetLevel(Warn)
	case hclog.Error:
		a.Logger.SetLevel(Error)
	default:
		a.Logger.SetLevel(Info)
	}
}

func (a *hclogAdapter) GetLevel() hclog.Level {
	level := a.Logger.GetLevel()
	switch level {
	case Trace:
		return hclog.Trace
	case Debug:
		return hclog.Debug
	case Info:
		return hclog.Info
	case Warn:
		return hclog.Warn
	case Error:
		return hclog.Error
	default:
		return hclog.Info
	}
}

func (a *hclogAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Trace:
		a.Logger.Trace(msg, args...)
	case hclog.Debug:
		a.Logger.Debug(msg, args...)
	case hclog.Info:
		a.Logger.Info(msg, args...)
	case hclog.Warn:
		a.Logger.Warn(msg, args...)
	case hclog.Error:
		a.Logger.Error(msg, args...)
	}
}

func (a *hclogAdapter) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	if opts == nil {
		return a.Logger.StandardLogger(nil)
	}
	stdOpts := &StandardLoggerOptions{
		ForceLevel: convertHclogLevel(opts.ForceLevel),
		InferLevel: opts.InferLevels,
	}
	return a.Logger.StandardLogger(stdOpts)
}

func (a *hclogAdapter) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	if opts == nil {
		return a.Logger.StandardWriter(nil)
	}
	stdOpts := &StandardLoggerOptions{
		ForceLevel: convertHclogLevel(opts.ForceLevel),
		InferLevel: opts.InferLevels,
	}
	return a.Logger.StandardWriter(stdOpts)
}

func convertHclogLevel(level hclog.Level) LogLevel {
	switch level {
	case hclog.Trace:
		return Trace
	case hclog.Debug:
		return Debug
	case hclog.Info:
		return Info
	case hclog.Warn:
		return Warn
	case hclog.Error:
		return Error
	default:
		return Info
	}
}