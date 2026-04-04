package config

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// StorageConfig holds the raw HCL for a storage block:
//
//	storage "file" {
//	  path = "./data"
//	}
type StorageConfig struct {
	Type string   `hcl:"type,label"`
	Body hcl.Body `hcl:",remain"`
}

// DecodeBody evaluates all attributes in the storage block and returns them as
// a plain map. The same eval context used for plugin configs is accepted so
// env() and other functions work here too.
func (s *StorageConfig) DecodeBody(ctx *hcl.EvalContext) (map[string]any, error) {
	if s.Body == nil {
		return make(map[string]any), nil
	}
	if synBody, ok := s.Body.(*hclsyntax.Body); ok {
		return decodeStorageSyntaxBody(ctx, synBody)
	}
	// Fallback for JSON-based HCL: attributes only.
	attrs, diags := s.Body.JustAttributes()
	if diags.HasErrors() {
		return nil, fmt.Errorf("%s", diags.Error())
	}
	result := make(map[string]any)
	for name, attr := range attrs {
		val, diags := attr.Expr.Value(ctx)
		if diags.HasErrors() {
			return nil, fmt.Errorf("attribute %q: %s", name, diags.Error())
		}
		result[name] = storageCtyValueToGo(val)
	}
	return result, nil
}

func decodeStorageSyntaxBody(ctx *hcl.EvalContext, body *hclsyntax.Body) (map[string]any, error) {
	result := make(map[string]any)
	for name, attr := range body.Attributes {
		val, diags := attr.Expr.Value(ctx)
		if diags.HasErrors() {
			return nil, fmt.Errorf("attribute %q: %s", name, diags.Error())
		}
		result[name] = storageCtyValueToGo(val)
	}
	return result, nil
}

func storageCtyValueToGo(value cty.Value) any {
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
			result = append(result, storageCtyValueToGo(v))
		}
		return result
	case ty.IsObjectType() || ty.IsMapType():
		result := make(map[string]any)
		for it := value.ElementIterator(); it.Next(); {
			k, v := it.Element()
			result[k.AsString()] = storageCtyValueToGo(v)
		}
		return result
	default:
		return value.GoString()
	}
}
