package config

import (
	"context"
	"reflect"
	"strings"

	"github.com/mwantia/fabric/pkg/container"
)

// LoggerTagProcessor handles fabric:"logger" and fabric:"logger:<name>" tags
// for automatic logger injection with optional named loggers.
//
// Supported tag formats:
//   - `fabric:"logger"` - Injects the base logger service
//   - `fabric:"logger:<name>"` - Injects a named logger (e.g., logger.Named("database"))
type ConfigTagProcessor struct {
	cfg *AgentConfig
}

// NewLoggerTagProcessor creates a new LoggerTagProcessor instance.
func NewLoggerTagProcessor(cfg *AgentConfig) *ConfigTagProcessor {
	return &ConfigTagProcessor{
		cfg: cfg,
	}
}

// GetPriority returns the processing priority for this processor.
// Priority 50 ensures it runs before the default inject processor (priority 0)
// but after any custom high-priority processors.
func (p *ConfigTagProcessor) GetPriority() int {
	return 50
}

// CanProcess returns true if this processor can handle the given tag value.
// The LoggerTagProcessor handles:
//   - "config" - for base logger injection
//   - "config:<name>" - for named logger injection
//
// All matching is case-insensitive.
func (p *ConfigTagProcessor) CanProcess(value string) bool {
	return strings.EqualFold(value, "config") || strings.HasPrefix(strings.ToLower(value), "config:")
}

// Process handles the injection of loggers for fabric:"logger" tags.
// It supports both base and named logger injection:
//   - "config" - resolves the base LoggerService
//   - "config:<name>" - resolves the base LoggerService and calls Named(name)
//
// The method parses the tag value to extract the logger name and then
// resolves the appropriate logger from the container.
func (p *ConfigTagProcessor) Process(ctx context.Context, sc *container.ServiceContainer, field reflect.StructField, value string) (any, error) {
	// Parse the tag value to extract the logger name
	configName := ""
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 {
			configName = strings.ToLower(strings.TrimSpace(parts[1]))
		}
	}

	switch configName {
	case "server":
		return p.cfg.Server, nil
	case "metrics":
		return p.cfg.Metrics, nil
	default:
		return p.cfg, nil
	}
}
