package plugins

import "github.com/hashicorp/go-hclog"

// DriverFactory creates a Driver implementation with a logger.
type DriverFactory func(log hclog.Logger) Driver

// registry holds all registered plugin factories.
var registry = make(map[string]DriverFactory)

// Register adds a plugin factory to the registry.
// This should be called in init() functions of plugin packages.
func Register(name string, factory DriverFactory) {
	registry[name] = factory
}

// Get returns a plugin factory by name, or nil if not found.
func Get(name string) DriverFactory {
	return registry[name]
}

// Names returns all registered plugin names.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
