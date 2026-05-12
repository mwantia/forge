package log

import (
	"io"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"gopkg.in/natefinch/lumberjack.v2"
)

func SetupLumberjack(cfg LogConfig) error {
	writers := make([]io.Writer, 0)

	level := globLevel
	displayColour := globDisplayColour

	if displayColour {
		writers = append(writers, NewColouredWriter(os.Stdout))
	} else {
		writers = append(writers, os.Stdout)
	}

	if cfg.Level != "" {
		if lvl := hclog.LevelFromString(cfg.Level); lvl != hclog.NoLevel {
			level = lvl
		}
	}

	if cfg.File != "" {
		rotDuration := 24 * time.Hour
		if cfg.RotateDuration != "" {
			if duration, err := time.ParseDuration(cfg.RotateDuration); err == nil {
				rotDuration = duration
			}
		}

		writers = append(writers, &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    cfg.RotateBytes / 1024 / 1024,
			MaxBackups: cfg.RotateMaxFiles,
			MaxAge:     int(rotDuration.Hours() / 24),
		})
	}

	out := io.MultiWriter(writers...)

	if or, ok := globInterceptor.(hclog.OutputResettable); ok {
		or.ResetOutputWithFlush(&hclog.LoggerOptions{
			Name:        "forge",
			DisableTime: true,
			Level:       level,
			Output:      out,
		}, globWriter)
	}
	globInterceptor.SetLevel(level)

	return nil
}
