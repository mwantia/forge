package config

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mwantia/forge/internal/service/template"
	"github.com/zclconf/go-cty/cty"
)

// Decode decodes an hcl.Body into T using gohcl. Returns zero value if body is nil.
func Decode[T any](body hcl.Body, tmpl *template.Template) (T, error) {
	var target T
	if body == nil {
		return target, nil
	}

	if diags := gohcl.DecodeBody(body, tmpl.Eval(), &target); diags.HasErrors() {
		return target, fmt.Errorf("%s", diags.Error())
	}
	return target, nil
}

func DecodeBody(body hcl.Body, tmpl *template.Template) (map[string]any, error) {
	if body == nil {
		return make(map[string]any), nil
	}

	if synBody, ok := body.(*hclsyntax.Body); ok {
		return decodeSyntaxBody(synBody, tmpl)
	}

	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return nil, fmt.Errorf("%s", diags.Error())
	}
	result := make(map[string]any)
	for name, attr := range attrs {
		val, diags := attr.Expr.Value(tmpl.Eval())
		if diags.HasErrors() {
			return nil, fmt.Errorf("attribute %q: %s", name, diags.Error())
		}
		result[name] = ctyValueToGo(val)
	}
	return result, nil
}

func decodeSyntaxBody(body *hclsyntax.Body, tmpl *template.Template) (map[string]any, error) {
	result := make(map[string]any)

	for name, attr := range body.Attributes {
		val, diags := attr.Expr.Value(tmpl.Eval())
		if diags.HasErrors() {
			return nil, fmt.Errorf("attribute %q: %s", name, diags.Error())
		}
		result[name] = ctyValueToGo(val)
	}

	for _, block := range body.Blocks {
		blockMap, err := decodeSyntaxBody(block.Body, tmpl)
		if err != nil {
			return nil, fmt.Errorf("block %q: %w", block.Type, err)
		}

		if len(block.Labels) == 0 {
			result[block.Type] = blockMap
		} else {
			existing, ok := result[block.Type]
			if !ok {
				existing = make(map[string]any)
				result[block.Type] = existing
			}
			if m, ok := existing.(map[string]any); ok {
				m[block.Labels[0]] = blockMap
			}
		}
	}

	return result, nil
}

func ctyValueToGo(value cty.Value) any {
	ty := value.Type()

	switch {
	case ty == cty.String:
		return value.AsString()
	case ty == cty.Number:
		f, _ := value.AsBigFloat().Float64()
		return f
	case ty == cty.Bool:
		return value.True()
	case ty.IsListType() || ty.IsTupleType() || ty.IsSetType():
		var result []any
		for it := value.ElementIterator(); it.Next(); {
			_, v := it.Element()
			result = append(result, ctyValueToGo(v))
		}
		return result
	case ty.IsObjectType() || ty.IsMapType():
		result := make(map[string]any)
		for it := value.ElementIterator(); it.Next(); {
			k, v := it.Element()
			result[k.AsString()] = ctyValueToGo(v)
		}
		return result
	default:
		return value.GoString()
	}
}
