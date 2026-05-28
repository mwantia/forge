package storage

import (
	"context"
	"fmt"
	"strings"
)

// Migrate copies all keys from source to destination by recursively walking
// the source tree starting at the root prefix. Leaf keys are copied with
// ReadRaw/WriteRaw. The destination is not cleared beforehand; overlapping keys
// are overwritten.
func Migrate(ctx context.Context, source StorageBackend, destination StorageBackend) error {
	return migratePrefix(ctx, source, destination, "")
}

// migratePrefix recursively walks prefix in source, copying each leaf key to
// destination. Sub-prefix entries (ending in "/") are visited recursively;
// all others are treated as leaves.
func migratePrefix(ctx context.Context, source, destination StorageBackend, prefix string) error {
	entries, err := source.ListEntry(ctx, prefix)
	if err != nil {
		return fmt.Errorf("list %q: %w", prefix, err)
	}

	for _, entry := range entries {
		full := prefix + entry

		if strings.HasSuffix(entry, "/") {
			if err := migratePrefix(ctx, source, destination, full); err != nil {
				return err
			}
			continue
		}

		data, err := source.ReadRaw(ctx, full)
		if err != nil {
			return fmt.Errorf("read %q: %w", full, err)
		}
		if data == nil {
			// Key reported by ListEntry but not readable; skip silently.
			continue
		}

		if err := destination.WriteRaw(ctx, full, data); err != nil {
			return fmt.Errorf("write %q: %w", full, err)
		}

	}

	return nil
}
