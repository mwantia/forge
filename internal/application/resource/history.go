package resource

import (
	"github.com/sergi/go-diff/diffmatchpatch"
)

// computeDiff returns a Myers diff between oldText and newText.
// Returns (delta string, pretty-printed text).
func computeDiff(oldText, newText string) (string, string) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldText, newText, false)
	dmp.DiffCleanupSemantic(diffs)
	return dmp.DiffToDelta(diffs), dmp.DiffPrettyText(diffs)
}
