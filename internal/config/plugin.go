package config

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

type PluginConfig struct {
	Name   string            `hcl:"name,label"`
	Type   string            `hcl:"type,label"`
	Config *PluginConfigBody `hcl:"config,block"`
}

type PluginConfigBody struct {
	Body hcl.Body `hcl:",remain"`
}

func (b *PluginConfigBody) DecodeBody(ctx *hcl.EvalContext) (map[string]any, error) {
	// Return empty map if undefined config
	if b.Body == nil {
		return make(map[string]any), nil
	}
	// Native HCL syntax files expose the full AST via hclsyntax.Body.
	if synBody, ok := b.Body.(*hclsyntax.Body); ok {
		return decodeSyntaxBody(ctx, synBody)
	}
	// Fallback for non-native bodies (e.g. JSON-based HCL): attributes only.
	attrs, diags := b.Body.JustAttributes()
	if diags.HasErrors() {
		return nil, fmt.Errorf("%s", diags.Error())
	}
	result := make(map[string]any)
	for name, attr := range attrs {
		val, diags := attr.Expr.Value(ctx)
		if diags.HasErrors() {
			return nil, fmt.Errorf("attribute %q: %s", name, diags.Error())
		}
		result[name] = ctyValueToGo(val)
	}
	return result, nil
}

func decodeSyntaxBody(ctx *hcl.EvalContext, body *hclsyntax.Body) (map[string]any, error) {
	result := make(map[string]any)

	for name, attr := range body.Attributes {
		val, diags := attr.Expr.Value(ctx)
		if diags.HasErrors() {
			return nil, fmt.Errorf("attribute %q: %s", name, diags.Error())
		}
		result[name] = ctyValueToGo(val)
	}

	for _, block := range body.Blocks {
		blockMap, err := decodeSyntaxBody(ctx, block.Body)
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
