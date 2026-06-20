package dag

import "fmt"

// Resource storage layout (flat bucket — no namespace prefix):
//
//	resources/refs/<id>          mutable hash pointer
//	resources/meta/<id>.json     mutable sidecar (ResourceMeta)
//
// The shared object pool (objects/<aa>/<rest>) holds immutable content blobs.

// ResourceRefKey returns the storage key for a resource ref by ID.
func ResourceRefKey(id string) string {
	return fmt.Sprintf("resources/refs/%s", id)
}

// ResourceRefsPrefix returns the prefix covering all resource refs.
func ResourceRefsPrefix() string {
	return "resources/refs/"
}

// ResourceMetaKey returns the storage key for a resource's mutable sidecar.
func ResourceMetaKey(id string) string {
	return fmt.Sprintf("resources/meta/%s.json", id)
}
