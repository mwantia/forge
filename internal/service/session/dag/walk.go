package dag

import (
	"context"
	"fmt"
)

// WalkEntry pairs a message body with its hash. Walk returns these in
// chronological order (root first, HEAD last).
type WalkEntry struct {
	Hash    string
	Message *MessageObj
}

// Walk traverses a session's parent chain starting at refOrHash and
// returns the messages in chronological order.
//
//   - refOrHash may be a 64-char hex hash (resolved against the object
//     store directly) or a ref name (resolved via the ref store).
//   - offset skips that many of the most-recent messages before collecting.
//   - limit caps the number of returned entries; <=0 means "all".
//
// Result order is root-first. A missing parent ends the walk cleanly.
func Walk(ctx context.Context,
	objects *ObjectStore, refs *RefStore,
	sessionID, refOrHash string,
	limit, offset int,
) ([]WalkEntry, error) {

	start, err := resolveStart(ctx, objects, refs, sessionID, refOrHash)
	if err != nil {
		return nil, err
	}

	var collected []WalkEntry
	skipped := 0
	cur := start
	for cur != "" {
		m, err := objects.GetMessage(ctx, cur)
		if err != nil {
			if err == ErrNotFound {
				break
			}
			return nil, err
		}
		if skipped < offset {
			skipped++
			cur = m.ParentHash
			continue
		}
		if limit > 0 && len(collected) >= limit {
			break
		}
		collected = append(collected, WalkEntry{Hash: cur, Message: m})
		cur = m.ParentHash
	}

	// Reverse for chronological order.
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}
	return collected, nil
}

// resolveStart maps a ref-or-hash string to a starting message hash.
// 64-char hex strings are taken as object hashes; everything else is a
// ref name, looked up in refs.
func resolveStart(ctx context.Context,
	objects *ObjectStore, refs *RefStore,
	sessionID, refOrHash string,
) (string, error) {
	if refOrHash == "" {
		return "", fmt.Errorf("dag: empty ref or hash")
	}
	if isFullHash(refOrHash) {
		has, err := objects.Has(ctx, refOrHash)
		if err != nil {
			return "", err
		}
		if !has {
			return "", ErrNotFound
		}
		return refOrHash, nil
	}
	return refs.Read(ctx, sessionID, refOrHash)
}

func isFullHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}
