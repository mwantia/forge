package session

import (
	"context"

	domdag "github.com/mwantia/forge/internal/domain/dag"
)

// SessionManager is the narrow surface other services use to interact with
// session state.
type SessionManager interface {
	ResolveSession(ctx context.Context, idOrName string) (*SessionMetadata, error)
	LoadSession(ctx context.Context, id string) (*SessionMetadata, error)
	ListMessages(ctx context.Context, sessionID string, offset, limit int) ([]*Message, error)
	AppendMessage(ctx context.Context, sessionID string, msg *Message) (string, error)
	HeadHash(ctx context.Context, sessionID string) (string, error)
	PutPromptContext(ctx context.Context, p *domdag.PromptContext) (string, error)
	PutToolCatalog(ctx context.Context, t *domdag.ToolCatalog) (string, error)
	GetPromptContext(ctx context.Context, hash string) (*domdag.PromptContext, error)
	GetMessageObj(ctx context.Context, hash string) (*domdag.MessageObj, error)
	AppendMessageToRef(ctx context.Context, sessionID, ref string, msg *Message) (string, error)
	ListMessagesFromRef(ctx context.Context, sessionID, ref string, offset, limit int) ([]*Message, error)
	ListRefs(ctx context.Context, sessionID string) (map[string]string, error)
	ReadRef(ctx context.Context, sessionID, name string) (string, error)
	WriteRef(ctx context.Context, sessionID, name, hash string) error
	CASRef(ctx context.Context, sessionID, name, expected, next string) error
	DeleteRef(ctx context.Context, sessionID, name string) error
	RenameRef(ctx context.Context, sessionID, oldName, newName string) error
	PutMessageObj(ctx context.Context, obj *domdag.MessageObj) (string, error)
	ResolveMessageHash(ctx context.Context, sessionID, hashOrPrefix string) (string, error)
	CheckoutRef(ctx context.Context, sessionID, targetBranch string) error
	AccumulateDuration(ctx context.Context, sessionID string, ms int64)
}
