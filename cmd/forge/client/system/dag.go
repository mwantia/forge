package system

import (
	"encoding/json"
	"fmt"
	"os"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/refs"
	"github.com/mwantia/forge-sdk/pkg/api/v2/system"
	"github.com/spf13/cobra"
)

func SystemDagCmd(client func() *v2.ForgeApi) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dag",
		Short: "Inspect and debug the DAG object store",
		Long: "Commands for inspecting the content-addressed DAG object store.\n\n" +
			"All session data (messages, prompt contexts, tool catalogs) is stored as immutable\n" +
			"blobs addressed by sha256 of their canonical JSON.\n\n" +
			"Note: subcommands that take <session> accept a session ID only, not a session name.",
	}

	cmd.AddCommand(dagCatCmd(client))
	cmd.AddCommand(dagTypeCmd(client))
	cmd.AddCommand(dagLogCmd(client))
	cmd.AddCommand(dagDiffCmd(client))
	cmd.AddCommand(dagRefsCmd(client))
	cmd.AddCommand(dagVerifyCmd(client))
	cmd.AddCommand(dagObjectsCmd(client))
	cmd.AddCommand(dagGCCmd(client))

	return cmd
}

func dagCatCmd(client func() *v2.ForgeApi) *cobra.Command {
	var pretty, jsonOut bool

	cmd := &cobra.Command{
		Use:   "cat <hash>",
		Short: "Print a DAG object's canonical JSON",
		Long: "Fetch the raw canonical JSON for an object by its full hash or a prefix\n" +
			"(minimum 4 hex characters). Pass --pretty to indent the output.\n" +
			"The X-Forge-Object-Type response header indicates the object kind.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			obj, err := client().System.DagCat(cmd.Context(), system.DagCatRequest{
				Hash:   args[0],
				Pretty: pretty || jsonOut,
			})
			if err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"type": obj.Type,
					"data": obj.Raw,
				})
			}
			fmt.Printf("type: %s\n%s\n", obj.Type, string(obj.Raw))
			return nil
		},
	}
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Indent output")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON envelope {type, data}")
	return cmd
}

func dagTypeCmd(client func() *v2.ForgeApi) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "type <hash>",
		Short: "Print the type of a DAG object",
		Long: "Report the type of a DAG object (message, prompt_context, tool_catalog)\n" +
			"without fetching the full body.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().System.DagType(cmd.Context(), system.DagTypeRequest{Hash: args[0]})
			if err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]string{"type": resp.Type})
			}
			fmt.Println(resp.Type)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	return cmd
}

func dagLogCmd(client func() *v2.ForgeApi) *cobra.Command {
	var ref string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "log <session>",
		Short: "Walk a session's message chain (newest first)",
		Long: "Walk the message chain from the tip of the named ref back to the root,\n" +
			"printing hash, role, and a content preview for each entry.\n\n" +
			"Note: <session> must be a session ID, not a session name.\n" +
			"Use 'forge sessions log' if you want name resolution.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().System.DagLog(cmd.Context(), system.DagLogRequest{
				SessionID: args[0],
				Ref:       ref,
			})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			for entry := range resp.Entries {
				if jsonOut {
					_ = enc.Encode(entry)
					continue
				}
				ts := ""
				if !entry.CreatedAt.IsZero() {
					ts = " " + entry.CreatedAt.Format("2006-01-02 15:04:05")
				}
				fmt.Printf("%s  %-9s %s%s\n", entry.ShortHash, entry.Role, entry.Preview, ts)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "Ref name to walk (default: HEAD)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit NDJSON")
	return cmd
}

func dagDiffCmd(client func() *v2.ForgeApi) *cobra.Command {
	return &cobra.Command{
		Use:   "diff <hash-a> <hash-b>",
		Short: "Unified diff of two DAG objects' canonical JSON",
		Long: "Fetch two objects by hash (or prefix), canonicalize their JSON, and emit\n" +
			"a unified diff to stdout. Useful for comparing versions of a message or\n" +
			"two prompt contexts across model runs.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().System.DagDiff(cmd.Context(), system.DagDiffRequest{
				HashA: args[0],
				HashB: args[1],
			})
			if err != nil {
				return err
			}
			fmt.Print(resp.Diff)
			return nil
		},
	}
}

func dagRefsCmd(client func() *v2.ForgeApi) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "refs <session>",
		Short: "List all refs for a session",
		Long: "List all named refs (HEAD and branches) for the given session and their\n" +
			"current tip hashes.\n\n" +
			"Note: <session> must be a session ID, not a session name.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refsResp, err := client().Refs.List(cmd.Context(), refs.RefsListRequest{SessionID: args[0]})
			if err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(refsResp.Refs)
			}
			for name, hash := range refsResp.Refs {
				fmt.Printf("%-30s %s\n", name, hash)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	return cmd
}

func dagVerifyCmd(client func() *v2.ForgeApi) *cobra.Command {
	var ref string
	var all bool

	cmd := &cobra.Command{
		Use:   "verify <session>",
		Short: "Verify integrity of reachable DAG objects",
		Long: "Walk every reachable object from the named ref, re-hash each blob, and\n" +
			"report any mismatches or missing parents. Exits with status 1 if any errors are found.\n\n" +
			"Note: <session> must be a session ID, not a session name. Use --all to verify every session.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := client().System.DagVerify(cmd.Context(), system.DagVerifyRequest{
				SessionID: args[0],
				Ref:       ref,
				All:       all,
			})
			if err != nil {
				return err
			}
			if result.OK {
				fmt.Println("ok")
				return nil
			}
			for _, e := range result.Errors {
				fmt.Fprintln(os.Stderr, "error:", e)
			}
			os.Exit(1)
			return nil
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "Ref to verify (default: HEAD)")
	cmd.Flags().BoolVar(&all, "all", false, "Verify all sessions")
	return cmd
}

func dagObjectsCmd(client func() *v2.ForgeApi) *cobra.Command {
	var prefix string
	var list, jsonOut bool

	cmd := &cobra.Command{
		Use:   "objects",
		Short: "Count or list objects in the DAG store",
		Long: "Count the total number of objects in the store, optionally filtered by shard prefix\n" +
			"(2 hex chars). Pass --list to stream object hashes instead of just counting.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !list {
				resp, err := client().System.DagObjectsCount(cmd.Context(), system.DagObjectsCountRequest{Prefix: prefix})
				if err != nil {
					return err
				}
				if jsonOut {
					return json.NewEncoder(os.Stdout).Encode(map[string]int{"count": resp.Count})
				}
				fmt.Println(resp.Count)
				return nil
			}
			resp, err := client().System.DagObjectsList(cmd.Context(), system.DagObjectsListRequest{Prefix: prefix})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			for entry := range resp.Entries {
				if jsonOut {
					_ = enc.Encode(entry)
					continue
				}
				fmt.Println(entry.Hash)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "Filter by shard prefix (2 hex chars)")
	cmd.Flags().BoolVar(&list, "list", false, "Stream hashes instead of counting")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON/NDJSON")
	return cmd
}

func dagGCCmd(client func() *v2.ForgeApi) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Garbage-collect unreachable DAG objects",
		Long: "Mark all objects reachable from any session ref, then delete the rest.\n" +
			"Pass --dry-run to report how many objects would be swept without deleting.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := client().System.DagGC(cmd.Context(), system.DagGCRequest{DryRun: dryRun})
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Printf("dry-run  total: %d  would sweep: %d\n", result.Total, result.Swept)
			} else {
				fmt.Printf("total: %d  kept: %d  swept: %d\n", result.Total, result.Kept, result.Swept)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report without deleting")
	return cmd
}
