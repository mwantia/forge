package dag

import (
	"fmt"
	"strings"
	"time"
)

// Resource storage layout (mirrors the session layout):
//
//	resources/<namespace>/refs/<name>                        mutable hash pointer
//	resources/<namespace>/log/<020d-unix_nano>_<hash>.json  ResourceMeta sidecar
//
// The shared object pool (objects/<aa>/<rest>) is used for content blobs,
// identical to session MessageObjs.

func resourceBucket(namespace string) string {
	return "resources/" + strings.Trim(namespace, "/")
}

// ResourceRefKey returns the storage key for a named resource ref.
func ResourceRefKey(namespace, name string) string {
	return fmt.Sprintf("%s/refs/%s", resourceBucket(namespace), name)
}

// ResourceRefsPrefix returns the prefix for all refs under namespace.
func ResourceRefsPrefix(namespace string) string {
	return resourceBucket(namespace) + "/refs/"
}

// ResourceLogPrefix returns the prefix for all sidecar log entries under namespace.
func ResourceLogPrefix(namespace string) string {
	return resourceBucket(namespace) + "/log/"
}

// ResourceLogKey returns the storage key for a ResourceMeta sidecar.
// The zero-padded unix-nano prefix gives chronological order from a plain ListEntry.
func ResourceLogKey(namespace string, createdAt time.Time, hash string) string {
	return fmt.Sprintf("%s%020d_%s.json", ResourceLogPrefix(namespace), createdAt.UnixNano(), hash)
}
