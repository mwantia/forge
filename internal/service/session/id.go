package session

import (
	"github.com/mwantia/forge-sdk/pkg/contenthash"
)

// DeriveSessionID hashes a session's creation event per
// docs/03-proposal-merkle-DAG-concept.md §1.5. The first 16 bytes of the
// SHA-256 (32 hex chars) form the ID. Frozen at creation; renames or
// model swaps do not change the result.
func DeriveSessionID(name string, createdAtUnixNano int64, parent string) string {
	full, err := contenthash.Hash(map[string]any{
		"name":                 name,
		"created_at_unix_nano": createdAtUnixNano,
		"parent_session_id":    parent,
	})
	if err != nil {
		// Hashing a fixed shape can only fail on programmer error.
		panic("session: contenthash failed on session-id derivation: " + err.Error())
	}
	return full[:32]
}
