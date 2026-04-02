package eval

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// NewEvalContext builds an HCL evaluation context.
//
// vars are additional key=value pairs exposed under the "var" namespace,
// e.g. var.my_key. Pass nil when no extra variables are needed.
//
// Relative paths in file() and path() are resolved against the working
// directory of the forge process (i.e. where it was launched from).
func NewEvalContext(vars map[string]string) *hcl.EvalContext {
	return &hcl.EvalContext{
		Variables: buildVariables(vars),
		Functions: buildFunctions(),
	}
}

func buildVariables(vars map[string]string) map[string]cty.Value {
	// env.VAR_NAME — mirrors Nomad's env accessor
	envMap := map[string]cty.Value{}
	for _, e := range os.Environ() {
		k, v, found := strings.Cut(e, "=")
		if found {
			envMap[k] = cty.StringVal(v)
		}
	}
	if len(envMap) == 0 {
		envMap["_"] = cty.StringVal("") // go-cty panics on empty object literal
	}

	// var.KEY — caller-supplied extra variables
	varMap := map[string]cty.Value{}
	for k, v := range vars {
		varMap[k] = cty.StringVal(v)
	}
	if len(varMap) == 0 {
		varMap["_"] = cty.StringVal("")
	}

	return map[string]cty.Value{
		"env": cty.ObjectVal(envMap),
		"var": cty.ObjectVal(varMap),
	}
}

func buildFunctions() map[string]function.Function {
	return map[string]function.Function{
		// ── Environment ─────────────────────────────────────────────────────────
		// env("VAR") — returns "" if unset, never errors
		"env": function.New(&function.Spec{
			Params: []function.Parameter{{Name: "name", Type: cty.String}},
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				return cty.StringVal(os.Getenv(args[0].AsString())), nil
			},
		}),

		// ── File system ─────────────────────────────────────────────────────────
		// file(path) — reads file contents relative to the working directory
		"file": function.New(&function.Spec{
			Params: []function.Parameter{{Name: "path", Type: cty.String}},
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				data, err := os.ReadFile(args[0].AsString())
				if err != nil {
					return cty.NilVal, fmt.Errorf("file: %w", err)
				}
				return cty.StringVal(string(data)), nil
			},
		}),

		// path(p) — resolves p to an absolute path (relative to working directory)
		"path": function.New(&function.Spec{
			Params: []function.Parameter{{Name: "path", Type: cty.String}},
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				abs, err := filepath.Abs(args[0].AsString())
				if err != nil {
					return cty.NilVal, fmt.Errorf("path: %w", err)
				}
				return cty.StringVal(abs), nil
			},
		}),

		// ── String ──────────────────────────────────────────────────────────────
		"upper":      stdlib.UpperFunc,
		"lower":      stdlib.LowerFunc,
		"trimspace":  stdlib.TrimSpaceFunc,
		"trim":       stdlib.TrimFunc,
		"trimprefix": stdlib.TrimPrefixFunc,
		"trimsuffix": stdlib.TrimSuffixFunc,
		"chomp":      stdlib.ChompFunc,
		"indent":     stdlib.IndentFunc,
		"replace":    stdlib.ReplaceFunc,
		"split":      stdlib.SplitFunc,
		"join":       stdlib.JoinFunc,
		"substr":     stdlib.SubstrFunc,
		"strlen":     stdlib.StrlenFunc,
		"format":     stdlib.FormatFunc,
		"formatlist": stdlib.FormatListFunc,
		"contains":   stdlib.ContainsFunc,

		// ── Encoding ────────────────────────────────────────────────────────────
		"base64encode": function.New(&function.Spec{
			Params: []function.Parameter{{Name: "str", Type: cty.String}},
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				return cty.StringVal(base64.StdEncoding.EncodeToString([]byte(args[0].AsString()))), nil
			},
		}),
		"base64decode": function.New(&function.Spec{
			Params: []function.Parameter{{Name: "str", Type: cty.String}},
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				dec, err := base64.StdEncoding.DecodeString(args[0].AsString())
				if err != nil {
					return cty.NilVal, fmt.Errorf("base64decode: %w", err)
				}
				return cty.StringVal(string(dec)), nil
			},
		}),
		"jsonencode": stdlib.JSONEncodeFunc,
		"jsondecode": stdlib.JSONDecodeFunc,

		// ── Math ────────────────────────────────────────────────────────────────
		"abs":   stdlib.AbsoluteFunc,
		"ceil":  stdlib.CeilFunc,
		"floor": stdlib.FloorFunc,
		"min":   stdlib.MinFunc,
		"max":   stdlib.MaxFunc,

		// ── Type conversion ─────────────────────────────────────────────────────
		"tostring": stdlib.MakeToFunc(cty.String),
		"tonumber": stdlib.MakeToFunc(cty.Number),
		"tobool":   stdlib.MakeToFunc(cty.Bool),
	}
}
