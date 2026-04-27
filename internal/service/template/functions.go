package template

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

type TemplateOption func(*Template) error

// WithFunction registers a single custom cty function under the given name.
// Use this to extend the template with domain-specific functions not covered
// by the built-in With* options.
//
//	${name(args...)}
func WithFunction(name string, f function.Function) TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		t.funcs[name] = f

		return nil
	}
}

// WithStdlib registers the standard string, JSON, and math functions sourced
// from the cty stdlib. These mirror the built-in functions available in HCL
// (Terraform-style) config files.
//
// String:
//
//	${upper(str)}                     — uppercase
//	${lower(str)}                     — lowercase
//	${trimspace(str)}                 — strip leading/trailing whitespace
//	${trim(str, cutset)}              — strip cutset chars from both ends
//	${trimprefix(str, prefix)}        — strip prefix
//	${trimsuffix(str, suffix)}        — strip suffix
//	${chomp(str)}                     — strip trailing newline
//	${indent(spaces, str)}            — indent all lines after the first
//	${replace(str, old, new)}         — replace all occurrences
//	${split(sep, str)}                — split into list
//	${join(sep, list)}                — join list into string
//	${substr(str, offset, length)}    — substring by byte offset/length
//	${strlen(str)}                    — string length in bytes
//	${format(fmt, args...)}           — sprintf-style formatting
//	${formatlist(fmt, lists...)}      — format applied per-element
//	${contains(list, value)}          — list membership test
//
// JSON:
//
//	${jsonencode(value)}              — encode value to JSON string
//	${jsondecode(str)}                — decode JSON string to value
//
// Math:
//
//	${abs(n)}                         — absolute value
//	${ceil(n)}                        — round up
//	${floor(n)}                       — round down
//	${min(a, b, ...)}                 — minimum
//	${max(a, b, ...)}                 — maximum
func WithStdlib() TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		t.funcs["upper"] = stdlib.UpperFunc
		t.funcs["lower"] = stdlib.LowerFunc
		t.funcs["trimspace"] = stdlib.TrimSpaceFunc
		t.funcs["trim"] = stdlib.TrimFunc
		t.funcs["trimprefix"] = stdlib.TrimPrefixFunc
		t.funcs["trimsuffix"] = stdlib.TrimSuffixFunc
		t.funcs["chomp"] = stdlib.ChompFunc
		t.funcs["indent"] = stdlib.IndentFunc
		t.funcs["replace"] = stdlib.ReplaceFunc
		t.funcs["split"] = stdlib.SplitFunc
		t.funcs["join"] = stdlib.JoinFunc
		t.funcs["substr"] = stdlib.SubstrFunc
		t.funcs["strlen"] = stdlib.StrlenFunc
		t.funcs["format"] = stdlib.FormatFunc
		t.funcs["formatlist"] = stdlib.FormatListFunc
		t.funcs["contains"] = stdlib.ContainsFunc

		t.funcs["jsonencode"] = stdlib.JSONEncodeFunc
		t.funcs["jsondecode"] = stdlib.JSONDecodeFunc

		t.funcs["abs"] = stdlib.AbsoluteFunc
		t.funcs["ceil"] = stdlib.CeilFunc
		t.funcs["floor"] = stdlib.FloorFunc
		t.funcs["min"] = stdlib.MinFunc
		t.funcs["max"] = stdlib.MaxFunc

		return nil
	}
}

// WithGenerate registers ID and name generation functions.
//
//	${uniquename()}   — human-readable unique name (e.g. "bold-ritchie-theta")
//	${uuid()}         — random UUID v4
//	${uuidv6()}       — time-ordered UUID v6
//	${uuidv7()}       — time-ordered UUID v7
func WithGenerate() TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		t.funcs["uniquename"] = function.New(&function.Spec{
			Params: make([]function.Parameter, 0),
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				return cty.StringVal(GenerateUniqueName()), nil
			},
		})
		t.funcs["uuid"] = function.New(&function.Spec{
			Params: make([]function.Parameter, 0),
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				return cty.StringVal(uuid.New().String()), nil
			},
		})
		t.funcs["uuidv6"] = function.New(&function.Spec{
			Params: make([]function.Parameter, 0),
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				v6, err := uuid.NewV6()
				if err != nil {
					return cty.NilVal, fmt.Errorf("failed to generate uuidv6")
				}
				return cty.StringVal(v6.String()), nil
			},
		})
		t.funcs["uuidv7"] = function.New(&function.Spec{
			Params: make([]function.Parameter, 0),
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				v7, err := uuid.NewV7()
				if err != nil {
					return cty.NilVal, fmt.Errorf("failed to generate uuidv7")
				}
				return cty.StringVal(v7.String()), nil
			},
		})

		return nil
	}
}

