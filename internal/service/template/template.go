package template

import (
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type Template struct {
	mu    sync.RWMutex
	vars  map[string]cty.Value
	funcs map[string]function.Function
}

func NewTemplate(opts ...TemplateOption) (*Template, error) {
	template := &Template{
		vars:  make(map[string]cty.Value),
		funcs: make(map[string]function.Function),
	}
	for _, opt := range opts {
		if err := opt(template); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return template, nil
}

func (t *Template) Eval() *hcl.EvalContext {
	return &hcl.EvalContext{
		Variables: buildNestedVariableTree(t.vars),
		Functions: t.funcs,
	}
}

// Clone returns a new Template that inherits all functions and variables from
// the receiver. Additional options are applied on top, making the clone
// suitable for narrower scopes (e.g. per-session values layered onto a shared
// base template). The original is not modified.
func (t *Template) Clone(opts ...TemplateOption) (*Template, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	clone := &Template{
		vars:  make(map[string]cty.Value),
		funcs: make(map[string]function.Function),
	}

	maps.Copy(clone.funcs, t.funcs)
	maps.Copy(clone.vars, t.vars)

	for _, opt := range opts {
		if err := opt(clone); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return clone, nil
}

// Render parses text as an HCL template expression and evaluates it
// against the registered functions and variables.
//
// Interpolations use the ${...} syntax — e.g. ${session.id}, ${upper(env("USER"))}, ${date("2006-01-02", now())}.
// Plain text with no interpolations is returned unchanged.
func (t *Template) Render(text string) (string, error) {
	if text == "" {
		return text, nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	expr, diags := hclsyntax.ParseTemplate([]byte(text), "template", hcl.Pos{
		Line:   1,
		Column: 1,
		Byte:   0,
	})

	if diags.HasErrors() {
		return "", fmt.Errorf("template parse error: %s", diags.Error())
	}

	val, diags := expr.Value(t.Eval())
	if diags.HasErrors() {
		return "", fmt.Errorf("template render error: %s", diags.Error())
	}

	return val.AsString(), nil
}

func (t *Template) Decode(path string, target any) error {
	if path == "" {
		return fmt.Errorf("unable to decode empty filepath")
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if err := hclsimple.DecodeFile(path, t.Eval(), target); err != nil {
		return fmt.Errorf("failed to decode file: %w", err)
	}

	return nil
}

// buildNestedVariableTree converts a flat dot-notated map into the nested
// cty.ObjectVal structure required by hcl.EvalContext. For example:
//
//	{"session.id": cty.StringVal("x"), "session.name": cty.StringVal("y")}
//
// becomes Variables["session"] = cty.ObjectVal({"id": ..., "name": ...}).
func buildNestedVariableTree(flat map[string]cty.Value) map[string]cty.Value {
	grouped := make(map[string]map[string]cty.Value)
	result := make(map[string]cty.Value)

	for k, v := range flat {
		prefix, rest, found := strings.Cut(k, ".")
		if !found {
			result[k] = v
			continue
		}
		if grouped[prefix] == nil {
			grouped[prefix] = make(map[string]cty.Value)
		}
		grouped[prefix][rest] = v
	}

	for prefix, sub := range grouped {
		nested := buildNestedVariableTree(sub)
		if len(nested) > 0 {
			result[prefix] = cty.ObjectVal(nested)
		}
	}

	return result
}
