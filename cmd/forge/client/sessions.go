package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/mwantia/forge/internal/session"
	"github.com/mwantia/forge/pkg/plugins"
	"github.com/spf13/cobra"
)

func NewSessionsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage forge sessions",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *ForgeClient { return resolveClient(httpAddr, httpToken) }

	cmd.AddCommand(newSessionsListCmd(client))
	cmd.AddCommand(newSessionsCreateCmd(client))
	cmd.AddCommand(newSessionsGetCmd(client))
	cmd.AddCommand(newSessionsDeleteCmd(client))
	cmd.AddCommand(newSessionsToolsCmd(client))
	cmd.AddCommand(newSessionsMessagesCmd(client))
	cmd.AddCommand(newSessionsSendCmd(client))
	cmd.AddCommand(NewSessionsSandboxCommand(client))

	return cmd
}

func newSessionsListCmd(client func() *ForgeClient) *cobra.Command {
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Sessions []*session.Session `json:"sessions"`
			}
			path := fmt.Sprintf("/v1/sessions?limit=%d&offset=%d", limit, offset)
			if err := client().get(path, &resp); err != nil {
				return err
			}

			if len(resp.Sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tMODEL\tMESSAGES\tCREATED")
			for _, s := range resp.Sessions {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					s.ID,
					s.Name,
					s.Model,
					s.MessageCount,
					s.CreatedAt.Format(time.DateTime),
				)
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of sessions to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of sessions to skip")

	return cmd
}

func newSessionsCreateCmd(client func() *ForgeClient) *cobra.Command {
	var (
		name              string
		model             string
		memory            string
		tools             []string
		systemPrompt      string
		maxToolIterations int
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new session",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := session.CreateOptions{
				Name:              name,
				Model:             model,
				Memory:            memory,
				Tools:             tools,
				SystemPrompt:      systemPrompt,
				MaxToolIterations: maxToolIterations,
			}
			var sess session.Session
			if err := client().post("/v1/sessions", opts, &sess); err != nil {
				return err
			}
			printSession(&sess)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Session name (auto-generated if not set)")
	cmd.Flags().StringVar(&model, "model", "", "Model to use (format: provider/model, required)")
	cmd.Flags().StringVar(&memory, "memory", "", "Memory plugin name")
	cmd.Flags().StringSliceVar(&tools, "tools", nil, "Tool plugin names (comma-separated)")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "System prompt for the session")
	cmd.Flags().IntVar(&maxToolIterations, "max-tool-iterations", 0, "Maximum tool call iterations (0 = default)")
	cmd.MarkFlagRequired("model")

	return cmd
}

func newSessionsGetCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get session details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var sess session.Session
			if err := client().get("/v1/sessions/"+args[0], &sess); err != nil {
				return err
			}
			printSession(&sess)
			return nil
		},
	}
}

func newSessionsDeleteCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := client().delete("/v1/sessions/" + args[0]); err != nil {
				return err
			}
			fmt.Printf("Session %s deleted.\n", args[0])
			return nil
		},
	}
}

func newSessionsMessagesCmd(client func() *ForgeClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Manage messages in a session",
	}
	cmd.AddCommand(newSessionsMessagesListCmd(client))
	cmd.AddCommand(newSessionsMessagesViewCmd(client))
	cmd.AddCommand(newSessionsMessagesCompactCmd(client))
	return cmd
}

func newSessionsMessagesListCmd(client func() *ForgeClient) *cobra.Command {
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List messages in a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Messages []*session.Message `json:"messages"`
			}
			path := fmt.Sprintf("/v1/sessions/%s/messages?limit=%d&offset=%d", args[0], limit, offset)
			if err := client().get(path, &resp); err != nil {
				return err
			}

			if len(resp.Messages) == 0 {
				fmt.Println("No messages found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tCREATED\tROLE\tCONTENT")
			for _, m := range resp.Messages {
				content := m.Content
				if len(content) > 80 {
					content = content[:77] + "..."
				}
				content = strings.ReplaceAll(content, "\n", " ")
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					m.ID,
					m.CreatedAt.Format(time.DateTime),
					m.Role,
					content,
				)
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of messages to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of messages to skip")

	return cmd
}

