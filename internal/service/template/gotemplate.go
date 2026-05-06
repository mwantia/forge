package template

import (
	"bytes"
	"fmt"
	gotemplate "text/template"

	"github.com/zclconf/go-cty/cty"
)

// GoFuncMap returns a text/template FuncMap that wraps every registered
// function.Function. Each wrapper converts Go args → cty via goToCtyByType,
// calls fn.Call(), and converts the result → Go via ctyToGo.
func (t *Template) GoFuncMap() gotemplate.FuncMap {
	t.mu.RLock()
	defer t.mu.RUnlock()

	fm := make(gotemplate.FuncMap, len(t.funcs))
	for name, fn := range t.funcs {
		params := fn.Params()
		varParam := fn.VarParam()

		fm[name] = func(args ...any) (any, error) {
			minArgs := len(params)
			if varParam == nil && len(args) != minArgs {
				return nil, fmt.Errorf("function %q expects %d args, got %d", name, minArgs, len(args))
			}
			if varParam != nil && len(args) < minArgs {
				return nil, fmt.Errorf("function %q expects at least %d args, got %d", name, minArgs, len(args))
			}

			ctyArgs := make([]cty.Value, len(args))
			for i, arg := range args {
				var paramType cty.Type
				if i < len(params) {
					paramType = params[i].Type
				} else {
					paramType = varParam.Type
				}
				v, err := goToCtyByType(arg, paramType)
				if err != nil {
					return nil, fmt.Errorf("function %q arg %d: %w", name, i, err)
				}
				ctyArgs[i] = v
			}

			result, err := fn.Call(ctyArgs)
			if err != nil {
				return nil, err
			}
			return ctyToGo(result)
		}
	}
	return fm
}

// GoData builds the nested variable tree and converts every cty.Value leaf to
// a plain Go value via ctyToGo. The result is a map[string]any usable as Go
// template dot data — e.g. {{ .session.id }}, {{ .runtime.version }}.
func (t *Template) GoData() (map[string]any, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	nested := buildNestedVariableTree(t.vars)
	result := make(map[string]any, len(nested))
	for k, v := range nested {
		converted, err := ctyToGo(v)
		if err != nil {
			return nil, fmt.Errorf("variable %q: %w", k, err)
		}
		result[k] = converted
	}
	return result, nil
}

// RenderBody parses text as a Go text/template, registers GoFuncMap(), and
// executes with GoData() as dot.
//
// Variable access:  {{ .session.id }},  {{ .runtime.version }}
// Function calls:   {{ upper "foo" }},  {{ uuid }}
//
// Unlike RenderConfig, this syntax never conflicts with HCL ${...} escaping
// rules, making it suitable for human-authored body text and system prompts.
func (t *Template) RenderBody(text string) (string, error) {
	if text == "" {
		return text, nil
	}

	data, err := t.GoData()
	if err != nil {
		return "", fmt.Errorf("template data: %w", err)
	}

	tmpl, err := gotemplate.New("").Funcs(t.GoFuncMap()).Parse(text)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template render error: %w", err)
	}
	return buf.String(), nil
}
