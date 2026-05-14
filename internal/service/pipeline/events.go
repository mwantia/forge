package pipeline

import (
	"encoding/json"
	"fmt"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
)

// PipelineEvent is the sealed interface for all pipeline events.
// Use a type switch to handle concrete types; new event types require adding a case.
// Transport adapters receive these via chan PipelineEvent and serialize with ToWireEvent.
type PipelineEvent interface {
	pipelineEvent()
}

// ChunkBoundary identifies what kind of boundary produced a ChunkEvent.
// Consumers can use this to decide whether to render incrementally (token/sentence)
// or wait for a self-contained unit (block/final) before invoking a markdown renderer.
type ChunkBoundary string

const (
	ChunkBoundaryToken    ChunkBoundary = "token"
	ChunkBoundarySentence ChunkBoundary = "sentence"
	ChunkBoundaryBlock    ChunkBoundary = "block"
	ChunkBoundaryFinal    ChunkBoundary = "final"
)

// ChunkEvent carries a text chunk emitted at a boundary chosen by the server's
// pipeline.output policy. Text is additive: concatenating all non-final chunks
// reproduces the full assistant response.
type ChunkEvent struct {
	Text     string        `json:"text"`
	Thinking string        `json:"thinking,omitempty"`
	Boundary ChunkBoundary `json:"boundary"`
}

// ToolCallEvent signals the LLM requested a tool call.
type ToolCallEvent struct {
	CallID string         `json:"call_id,omitempty"`
	Name   string         `json:"name"`
	Args   map[string]any `json:"args,omitempty"`
}

// ToolResultEvent carries the result of an executed tool call.
type ToolResultEvent struct {
	CallID  string `json:"call_id,omitempty"`
	Name    string `json:"name"`
	Result  any    `json:"result"`
	IsError bool   `json:"is_error,omitempty"`
}

// ErrorEvent signals a pipeline or provider error. Does not necessarily close the stream.
type ErrorEvent struct {
	Message string `json:"message"`
}

// DoneEvent signals the pipeline finished cleanly. Always the last event on the channel.
type DoneEvent struct {
	Usage    *sdkplugins.TokenUsage `json:"usage,omitempty"`
	Metadata map[string]any         `json:"metadata,omitempty"`
}

func (ChunkEvent) pipelineEvent()      {}
func (ToolCallEvent) pipelineEvent()   {}
func (ToolResultEvent) pipelineEvent() {}
func (ErrorEvent) pipelineEvent()      {}
func (DoneEvent) pipelineEvent()       {}

// WireEvent is the JSON-serializable envelope used at transport adapter boundaries.
// The Type field identifies the event; Data holds the marshaled concrete event payload.
type WireEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty" swaggertype:"object"`
}

// ToWireEvent converts a typed PipelineEvent to a WireEvent for NDJSON / SSE / WS output.
func ToWireEvent(ev PipelineEvent) (WireEvent, error) {
	var evType string
	switch ev.(type) {
	case ChunkEvent:
		evType = "chunk"
	case ToolCallEvent:
		evType = "tool_call"
	case ToolResultEvent:
		evType = "tool_result"
	case ErrorEvent:
		evType = "error"
	case DoneEvent:
		evType = "done"
	default:
		return WireEvent{}, fmt.Errorf("unknown pipeline event type: %T", ev)
	}

	d, err := json.Marshal(ev)
	if err != nil {
		return WireEvent{}, fmt.Errorf("failed to marshal %T: %w", ev, err)
	}

	return WireEvent{
		Type: evType,
		Data: d,
	}, nil
}
