package ollama

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/mwantia/forge/pkg/plugins"
)

// ollamaChatStream reads newline-delimited JSON chunks from the Ollama /api/chat response.
// Tool calls are buffered and returned on the final Done chunk.
type OllamaChatStream struct {
	id      string
	scanner *bufio.Scanner
	body    io.ReadCloser
	done    bool
}

func NewChatStream(body io.ReadCloser) *OllamaChatStream {
	return &OllamaChatStream{
		id:      uuid.New().String(),
		scanner: bufio.NewScanner(body),
		body:    body,
	}
}

func (s *OllamaChatStream) Recv() (*plugins.ChatChunk, error) {
	if s.done {
		return nil, io.EOF
	}

	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if line == "" {
			continue
		}

		var resp OllamaChatResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			return nil, fmt.Errorf("failed to decode stream chunk: %w", err)
		}

		// Skip empty non-terminal chunks — they are structural artifacts, not keepalives.
		if resp.Message.Content == "" && !resp.Done {
			continue
		}

		chunk := &plugins.ChatChunk{
			ID:    s.id,
			Role:  resp.Message.Role,
			Delta: resp.Message.Content,
			Done:  resp.Done,
		}

		if resp.Done {
			s.done = true
			for _, tc := range resp.Message.ToolCalls {
				chunk.ToolCalls = append(chunk.ToolCalls, plugins.ChatToolCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}

		return chunk, nil
	}

	if err := s.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

func (s *OllamaChatStream) Close() error {
	return s.body.Close()
}
