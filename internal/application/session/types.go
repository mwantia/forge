package session

import domsession "github.com/mwantia/forge/internal/domain/session"

type (
	SessionMetadata = domsession.SessionMetadata
	SessionQuery    = domsession.SessionQuery
	Message         = domsession.Message
	MessageToolCall = domsession.MessageToolCall
	PluginConfig    = domsession.PluginConfig
)

// PluginConfigsFromNames converts plugin names from HTTP/CLI input into
// PluginConfig entries. Listed plugins start enabled — specifying a list at
// session creation is an explicit scope restriction; everything outside it is blocked.
func PluginConfigsFromNames(names []string) []PluginConfig {
	if len(names) == 0 {
		return nil
	}

	out := make([]PluginConfig, len(names))
	for i, n := range names {
		out[i] = PluginConfig{
			Name: n, 
			Enabled: true,
		}
	}
	
	return out
}
