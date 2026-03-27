package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mwantia/forge/pkg/plugins"
)

type pipelineItem struct {
	chunk *plugins.ChatChunk
	err   error
}

// pipelineStream wraps a channel of pipelineItems as a plugins.ChatStream.
type pipelineStream struct {
	ch <-chan pipelineItem
}

func (s *pipelineStream) Recv() (*plugins.ChatChunk, error) {
	item, ok := <-s.ch
	if !ok {
		return nil, io.EOF
	}
	if item.err != nil {
		return nil, item.err
	}
	return item.chunk, nil
}

func (s *pipelineStream) Close() error {
	for range s.ch {
	}
	return nil
}

// runPipeline runs the full dispatch loop in a goroutine:
//  1. Calls the provider and collects the response.
//  2. If the response has tool calls, executes them and loops.
//  3. When the final text response arrives (no tool calls), saves the assistant
//     message and emits the content as chunks to out.
func (m *Manager) runPipeline(
	ctx context.Context,
	sess *Session,
	sessionID string,
	messages []plugins.ChatMessage,
	toolDefs []plugins.ToolCall,
	toolsMap map[string]plugins.ToolsPlugin,
	out chan<- pipelineItem,
) {
	defer close(out)

	emittedContent := false

	for i := range sess.MaxToolIterations {
		stream, err := m.registry.Provider().Chat(ctx, sess.Model, messages, toolDefs)
		if err != nil {
			out <- pipelineItem{err: fmt.Errorf("provider error (iteration %d): %w", i+1, err)}
			return
		}

		result, err := plugins.CollectStream(stream)
		if err != nil {
			out <- pipelineItem{err: fmt.Errorf("stream error (iteration %d): %w", i+1, err)}
			return
		}

		if len(result.ToolCalls) == 0 {
			// Final text response — persist and emit to the client.
			m.persistAssistantMessage(sessionID, result.Content, nil)
			m.refreshMessageCount(sess, sessionID)
			if emittedContent && result.Content != "" {
				emitSeparator(out, result)
			}
			replayAsStream(out, result)
			return
		}

		// Intermediate turn: the model wants to call tools.
		m.persistAssistantMessage(sessionID, result.Content, result.ToolCalls)

		// Stream the intermediate assistant text to the client now, before tool execution.
		// The client sees this content while tools run in the background.
		if result.Content != "" {
			if emittedContent {
				emitSeparator(out, result)
			}
			emitContent(out, result)
			emittedContent = true
		}

		assistantMsg := plugins.ChatMessage{
			Role:    result.Role,
			Content: result.Content,
			ToolCalls: &plugins.ChatMessageToolCalls{
				ToolCalls: result.ToolCalls,
			},
		}
		messages = append(messages, assistantMsg)

		for _, tc := range result.ToolCalls {
			resultContent, isError := m.executeToolCall(ctx, toolsMap, tc)
			m.persistToolMessage(sessionID, tc, resultContent, isError)
			messages = append(messages, plugins.ChatMessage{
				Role:    "tool",
				Content: resultContent,
			})
		}
	}

	out <- pipelineItem{err: fmt.Errorf("max tool iterations (%d) reached without a final response", sess.MaxToolIterations)}
}

func (m *Manager) executeToolCall(ctx context.Context, toolsMap map[string]plugins.ToolsPlugin, tc plugins.ChatToolCall) (string, bool) {
	tp, ok := toolsMap[tc.Name]
	if !ok {
		m.log.Warn("Tool not found during execution", "tool", tc.Name)
		return fmt.Sprintf("error: tool '%s' not found", tc.Name), true
	}

	// Strip the "pluginName__" prefix to get the bare tool name.
	realName := tc.Name
	if idx := strings.Index(tc.Name, "__"); idx >= 0 {
		realName = tc.Name[idx+2:]
	}

	resp, err := tp.Execute(ctx, plugins.ExecuteRequest{
		Tool:      realName,
		Arguments: tc.Arguments,
		CallID:    tc.ID,
	})
	if err != nil {
		m.log.Warn("Tool execution error", "tool", tc.Name, "error", err)
		return fmt.Sprintf("error: %v", err), true
	}

	b, _ := json.Marshal(resp.Result)
	return string(b), resp.IsError
}

func (m *Manager) persistAssistantMessage(sessionID, content string, toolCalls []plugins.ChatToolCall) {
	msg := &Message{
		ID:        uuid.New().String(),
		Role:      "assistant",
		Content:   content,
		CreatedAt: time.Now(),
	}
	for _, tc := range toolCalls {
		msg.ToolCalls = append(msg.ToolCalls, ToolCallEntry{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		})
	}
	if err := m.store.SaveMessage(sessionID, msg); err != nil {
		m.log.Error("Failed to save assistant message", "error", err)
	}
}

func (m *Manager) persistToolMessage(sessionID string, tc plugins.ChatToolCall, result string, isError bool) {
	msg := &Message{
		ID:      uuid.New().String(),
		Role:    "tool",
		Content: result,
		ToolCalls: []ToolCallEntry{{
			ID:      tc.ID,
			Name:    tc.Name,
			Result:  result,
			IsError: isError,
		}},
		CreatedAt: time.Now(),
	}
	if err := m.store.SaveMessage(sessionID, msg); err != nil {
		m.log.Error("Failed to save tool message", "error", err)
	}
}

// refreshMessageCount reloads and updates the session's message count and UpdatedAt.
func (m *Manager) refreshMessageCount(_ *Session, sessionID string) {
	stored, err := m.store.LoadSession(sessionID)
	if err != nil {
		return
	}
	stored.MessageCount = m.store.CountMessages(sessionID)
	stored.UpdatedAt = time.Now()
	if err := m.store.SaveSession(stored); err != nil {
		m.log.Error("Failed to update session metadata", "error", err)
	}
}

// emitSeparator sends a "\n\n" chunk to visually separate consecutive assistant messages.
func emitSeparator(out chan<- pipelineItem, result *plugins.ChatResult) {
	out <- pipelineItem{chunk: &plugins.ChatChunk{
		ID:    result.ID,
		Role:  result.Role,
		Delta: "\n\n",
	}}
}

// emitContent splits result content into small chunks and sends them to out
// without a Done marker. Used for intermediate assistant messages during tool
// iterations so the client sees them before tool execution completes.
func emitContent(out chan<- pipelineItem, result *plugins.ChatResult) {
	const chunkSize = 64
	content := []rune(result.Content)
	for len(content) > 0 {
		n := min(chunkSize, len(content))
		out <- pipelineItem{chunk: &plugins.ChatChunk{
			ID:    result.ID,
			Role:  result.Role,
			Delta: string(content[:n]),
		}}
		content = content[n:]
	}
}

// replayAsStream emits the result content as chunks followed by a Done chunk.
// Used for the final assistant response after all tool iterations complete.
func replayAsStream(out chan<- pipelineItem, result *plugins.ChatResult) {
	emitContent(out, result)
	out <- pipelineItem{chunk: &plugins.ChatChunk{
		ID:       result.ID,
		Role:     result.Role,
		Done:     true,
		Metadata: result.Metadata,
	}}
}
