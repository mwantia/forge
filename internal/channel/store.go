package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mwantia/forge/internal/storage"
)

// ChannelBinding records which Forge session is currently bound to a channel.
type ChannelBinding struct {
	SessionID   string    `json:"session_id"`
	SessionName string    `json:"session_name"`
	BoundAt     time.Time `json:"bound_at"`
}

// BindingStore persists per-channel session bindings so they survive service
// restarts. Each plugin gets its own entry at:
//
//	channels/{pluginName}/bindings.json
//
// Bindings are cached in memory after the initial Load to avoid repeated
// backend round trips on every read.
type BindingStore struct {
	mu       sync.RWMutex
	backend  storage.Backend
	plugin   string
	bindings map[string]*ChannelBinding // channelID → binding
}

func newBindingStore(backend storage.Backend, pluginName string) *BindingStore {
	return &BindingStore{
		backend:  backend,
		plugin:   pluginName,
		bindings: make(map[string]*ChannelBinding),
	}
}

func (s *BindingStore) bindingsKey() string {
	return "channels/" + s.plugin + "/bindings.json"
}

// Load reads persisted bindings from the backend. Called during dispatcher
// Setup. A missing key is not an error — it just means no bindings exist yet.
func (s *BindingStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.backend.GetRaw(context.Background(), s.bindingsKey())
	if err != nil {
		return fmt.Errorf("failed to read bindings: %w", err)
	}
	if data == nil {
		return nil
	}
	return json.Unmarshal(data, &s.bindings)
}

// All returns a copy of all current bindings keyed by channelID.
func (s *BindingStore) All() map[string]*ChannelBinding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*ChannelBinding, len(s.bindings))
	for k, v := range s.bindings {
		out[k] = v
	}
	return out
}

// Get returns the binding for a channelID, if one exists.
func (s *BindingStore) Get(channelID string) (*ChannelBinding, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bindings[channelID]
	return b, ok
}

// Set binds a channelID to a session and persists to the backend.
func (s *BindingStore) Set(channelID string, b *ChannelBinding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindings[channelID] = b
	return s.save()
}

// Delete removes the binding for a channelID and persists to the backend.
func (s *BindingStore) Delete(channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.bindings, channelID)
	return s.save()
}

// save writes the current bindings to the backend. Must be called under mu.Lock.
func (s *BindingStore) save() error {
	data, err := json.MarshalIndent(s.bindings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bindings: %w", err)
	}
	return s.backend.PutRaw(context.Background(), s.bindingsKey(), data)
}
