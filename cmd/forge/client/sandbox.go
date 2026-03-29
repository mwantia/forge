package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mwantia/forge/internal/sandbox"
	"github.com/mwantia/forge/pkg/plugins"
	"github.com/spf13/cobra"
)

// NewSessionsSandboxCommand returns the `forge sessions sandbox` subcommand group.
// It is registered on the sessions command so the session relationship is visible.
func NewSessionsSandboxCommand(client func() *ForgeClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage sandboxes for a session",
	}

	cmd.AddCommand(newSandboxListCmd(client))
	cmd.AddCommand(newSandboxCreateCmd(client))
	cmd.AddCommand(newSandboxGetCmd(client))
	cmd.AddCommand(newSandboxDeleteCmd(client))
	cmd.AddCommand(newSandboxExecCmd(client))
	cmd.AddCommand(newSandboxCopyInCmd(client))
	cmd.AddCommand(newSandboxCopyOutCmd(client))
	cmd.AddCommand(newSandboxStatCmd(client))
	cmd.AddCommand(newSandboxReadCmd(client))

	return cmd
}

func newSandboxListCmd(client func() *ForgeClient) *cobra.Command {
	var sessionID string
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sandboxes",
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp struct {
				Sandboxes []*sandbox.Sandbox `json:"sandboxes"`
			}
			path := fmt.Sprintf("/v1/sandboxes?limit=%d&offset=%d", limit, offset)
			if sessionID != "" {
				path += "&session_id=" + sessionID
			}
			if err := client().get(path, &resp); err != nil {
				return err
			}
			if len(resp.Sandboxes) == 0 {
				fmt.Println("No sandboxes found.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSESSION\tDRIVER\tSTATUS\tCREATED")
			for _, sb := range resp.Sandboxes {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					sb.ID,
					sb.Name,
					sb.SessionID,
					sb.IsolationDriver,
					string(sb.Status),
					sb.CreatedAt.Format(time.DateTime),
				)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "Filter by session ID")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of sandboxes to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of sandboxes to skip")
	return cmd
}

func newSandboxCreateCmd(client func() *ForgeClient) *cobra.Command {
	var sessionID, name, driver, workDir string
	var allowPaths []string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new sandbox",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session-id is required")
			}

			var rules []plugins.SandboxPathRule
			for _, p := range allowPaths {
				writable := false
				if strings.HasSuffix(p, ":rw") {
					writable = true
					p = strings.TrimSuffix(p, ":rw")
				} else {
					p = strings.TrimSuffix(p, ":ro")
				}
				rules = append(rules, plugins.SandboxPathRule{Path: p, Writable: writable})
			}

			opts := sandbox.CreateOptions{
				SessionID:       sessionID,
				Name:            name,
				IsolationDriver: driver,
				Spec: plugins.SandboxSpec{
					WorkDir:          workDir,
					AllowedHostPaths: rules,
				},
			}

			var sb sandbox.Sandbox
			if err := client().post("/v1/sandboxes", opts, &sb); err != nil {
				return err
			}

			fmt.Printf("Sandbox created: %s (%s)\n", sb.Name, sb.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "Owning session ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "Human-readable name (auto-generated if empty)")
	cmd.Flags().StringVar(&driver, "driver", "builtin", "Isolation driver name")
	cmd.Flags().StringVar(&workDir, "work-dir", "", "Working directory inside the sandbox")
	cmd.Flags().StringArrayVar(&allowPaths, "allow-path", nil, "Host path to allow (append :rw for write access, e.g. /data:rw)")
	_ = cmd.MarkFlagRequired("session-id")
	return cmd
}

func newSandboxGetCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get sandbox details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var sb sandbox.Sandbox
			if err := client().get("/v1/sandboxes/"+args[0], &sb); err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(sb)
		},
	}
}

