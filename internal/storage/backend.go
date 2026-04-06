package storage

import "context"

// Backend is the generic key-value storage interface that all storage backends
// must implement. Keys use forward-slash separators regardless of OS.
//
// List semantics (Vault-style):
//   - Entries ending in "/" are sub-prefixes (analogous to directories).
//   - Entries without a trailing "/" are leaf values.
type Backend interface {
	// Get retrieves the value stored at key.
	// Returns nil, if the key does not exist.
	GetRaw(ctx context.Context, key string) ([]byte, error)

	// GetJSON fetches the value at key and JSON-unmarshals it.
	// Returns nil, when the key does not exist.
	GetJson(ctx context.Context, key string, v any) error

	// Put writes value at key, creating any intermediate path segments as needed.
	PutRaw(ctx context.Context, key string, value []byte) error

	// PutJSON JSON-marshals val and writes it at key.
	PutJson(ctx context.Context, key string, v any) error

	// List returns the immediate children of prefix. Sub-prefix entries end
	// with "/"; leaf entries do not. prefix itself must end with "/" or be
	// empty.
	List(ctx context.Context, prefix string) ([]string, error)

	// Delete removes the value at key.
	// It is not an error if the key does not exist.
	Delete(ctx context.Context, key string) error

	// DeletePrefix removes all keys that share the given prefix. This is the
	// equivalent of a recursive / cascading delete.
	DeletePrefix(ctx context.Context, prefix string) error
}