func newSessionsMessagesViewCmd(client func() *ForgeClient) *cobra.Command {
	var noRender bool

	cmd := &cobra.Command{
		Use:   "view <session-id> <message-id>",
		Short: "View the content of a single message",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID, messageID := args[0], args[1]
			var msg session.Message
			if err := client().get("/v1/sessions/"+sessionID+"/messages/"+messageID, &msg); err != nil {
				return err
			}

			var sb strings.Builder
			sb.WriteString("---\n")
			fmt.Fprintf(&sb, "ID:      %s\n", msg.ID)
			fmt.Fprintf(&sb, "Role:    %s\n", msg.Role)
			fmt.Fprintf(&sb, "Created: %s\n", msg.CreatedAt.Format(time.DateTime))
			if len(msg.ToolCalls) > 0 {
				fmt.Fprintf(&sb, "Tools:   %d\n", len(msg.ToolCalls))
			}
			sb.WriteString("---\n")
			if msg.Content != "" {
				sb.WriteString("\n")
				sb.WriteString(msg.Content)
			}
			fmt.Println(renderMarkdown(sb.String(), noRender))
			return nil
		},
	}

	cmd.Flags().BoolVar(&noRender, "no-render", false, "Print raw content without markdown rendering")
	return cmd
}

func newSessionsMessagesCompactCmd(client func() *ForgeClient) *cobra.Command {
	var stripTools bool

	cmd := &cobra.Command{
		Use:   "compact <id>",
		Short: "Compact messages in a session by removing redundant entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stripTools {
				return fmt.Errorf("no compaction options specified (use --strip-tools)")
			}

			body := map[string]any{
				"strip_tools": stripTools,
			}
			var result session.CompactResult
			if err := client().post("/v1/sessions/"+args[0]+"/messages/compact", body, &result); err != nil {
				return err
			}

			fmt.Printf("Compacted: %d → %d messages (%d deleted)\n", result.Before, result.After, result.Deleted)
			return nil
		},
	}

	cmd.Flags().BoolVar(&stripTools, "strip-tools", false, "Remove tool result messages and intermediate assistant tool-call turns")

	return cmd
}

func newSessionsSendCmd(client func() *ForgeClient) *cobra.Command {
	var stream, noRender bool

	cmd := &cobra.Command{
		Use:   "send <id> <content>",
		Short: "Send a message to a session",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, content := args[0], args[1]
			body := map[string]any{
				"content": content,
				"stream":  stream,
			}

			if stream {
				return streamMessage(client(), id, body, noRender)
			}

			var result plugins.ChatResult
			if err := client().post("/v1/sessions/"+id+"/messages", body, &result); err != nil {
				return err
			}
			fmt.Println(renderMarkdown(result.Content, noRender))
			return nil
		},
	}

	cmd.Flags().BoolVar(&stream, "stream", false, "Stream the response as it arrives")
	cmd.Flags().BoolVar(&noRender, "no-render", false, "Print raw output without markdown rendering")

	return cmd
}

func newSessionsToolsCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "tools <id>",
		Short: "List all tools available to a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Tools []plugins.ToolCall `json:"tools"`
			}
			if err := client().get("/v1/sessions/"+args[0]+"/tools", &resp); err != nil {
				return err
			}

			if len(resp.Tools) == 0 {
				fmt.Println("No tools found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TOOL\tDESCRIPTION")
			for _, t := range resp.Tools {
				desc := t.Description
				if len(desc) > 70 {
					desc = desc[:67] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\n", t.Name, desc)
			}
			return w.Flush()
		},
	}
}

func streamMessage(c *ForgeClient, id string, body map[string]any, noRender bool) error {
	resp, err := c.postRaw("/v1/sessions/"+id+"/messages", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var buf strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk plugins.ChatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		buf.WriteString(chunk.Delta)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	fmt.Println(renderMarkdown(buf.String(), noRender))
	return nil
}

func renderMarkdown(content string, noRender bool) string {
	if noRender {
		return content
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return out
}

func printSession(s *session.Session) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID:\t%s\n", s.ID)
	fmt.Fprintf(w, "Name:\t%s\n", s.Name)
	fmt.Fprintf(w, "Model:\t%s\n", s.Model)
	if s.Memory != "" {
		fmt.Fprintf(w, "Memory:\t%s\n", s.Memory)
	}
	if len(s.Tools) > 0 {
		fmt.Fprintf(w, "Tools:\t%s\n", strings.Join(s.Tools, ", "))
	}
	if s.SystemPrompt != "" {
		prompt := s.SystemPrompt
		if len(prompt) > 60 {
			prompt = prompt[:57] + "..."
		}
		fmt.Fprintf(w, "System Prompt:\t%s\n", prompt)
	}
	fmt.Fprintf(w, "Max Tool Iterations:\t%s\n", strconv.Itoa(s.MaxToolIterations))
	fmt.Fprintf(w, "Messages:\t%d\n", s.MessageCount)
	fmt.Fprintf(w, "Created:\t%s\n", s.CreatedAt.Format(time.DateTime))
	fmt.Fprintf(w, "Updated:\t%s\n", s.UpdatedAt.Format(time.DateTime))
	w.Flush()
}