func newSandboxDeleteCmd(client func() *ForgeClient) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Destroy a sandbox",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Printf("Destroy sandbox %q? [y/N] ", args[0])
				var answer string
				fmt.Scanln(&answer)
				if strings.ToLower(strings.TrimSpace(answer)) != "y" {
					fmt.Println("Aborted.")
					return nil
				}
			}
			if err := client().delete("/v1/sandboxes/" + args[0]); err != nil {
				return err
			}
			fmt.Printf("Sandbox %s destroyed.\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	return cmd
}

func newSandboxExecCmd(client func() *ForgeClient) *cobra.Command {
	var envVars []string
	var timeout int

	cmd := &cobra.Command{
		Use:   "exec <id> <command> [args...]",
		Short: "Execute a command inside a sandbox (streams output)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			env := make(map[string]string)
			for _, e := range envVars {
				k, v, _ := strings.Cut(e, "=")
				env[k] = v
			}

			req := plugins.SandboxExecRequest{
				Command:        args[1],
				Args:           args[2:],
				Env:            env,
				TimeoutSeconds: timeout,
			}

			resp, err := client().postRaw(fmt.Sprintf("/v1/sandboxes/%s/exec?stream=true", id), req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			return streamSandboxExec(resp)
		},
	}

	cmd.Flags().StringArrayVar(&envVars, "env", nil, "Environment variable (KEY=VALUE, repeatable)")
	cmd.Flags().IntVar(&timeout, "timeout", 30, "Execution timeout in seconds")
	return cmd
}

func newSandboxCopyInCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "copy-in <id> <host-path> <sandbox-path>",
		Short: "Copy a file from the host into a sandbox",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]string{
				"host_src":    args[1],
				"sandbox_dst": args[2],
			}
			var result map[string]any
			if err := client().post("/v1/sandboxes/"+args[0]+"/copy-in", body, &result); err != nil {
				return err
			}
			fmt.Println("Copied.")
			return nil
		},
	}
}

func newSandboxCopyOutCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "copy-out <id> <sandbox-path> <host-path>",
		Short: "Copy a file from a sandbox to the host",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]string{
				"sandbox_src": args[1],
				"host_dst":    args[2],
			}
			var result map[string]any
			if err := client().post("/v1/sandboxes/"+args[0]+"/copy-out", body, &result); err != nil {
				return err
			}
			fmt.Println("Copied.")
			return nil
		},
	}
}

func newSandboxStatCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "stat <id> <path>",
		Short: "Stat a path inside a sandbox",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var result plugins.SandboxStatResult
			path := fmt.Sprintf("/v1/sandboxes/%s/stat?path=%s", args[0], args[1])
			if err := client().get(path, &result); err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
}

func newSandboxReadCmd(client func() *ForgeClient) *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "read <id> <path>",
		Short: "Read a file from inside a sandbox",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var result struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			apiPath := fmt.Sprintf("/v1/sandboxes/%s/read?path=%s", args[0], args[1])
			if err := client().get(apiPath, &result); err != nil {
				return err
			}
			if outputFile != "" {
				return os.WriteFile(outputFile, []byte(result.Content), 0644)
			}
			fmt.Print(result.Content)
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFile, "output", "", "Write content to this local file (default: stdout)")
	return cmd
}

// streamSandboxExec reads SSE chunks from the exec response and prints them.
func streamSandboxExec(resp *http.Response) error {
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
		var chunk plugins.SandboxExecChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.IsError {
			fmt.Fprintf(os.Stderr, "error: %s\n", chunk.Data)
			return fmt.Errorf("execution error")
		}
		switch chunk.Stream {
		case "stderr":
			fmt.Fprint(os.Stderr, chunk.Data)
		default:
			fmt.Print(chunk.Data)
		}
		if chunk.Done {
			if chunk.ExitCode != 0 {
				return fmt.Errorf("command exited with code %d", chunk.ExitCode)
			}
			break
		}
	}
	return scanner.Err()
}
