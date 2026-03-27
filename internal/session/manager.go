package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/pkg/plugins"
)

const defaultMaxToolIterations = 10

// Manager handles session lifecycle and message dispatch.
type Manager struct {
	log      hclog.Logger
	store    *FileStore
	registry *registry.PluginRegistry
}

func NewManager(log hclog.Logger, dataDir string, reg *registry.PluginRegistry) *Manager {
	return &Manager{
		log:      log.Named("session"),
		store:    NewFileStore(dataDir),
		registry: reg,
	}
}

// CreateOptions is the request body for POST /v1/sessions.
type CreateOptions struct {
	Model             string   `json:"model"               binding:"required"`
	Memory            string   `json:"memory,omitempty"`
	Tools             []string `json:"tools,omitempty"`
	MaxToolIterations int      `json:"max_tool_iterations,omitempty"`
	SystemPrompt      string   `json:"system_prompt,omitempty"`
}

func (m *Manager) Create(opts CreateOptions) (*Session, error) {
	if opts.MaxToolIterations <= 0 {
		opts.MaxToolIterations = defaultMaxToolIterations
	}
	now := time.Now()
	sess := &Session{
		ID:                uuid.New().String(),
		Model:             opts.Model,
		Memory:            opts.Memory,
		Tools:             opts.Tools,
		MaxToolIterations: opts.MaxToolIterations,
		SystemPrompt:      opts.SystemPrompt,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := m.store.SaveSession(sess); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}
	return sess, nil
}

func (m *Manager) Get(id string) (*Session, error) {
	sess, err := m.store.LoadSession(id)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	return sess, nil
}

func (m *Manager) List(limit, offset int) ([]*Session, error) {
	return m.store.ListSessions(limit, offset)
}

func (m *Manager) Delete(id string) error {
	return m.store.DeleteSession(id)
}

// ListTools returns all tools available to a session, namespaced as "plugin__tool".
func (m *Manager) ListTools(ctx context.Context, id string) ([]plugins.ToolCall, error) {
	sess, err := m.store.LoadSession(id)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	_, toolDefs, err := m.resolveTools(ctx, sess.Tools)
	if err != nil {
		return nil, err
	}
	return toolDefs, nil
}

func (m *Manager) GetMessages(id string, limit, offset int) ([]*Message, error) {
	if _, err := m.store.LoadSession(id); err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	return m.store.ListMessages(id, limit, offset)
}

// Dispatch saves the user message, runs the full pipeline, and returns a stream
// of the final assistant response. The stream must always be closed by the caller.
func (m *Manager) Dispatch(ctx context.Context, sessionID, content string) (plugins.ChatStream, error) {
	sess, err := m.store.LoadSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	history, err := m.store.ListMessages(sessionID, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load message history: %w", err)
	}

	messages := buildChatMessages(sess, history, content)

	toolsMap, toolDefs, err := m.resolveTools(ctx, sess.Tools)
	if err != nil {
		return nil, err
	}

	userMsg := &Message{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}
	if err := m.store.SaveMessage(sessionID, userMsg); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	out := make(chan pipelineItem, 256)
	go m.runPipeline(ctx, sess, sessionID, messages, toolDefs, toolsMap, out)
	return &pipelineStream{ch: out}, nil
}

func (m *Manager) resolveTools(ctx context.Context, names []string) (map[string]plugins.ToolsPlugin, []plugins.ToolCall, error) {
	toolsMap := make(map[string]plugins.ToolsPlugin)
	var toolDefs []plugins.ToolCall

	for _, name := range names {
		tp, err := m.registry.GetToolsPlugin(ctx, name)
		if err != nil {
			return nil, nil, fmt.Errorf("tools plugin '%s' not found: %w", name, err)
		}
		resp, err := tp.ListTools(ctx, plugins.ListToolsFilter{})
		if err != nil {
			m.log.Warn("Failed to list tools from plugin", "plugin", name, "error", err)
			continue
		}
		for _, def := range resp.Tools {
			prefixed := name + "__" + def.Name
			toolDefs = append(toolDefs, plugins.ToolCall{
				Name:        prefixed,
				Description: def.Description,
				Parameters:  def.Parameters,
			})
			toolsMap[prefixed] = tp
		}
	}
	return toolsMap, toolDefs, nil
}

func buildChatMessages(sess *Session, history []*Message, newContent string) []plugins.ChatMessage {
	var messages []plugins.ChatMessage

	if sess.SystemPrompt != "" {
		messages = append(messages, plugins.ChatMessage{
			Role:    "system",
			Content: sess.SystemPrompt,
		})
	}

	for _, msg := range history {
		cm := plugins.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			calls := make([]plugins.ChatToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				calls = append(calls, plugins.ChatToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
			cm.ToolCalls = &plugins.ChatMessageToolCalls{ToolCalls: calls}
		}
		messages = append(messages, cm)
	}

	messages = append(messages, plugins.ChatMessage{
		Role:    "user",
		Content: newContent,
	})

	return messages
}
