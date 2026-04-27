package pipeline

import (
	"context"
	"math/rand/v2"
	"strings"
	"time"
	"unicode"
)

// chunker buffers provider token deltas and emits ChunkEvents at boundaries
// chosen by the configured output policy. It also honors code-fence atomicity
// and an optional max-chunk-byte cap, and applies pacing between emissions.
//
// The zero value is not usable; construct via newChunker.
type chunker struct {
	out    chan<- PipelineEvent
	policy resolvedOutput
	buf    strings.Builder
	// inFence is true when the running buffer contains an unclosed ``` fence.
	inFence bool
}

func newChunker(out chan<- PipelineEvent, policy resolvedOutput) *chunker {
	return &chunker{out: out, policy: policy}
}

// push adds a delta to the buffer and emits any chunks whose boundary has been
// reached. Thinking is forwarded as a token-boundary chunk regardless of mode —
// reasoning is out-of-band and doesn't belong in assembled markdown.
func (c *chunker) push(ctx context.Context, delta, thinking string) error {
	if thinking != "" {
		if err := c.emit(ctx, ChunkEvent{Thinking: thinking, Boundary: ChunkBoundaryToken}); err != nil {
			return err
		}
	}
	if delta == "" {
		return nil
	}

	c.buf.WriteString(delta)
	c.updateFenceState(delta)

	for {
		flushed, err := c.tryEmit(ctx)
		if err != nil {
			return err
		}
		if !flushed {
			return nil
		}
	}
}

// flush emits whatever remains in the buffer as the final chunk. Called when
// the provider signals Done (no more deltas coming for this turn).
func (c *chunker) flush(ctx context.Context) error {
	text := c.buf.String()
	c.buf.Reset()
	c.inFence = false
	if text == "" {
		return nil
	}
	return c.emit(ctx, ChunkEvent{Text: text, Boundary: ChunkBoundaryFinal})
}

// tryEmit inspects the buffer for a boundary matching the configured mode and,
// if found, emits a ChunkEvent carrying the prefix up to that boundary.
// Returns true when an emission occurred (caller should loop).
func (c *chunker) tryEmit(ctx context.Context) (bool, error) {
	switch c.policy.Mode {
	case OutputModeFinal:
		// Nothing until flush.
		return c.emitMaxBytes(ctx)
	case OutputModeToken:
		text := c.buf.String()
		if text == "" {
			return false, nil
		}
		c.buf.Reset()
		return true, c.emit(ctx, ChunkEvent{Text: text, Boundary: ChunkBoundaryToken})
	case OutputModeSentence:
		if ok, err := c.emitAtIndex(ctx, findSentenceEnd(c.buf.String(), c.inFence), ChunkBoundarySentence); ok || err != nil {
			return ok, err
		}
		return c.emitMaxBytes(ctx)
	default: // OutputModeBlock
		if ok, err := c.emitAtIndex(ctx, findBlockEnd(c.buf.String(), c.inFence, c.policy.CodeFence == CodeFenceAtomic), ChunkBoundaryBlock); ok || err != nil {
			return ok, err
		}
		return c.emitMaxBytes(ctx)
	}
}

// emitAtIndex emits buf[:idx] (plus any trailing separator) as a chunk with the
// given boundary kind when idx >= 0. Returns (true, err) on emission.
func (c *chunker) emitAtIndex(ctx context.Context, idx int, boundary ChunkBoundary) (bool, error) {
	if idx < 0 {
		return false, nil
	}
	text := c.buf.String()[:idx]
	remainder := c.buf.String()[idx:]
	c.buf.Reset()
	c.buf.WriteString(remainder)
	// Fence state may flip once we drop content; recompute from remainder.
	c.inFence = countFences(remainder)%2 == 1
	if text == "" {
		return false, nil
	}
	return true, c.emit(ctx, ChunkEvent{Text: text, Boundary: boundary})
}

