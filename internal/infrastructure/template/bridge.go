package template

import (
	"fmt"
	"math/big"

	"github.com/zclconf/go-cty/cty"
)

// ctyToGo converts a cty.Value to a plain Go value for use as Go template dot
// data or a function return value.
func ctyToGo(v cty.Value) (any, error) {
	if v == cty.NilVal || !v.IsKnown() || v.IsNull() {
		return nil, nil
	}

	ty := v.Type()
	switch {
	case ty == cty.String:
		return v.AsString(), nil

	case ty == cty.Number:
		return smallestNumber(v.AsBigFloat()), nil

	case ty == cty.Bool:
		return v.True(), nil

	case ty.IsObjectType() || ty.IsMapType():
		m := v.AsValueMap()
		result := make(map[string]any, len(m))
		for k, val := range m {
			converted, err := ctyToGo(val)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", k, err)
			}
			result[k] = converted
		}
		return result, nil

	case ty.IsListType() || ty.IsSetType() || ty.IsTupleType():
		elems := v.AsValueSlice()
		result := make([]any, len(elems))
		for i, elem := range elems {
			converted, err := ctyToGo(elem)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			result[i] = converted
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unsupported cty type: %s", ty.FriendlyName())
	}
}

func smallestNumber(b *big.Float) any {
	if v, acc := b.Int64(); acc == big.Exact {
		if int64(int(v)) == v {
			return int(v)
		}
		return v
	}
	f, _ := b.Float64()
	return f
}

// goToCtyByType converts a plain Go value to a cty.Value guided by the
// expected cty.Type from a function.Parameter. Used to bridge Go template call
// arguments into function.Function.Call().
func goToCtyByType(v any, ty cty.Type) (cty.Value, error) {
	if v == nil {
		return cty.NullVal(ty), nil
	}
	switch ty {
	case cty.String:
		s, ok := v.(string)
		if !ok {
			return cty.NilVal, fmt.Errorf("expected string, got %T", v)
		}
		return cty.StringVal(s), nil

	case cty.Number:
		switch n := v.(type) {
		case int:
			return cty.NumberIntVal(int64(n)), nil
		case int64:
			return cty.NumberIntVal(n), nil
		case float64:
			return cty.NumberFloatVal(n), nil
		default:
			return cty.NilVal, fmt.Errorf("expected number, got %T", v)
		}

	case cty.Bool:
		b, ok := v.(bool)
		if !ok {
			return cty.NilVal, fmt.Errorf("expected bool, got %T", v)
		}
		return cty.BoolVal(b), nil

	default:
		// DynamicPseudoType and other types: best-effort by Go type.
		switch val := v.(type) {
		case string:
			return cty.StringVal(val), nil
		case bool:
			return cty.BoolVal(val), nil
		case int:
			return cty.NumberIntVal(int64(val)), nil
		case int64:
			return cty.NumberIntVal(val), nil
		case float64:
			return cty.NumberFloatVal(val), nil
		default:
			return cty.NilVal, fmt.Errorf("cannot convert %T to cty type %s", v, ty.FriendlyName())
		}
	}
}
