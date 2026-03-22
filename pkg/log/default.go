package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

var _default Logger = &defaultLogger{
	options: newDefault(),
}

type defaultLogger struct {
	options     loggerOptions
	impliedArgs []interface{}
	mu          sync.RWMutex
}

func New(opts ...LoggerOption) Logger {
	options := newDefault()
	for _, opt := range opts {
		opt(&options)
	}

	return &defaultLogger{
		options: options,
	}
}

func SetDefault(log Logger) Logger {
	old := _default
	_default = log
	return old
}

func Named(name string) Logger {
	return _default.Named(name)
}

func (impl *defaultLogger) log(level LogLevel, msg string, args ...interface{}) {
	impl.mu.RLock()
	currentLevel := impl.options.Level
	impl.mu.RUnlock()

	if level < currentLevel {
		return
	}

	impl.mu.RLock()
	name := impl.options.Name
	timeFormat := impl.options.TimeFormat
	color := impl.options.Color
	impl.mu.RUnlock()

	timestamp := time.Now().Format(timeFormat)
	prefix := fmt.Sprintf("[%s] %-5s", timestamp, level)

	if name != "" {
		prefix = fmt.Sprintf("%s [%s]", prefix, name)
	}

	// Merge implied args with call args
	allArgs := append(impl.impliedArgs, args...)
	formattedMsg := formatMsg(msg, allArgs...)

	if color {
		fmt.Fprintf(os.Stdout, "%s%s %s\033[0m\n", Color(level), prefix, formattedMsg)
	} else {
		fmt.Fprintf(os.Stdout, "%s %s\n", prefix, formattedMsg)
	}
}

func formatMsg(msg string, args ...interface{}) string {
	if len(args) == 0 {
		return msg
	}

	// Format as key=value pairs
	result := msg
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			result = fmt.Sprintf("%s %v=%v", result, args[i], args[i+1])
		} else {
			result = fmt.Sprintf("%s %v", result, args[i])
		}
	}
	return result
}

func (impl *defaultLogger) Trace(msg string, args ...interface{}) {
	impl.log(Trace, msg, args...)
}

func (impl *defaultLogger) Debug(msg string, args ...interface{}) {
	impl.log(Debug, msg, args...)
}

func (impl *defaultLogger) Info(msg string, args ...interface{}) {
	impl.log(Info, msg, args...)
}

func (impl *defaultLogger) Warn(msg string, args ...interface{}) {
	impl.log(Warn, msg, args...)
}

func (impl *defaultLogger) Error(msg string, args ...interface{}) {
	impl.log(Error, msg, args...)
}

func (impl *defaultLogger) Log(level LogLevel, msg string, args ...interface{}) {
	impl.log(level, msg, args...)
}

func (impl *defaultLogger) IsTrace() bool {
	impl.mu.RLock()
	defer impl.mu.RUnlock()
	return impl.options.Level <= Trace
}

func (impl *defaultLogger) IsDebug() bool {
	impl.mu.RLock()
	defer impl.mu.RUnlock()
	return impl.options.Level <= Debug
}

func (impl *defaultLogger) IsInfo() bool {
	impl.mu.RLock()
	defer impl.mu.RUnlock()
	return impl.options.Level <= Info
}

func (impl *defaultLogger) IsWarn() bool {
	impl.mu.RLock()
	defer impl.mu.RUnlock()
	return impl.options.Level <= Warn
}

func (impl *defaultLogger) IsError() bool {
	impl.mu.RLock()
	defer impl.mu.RUnlock()
	return impl.options.Level <= Error
}

func (impl *defaultLogger) ImpliedArgs() []interface{} {
	return impl.impliedArgs
}

func (impl *defaultLogger) With(args ...interface{}) Logger {
	impl.mu.RLock()
	options := impl.options
	impl.mu.RUnlock()

	return &defaultLogger{
		options:     options,
		impliedArgs: append(impl.impliedArgs, args...),
	}
}

func (impl *defaultLogger) Name() string {
	impl.mu.RLock()
	defer impl.mu.RUnlock()
	return impl.options.Name
}

func (impl *defaultLogger) Named(name string) Logger {
	impl.mu.RLock()
	options := impl.options
	impl.mu.RUnlock()

	newName := name
	if options.Name != "" {
		newName = fmt.Sprintf("%s/%s", options.Name, name)
	}

	return &defaultLogger{
		options: loggerOptions{
			Name:       newName,
			Level:      options.Level,
			TimeFormat: options.TimeFormat,
			Color:      options.Color,
		},
	}
}

func (impl *defaultLogger) ResetNamed(name string) Logger {
	impl.mu.RLock()
	options := impl.options
	impl.mu.RUnlock()

	return &defaultLogger{
		options: loggerOptions{
			Name:       name,
			Level:      options.Level,
			TimeFormat: options.TimeFormat,
			Color:      options.Color,
		},
	}
}

func (impl *defaultLogger) SetLevel(level LogLevel) {
	impl.mu.Lock()
	defer impl.mu.Unlock()
	impl.options.Level = level
}

func (impl *defaultLogger) GetLevel() LogLevel {
	impl.mu.RLock()
	defer impl.mu.RUnlock()
	return impl.options.Level
}

func (impl *defaultLogger) StandardLogger(opts *StandardLoggerOptions) *log.Logger {
	impl.mu.RLock()
	name := impl.options.Name
	impl.mu.RUnlock()

	prefix := ""
	if name != "" {
		prefix = fmt.Sprintf("[%s] ", name)
	}
	return log.New(os.Stdout, prefix, log.LstdFlags)
}

func (impl *defaultLogger) StandardWriter(opts *StandardLoggerOptions) io.Writer {
	return os.Stdout
}