// emitMaxBytes enforces the MaxChunkBytes cap when set. Emits as a block
// boundary since the cut point is arbitrary — never mid-fence in atomic mode.
func (c *chunker) emitMaxBytes(ctx context.Context) (bool, error) {
	cap := c.policy.MaxChunkBytes
	if cap <= 0 || c.buf.Len() < cap {
		return false, nil
	}
	if c.inFence && c.policy.CodeFence == CodeFenceAtomic {
		return false, nil
	}
	text := c.buf.String()[:cap]
	remainder := c.buf.String()[cap:]
	c.buf.Reset()
	c.buf.WriteString(remainder)
	c.inFence = countFences(remainder)%2 == 1
	return true, c.emit(ctx, ChunkEvent{Text: text, Boundary: ChunkBoundaryBlock})
}

// emit paces the chunk (if pacing enabled) then sends it on the output channel.
// Respects ctx cancellation during the pacing sleep.
func (c *chunker) emit(ctx context.Context, ev ChunkEvent) error {
	if d := c.paceDelay(ev.Text); d > 0 {
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.out <- ev:
		return nil
	}
}

// paceDelay returns the sleep duration before emitting a chunk of text.
// Computes len/cps seconds, applies ±jitter, and adds a punctuation pause if
// the chunk ends on sentence-terminating punctuation.
func (c *chunker) paceDelay(text string) time.Duration {
	p := c.policy.Pacing
	if !p.Enabled || text == "" || p.CPS <= 0 {
		return 0
	}
	base := time.Duration(float64(len(text)) / float64(p.CPS) * float64(time.Second))
	if p.Jitter > 0 {
		// rand in [-jitter, +jitter] then scale base.
		scale := 1 + (rand.Float64()*2-1)*p.Jitter
		if scale < 0 {
			scale = 0
		}
		base = time.Duration(float64(base) * scale)
	}
	if p.PunctuationPauseMs > 0 && endsWithSentenceTerminator(text) {
		base += time.Duration(p.PunctuationPauseMs) * time.Millisecond
	}
	return base
}

// updateFenceState flips inFence for each ``` occurrence introduced by delta.
func (c *chunker) updateFenceState(delta string) {
	if n := countFences(delta); n%2 == 1 {
		c.inFence = !c.inFence
	}
}

// --- boundary detection ---

// findBlockEnd returns the index in s at which a markdown block boundary is
// considered complete, or -1 if no boundary is ready. A block ends at the
// second newline of a blank-line separator. When atomic is true and the cut
// would fall inside an unclosed fence, returns -1.
func findBlockEnd(s string, inFenceStart, atomic bool) int {
	inFence := inFenceStart
	for i := 0; i < len(s); i++ {
		// Track fence transitions as we scan so we don't split inside one.
		if strings.HasPrefix(s[i:], "```") {
			inFence = !inFence
			i += 2
			continue
		}
		if s[i] != '\n' {
			continue
		}
		// Look for "\n\n" (or "\n\r\n") — a paragraph separator.
		j := i + 1
		for j < len(s) && (s[j] == '\r') {
			j++
		}
		if j < len(s) && s[j] == '\n' {
			if atomic && inFence {
				continue
			}
			return j + 1
		}
	}
	return -1
}

// findSentenceEnd returns the index past a sentence terminator (. ! ?) followed
// by whitespace, or -1 if none. Never splits inside a fenced code block.
func findSentenceEnd(s string, inFenceStart bool) int {
	inFence := inFenceStart
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], "```") {
			inFence = !inFence
			i += 2
			continue
		}
		if inFence {
			continue
		}
		r := rune(s[i])
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		j := i + 1
		if j >= len(s) {
			return -1 // need the following rune to confirm
		}
		if unicode.IsSpace(rune(s[j])) {
			return j + 1
		}
	}
	return -1
}

// countFences returns the number of ``` fence markers in s.
func countFences(s string) int {
	n := 0
	for i := 0; i+3 <= len(s); i++ {
		if s[i] == '`' && s[i+1] == '`' && s[i+2] == '`' {
			n++
			i += 2
		}
	}
	return n
}

func endsWithSentenceTerminator(s string) bool {
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	if s == "" {
		return false
	}
	switch s[len(s)-1] {
	case '.', '!', '?':
		return true
	}
	return false
}