// WithTime registers time and date functions.
//
//	${now()}                          — current local time as RFC3339 string
//	${utcnow()}                       — current UTC time as RFC3339 string
//	${unixnow()}                      — current Unix timestamp (seconds, number)
//	${date(format, timestamp)}        — format an RFC3339 timestamp using a Go
//	                                    reference-time layout string
//	                                    (e.g. "2006-01-02", "Mon 15:04 MST")
func WithTime() TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		t.funcs["now"] = function.New(&function.Spec{
			Params: make([]function.Parameter, 0),
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				return cty.StringVal(time.Now().Format(time.RFC3339)), nil
			},
		})
		t.funcs["utcnow"] = function.New(&function.Spec{
			Params: make([]function.Parameter, 0),
			Type:   function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				return cty.StringVal(time.Now().UTC().Format(time.RFC3339)), nil
			},
		})
		t.funcs["unixnow"] = function.New(&function.Spec{
			Params: make([]function.Parameter, 0),
			Type:   function.StaticReturnType(cty.Number),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				return cty.NumberIntVal(time.Now().Unix()), nil
			},
		})
		t.funcs["date"] = function.New(&function.Spec{
			Params: []function.Parameter{
				{Name: "format", Type: cty.String},
				{Name: "timestamp", Type: cty.String},
			},
			Type: function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				format := args[0].AsString()
				timestamp := args[1].AsString()

				parsed, err := time.Parse(time.RFC3339, timestamp)
				if err != nil {
					return cty.NilVal, fmt.Errorf("failed to parse timestamp %q as RFC3339: %w", timestamp, err)
				}

				return cty.StringVal(parsed.Format(format)), nil
			},
		})

		return nil
	}
}

// WithFilePath registers file system access functions.
//
//	${file(path)}     — read the entire contents of a file as a string
//	${path(path)}     — resolve a path to its absolute form
func WithFilePath() TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		t.funcs["file"] = function.New(&function.Spec{
			Params: []function.Parameter{
				{Name: "path", Type: cty.String},
			},
			Type: function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				arg := args[0].AsString()
				if arg == "" {
					return cty.NilVal, fmt.Errorf("empty argument for %q received", "path")
				}

				data, err := os.ReadFile(arg)
				if err != nil {
					return cty.NilVal, fmt.Errorf("failed to read file %q: %w", arg, err)
				}

				return cty.StringVal(string(data)), nil
			},
		})
		t.funcs["path"] = function.New(&function.Spec{
			Params: []function.Parameter{
				{Name: "path", Type: cty.String},
			},
			Type: function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				arg := args[0].AsString()
				if arg == "" {
					return cty.Value{}, fmt.Errorf("empty argument for %q received", "path")
				}

				abs, err := filepath.Abs(arg)
				if err != nil {
					return cty.NilVal, fmt.Errorf("failed to get absolute path for %q: %w", arg, err)
				}

				return cty.StringVal(abs), nil
			},
		})

		return nil
	}
}

// WithEnv registers an environment variable lookup function.
// Returns an error if the variable is not set (use ${env.VAR} via WithRuntime
// for a nil-safe accessor instead).
//
//	${env(name)}      — value of the named environment variable
func WithEnv() TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		t.funcs["env"] = function.New(&function.Spec{
			Params: []function.Parameter{
				{Name: "name", Type: cty.String},
			},
			Type: function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				arg := args[0].AsString()
				if arg == "" {
					return cty.NilVal, fmt.Errorf("empty argument for %q received", "name")
				}

				env := os.Getenv(arg)
				if env == "" {
					return cty.NilVal, fmt.Errorf("no environmental variable for %q found", arg)
				}

				return cty.StringVal(env), nil
			},
		})

		return nil
	}
}

// WithBase64 registers Base64 encoding and decoding functions.
//
//	${base64encode(str)}   — encode a string to Base64 (standard encoding)
//	${base64decode(str)}   — decode a Base64 string (standard encoding)
func WithBase64() TemplateOption {
	return func(t *Template) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		t.funcs["base64encode"] = function.New(&function.Spec{
			Params: []function.Parameter{
				{Name: "str", Type: cty.String},
			},
			Type: function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				arg := args[0].AsString()
				if arg == "" {
					return cty.Value{}, fmt.Errorf("empty argument for %q received", "str")
				}

				return cty.StringVal(base64.StdEncoding.EncodeToString([]byte(arg))), nil
			},
		})
		t.funcs["base64decode"] = function.New(&function.Spec{
			Params: []function.Parameter{
				{Name: "str", Type: cty.String},
			},
			Type: function.StaticReturnType(cty.String),
			Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
				arg := args[0].AsString()
				if arg == "" {
					return cty.Value{}, fmt.Errorf("empty argument for %q received", "str")
				}

				decode, err := base64.StdEncoding.DecodeString(arg)
				if err != nil {
					return cty.NilVal, fmt.Errorf("failed to decode string %q: %w", arg, err)
				}

				return cty.StringVal(string(decode)), nil
			},
		})

		return nil
	}
}
