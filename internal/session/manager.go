package session

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge-sdk/pkg/random"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/internal/sandbox"
	"github.com/mwantia/forge/internal/storage"
)

const defaultMaxToolIterations = 10

// Manager handles session lifecycle and message dispatch.
type SessionManager struct {
	log            hclog.Logger             `fabric:"logger:session"`
	registry       *registry.PluginRegistry `fabric:"inject"`
	sandboxManager *sandbox.SandboxManager  `fabric:"inject"`
	backend        storage.Backend          `fabric:"inject"`
}

// CreateOptions is the request body for POST /v1/sessions.
type CreateOptions struct {
	Name              string   `json:"name,omitempty"`
	Title             string   `json:"title,omitempty"`
	Description       string   `json:"description,omitempty"`
	Parent            string   `json:"parent,omitempty"`
	Model             string   `json:"model"               binding:"required"`
	Memory            string   `json:"memory,omitempty"`
	Tools             []string `json:"tools,omitempty"`
	MaxToolIterations int      `json:"max_tool_iterations,omitempty"`
	SystemPrompt      string   `json:"system_prompt,omitempty"`
}

func (m *SessionManager) Create(opts CreateOptions) (*Session, error) {
	if opts.MaxToolIterations <= 0 {
		opts.MaxToolIterations = defaultMaxToolIterations
	}
	name := opts.Name
	if name == "" {
		name = m.generateUniqueName()
	}
	now := time.Now()
	sess := &Session{
		ID:                random.GenerateNewID(),
		Name:              name,
		Title:             opts.Title,
		Description:       opts.Description,
		Parent:            opts.Parent,
		Model:             opts.Model,
		Memory:            opts.Memory,
		Tools:             opts.Tools,
		MaxToolIterations: opts.MaxToolIterations,
		SystemPrompt:      opts.SystemPrompt,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := m.saveSession(sess); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}
	return sess, nil
}

func (m *SessionManager) Get(id string) (*Session, error) {
	sess, err := m.loadSession(id)
	if err == nil {
		return sess, nil
	}
	// Fall back to name lookup
	sess, err = m.findByName(id)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	if sess == nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return sess, nil
}

func (m *SessionManager) List(opts ListOptions) ([]*Session, error) {
	return m.listSessions(opts)
}

func (m *SessionManager) Delete(id string) error {
	sess, err := m.Get(id)
	if err != nil {
		return err
	}
	return m.deleteSession(sess.ID)
}

// UpdateMetaOptions holds the fields that can be updated on an existing session.
type UpdateMetaOptions struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (m *SessionManager) UpdateMeta(id string, opts UpdateMetaOptions) (*Session, error) {
	sess, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	if opts.Title != nil {
		sess.Title = *opts.Title
	}
	if opts.Description != nil {
		sess.Description = *opts.Description
	}
	sess.UpdatedAt = time.Now()
	if err := m.saveSession(sess); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}
	return sess, nil
}

// ListTools returns all tools available to a session, including built-in session tools.
func (m *SessionManager) ListTools(ctx context.Context, id string) ([]plugins.ToolCall, error) {
	sess, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	_, toolDefs, err := m.resolveAllTools(ctx, sess)
	if err != nil {
		return nil, err
	}
	return toolDefs, nil
}

func (m *SessionManager) GetMessage(sessionID, messageID string) (*Message, error) {
	sess, err := m.Get(sessionID)
	if err != nil {
		return nil, err
	}
	return m.getMessage(sess.ID, messageID)
}

func (m *SessionManager) GetMessages(id string, limit, offset int) ([]*Message, error) {
	sess, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	return m.listMessages(sess.ID, limit, offset)
}

// Dispatch saves the user message, runs the full pipeline, and returns a stream
// of the final assistant response. The stream must always be closed by the caller.
func (m *SessionManager) Dispatch(ctx context.Context, sessionID, content string) (plugins.ChatStream, error) {
	sess, err := m.Get(sessionID)
	if err != nil {
		return nil, err
	}

	history, err := m.listMessages(sess.ID, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load message history: %w", err)
	}

	messages := buildChatMessages(sess, history, content)

	toolsMap, toolDefs, err := m.resolveAllTools(ctx, sess)
	if err != nil {
		return nil, err
	}

	userMsg := &Message{
		ID:        random.GenerateNewID(),
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}
	if err := m.saveMessage(sess.ID, userMsg); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	out := make(chan pipelineItem, 256)
	go m.runPipeline(ctx, sess, sess.ID, messages, toolDefs, toolsMap, out)
	return &pipelineStream{ch: out}, nil
}

type CompactOptions struct {
	StripTools bool `json:"strip_tools"`
}

type CompactResult struct {
	Before  int `json:"before"`
	After   int `json:"after"`
	Deleted int `json:"deleted"`
}

func (m *SessionManager) Compact(id string, opts CompactOptions) (*CompactResult, error) {
	sess, err := m.Get(id)
	if err != nil {
		return nil, err
	}

	before := m.countMessages(sess.ID)

	deleted, err := m.compactMessages(sess.ID, opts.StripTools)
	if err != nil {
		return nil, fmt.Errorf("compaction failed: %w", err)
	}

	after := m.countMessages(sess.ID)

	sess.MessageCount = after
	sess.UpdatedAt = time.Now()
	if err := m.saveSession(sess); err != nil {
		m.log.Error("Failed to update session metadata after compact", "error", err)
	}

	return &CompactResult{Before: before, After: after, Deleted: deleted}, nil
}

// resolveAllTools resolves plugin tools for the session and appends the built-in
// session management tools (agent__session_*) and sandbox tools (agent__sandbox_*)
// when a sandbox manager is wired in.
func (m *SessionManager) resolveAllTools(ctx context.Context, sess *Session) (map[string]plugins.ToolsPlugin, []plugins.ToolCall, error) {
	toolsMap, toolDefs, err := m.resolveTools(ctx, sess.Tools)
	if err != nil {
		return nil, nil, err
	}

	sp := &SessionToolsPlugin{
		manager:   m,
		sessionID: sess.ID,
	}
	resp, _ := sp.ListTools(ctx, plugins.ListToolsFilter{})
	for _, def := range resp.Tools {
		prefixed := "agent__" + def.Name
		toolDefs = append(toolDefs, plugins.ToolCall{
			Name:        prefixed,
			Description: def.Description,
			Parameters:  def.Parameters,
		})
		toolsMap[prefixed] = sp
	}

	if m.sandboxManager != nil {
		sbp := m.sandboxManager.NewToolsPlugin(sess.ID)
		sbResp, _ := sbp.ListTools(ctx, plugins.ListToolsFilter{})
		for _, def := range sbResp.Tools {
			prefixed := "agent__" + def.Name
			toolDefs = append(toolDefs, plugins.ToolCall{
				Name:        prefixed,
				Description: def.Description,
				Parameters:  def.Parameters,
			})
			toolsMap[prefixed] = sbp
		}
	}

	return toolsMap, toolDefs, nil
}

func (m *SessionManager) resolveTools(ctx context.Context, names []string) (map[string]plugins.ToolsPlugin, []plugins.ToolCall, error) {
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
