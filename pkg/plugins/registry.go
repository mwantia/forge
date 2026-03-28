package plugins

import (
	"sort"

	"github.com/hashicorp/go-hclog"
)

// DriverFactory creates a Driver implementation with a logger.
type DriverFactory func(log hclog.Logger) Driver

// RegistryEntry holds a plugin factory and its metadata.
type RegistryEntry struct {
	Name        string
	Description string
	Factory     DriverFactory
}

// registry holds all registered plugin entries.
var registry = make(map[string]RegistryEntry)

// Register adds a plugin factory to the registry.
// This should be called in init() functions of plugin packages.
func Register(name, description string, factory DriverFactory) {
	registry[name] = RegistryEntry{
		Name:        name,
		Description: description,
		Factory:     factory,
	}
}

// Get returns a plugin factory by name, or nil if not found.
func Get(name string) DriverFactory {
	entry, ok := registry[name]
	if !ok {
		return nil
	}
	return entry.Factory
}

// List returns all registered plugin entries sorted by name.
func List() []RegistryEntry {
	entries := make([]RegistryEntry, 0, len(registry))
	for _, e := range registry {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// Names returns all registered plugin names.
func Names() []string {
	entries := List()
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names
}
