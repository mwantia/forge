package log

import (
	"context"
	"reflect"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
)

// LoggerTagProcessor handles fabric:"logger" and fabric:"logger:<name>" tags
// for automatic logger injection with optional named loggers.
//
// Supported tag formats:
//   - `fabric:"logger"` - Injects the base logger service
//   - `fabric:"logger:<name>"` - Injects a named logger (e.g., logger.Named("database"))
type HcLogTagProcessor struct {
}

// NewLoggerTagProcessor creates a new LoggerTagProcessor instance with `hclog.Default()`.
func NewLoggerTagProcessor() *HcLogTagProcessor {
	return &HcLogTagProcessor{}
}

func init() {
	container.AddTagProcessor(NewLoggerTagProcessor())
}

// GetPriority returns the processing priority for this processor.
// Priority 50 ensures it runs before the default inject processor (priority 0)
// but after any custom high-priority processors.
func (p *HcLogTagProcessor) GetPriority() int {
	return 50
}

// CanProcess returns true if this processor can handle the given tag value.
// The LoggerTagProcessor handles:
//   - "logger" - for base logger injection
//   - "logger:<name>" - for named logger injection
//
// All matching is case-insensitive.
func (p *HcLogTagProcessor) CanProcess(value string) bool {
	return strings.EqualFold(value, "logger") || strings.HasPrefix(strings.ToLower(value), "logger:")
}

// Process handles the injection of loggers for fabric:"logger" tags.
// It supports both base and named logger injection:
//   - "logger" - resolves the base LoggerService
//   - "logger:<name>" - resolves the base LoggerService and calls Named(name)
//
// The method parses the tag value to extract the logger name and then
// resolves the appropriate logger from the container.
func (p *HcLogTagProcessor) Process(ctx context.Context, sc *container.ServiceContainer, field reflect.StructField, value string) (any, error) {
	// Parse the tag value to extract the logger name
	loggerName := ""
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 {
			loggerName = strings.TrimSpace(parts[1])
		}
	}

	base := hclog.Default()
	// If a name is specified, create a named logger
	if loggerName != "" {
		return base.Named(loggerName), nil
	}
	// Otherwise, return the base logger
	return base, nil
}
