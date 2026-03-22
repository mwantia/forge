package log

import "io"

type loggerOptions struct {
	Name       string     `json:"name"`
	Level      LogLevel   `json:"log_level"`
	TimeFormat string     `json:"time_format"`
	Color      bool       `json:"enable_color"`
}

func newDefault() loggerOptions {
	return loggerOptions{
		Name:       "forge",
		Level:      Warn,
		TimeFormat: "2006-01-02 15:04:05",
		Color:      true,
	}
}

type LoggerOption func(*loggerOptions)

func WithName(name string) LoggerOption {
	return func(options *loggerOptions) {
		options.Name = name
	}
}

func WithLogLevel(level string) LoggerOption {
	return func(options *loggerOptions) {
		options.Level = parse(level)
	}
}

func WithLevel(level LogLevel) LoggerOption {
	return func(options *loggerOptions) {
		options.Level = level
	}
}

func WithTimeFormat(format string) LoggerOption {
	return func(options *loggerOptions) {
		options.TimeFormat = format
	}
}

func WithColor(enableColor bool) LoggerOption {
	return func(options *loggerOptions) {
		options.Color = enableColor
	}
}

// StandardLoggerOptions configures standard logger output.
type StandardLoggerOptions struct {
	// ForceLevel forces all output to use the provided level.
	ForceLevel LogLevel

	// InferLevel determines whether to infer log level from callsite.
	InferLevel bool

	// Writer overrides the output destination.
	Writer io.Writer
}