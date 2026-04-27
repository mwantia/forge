package config

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service/template"
)

// ConfigTagProcessor handles fabric:"config:<block>" tags.
// Finds the named block(s) in AgentConfig.Remain, injects labels into
// hcl:"...,label" fields, then calls gohcl.DecodeBody for the rest.
// Slice fields collect all matching blocks; scalar fields take the first.
type ConfigTagProcessor struct {
	mu sync.RWMutex

	cfg  *AgentConfig
	tmpl *template.Template
}

// sharedConfigProcessor is registered at init time so that container.Register
// calls that validate fabric tags can find a "config" processor immediately.
// The actual *AgentConfig and template are set later via SetConfig, before any
// Resolve call that needs config tags.
var sharedConfigProcessor = &ConfigTagProcessor{}

func init() {
	container.AddTagProcessor(sharedConfigProcessor)
}

// SetConfig sets the parsed config and shared template on the processor. Must
// be called after config.Parse() and before any container.Resolve that needs
// config tags.
func SetConfig(cfg *AgentConfig, tmpl *template.Template) {
	sharedConfigProcessor.mu.Lock()
	sharedConfigProcessor.cfg = cfg
	sharedConfigProcessor.tmpl = tmpl
	sharedConfigProcessor.mu.Unlock()
}

func (p *ConfigTagProcessor) GetPriority() int { return 50 }

func (p *ConfigTagProcessor) CanProcess(value string) bool {
	lower := strings.ToLower(value)
	return lower == "config" || strings.HasPrefix(lower, "config:")
}

func (p *ConfigTagProcessor) Process(_ context.Context, _ *container.ServiceContainer, field reflect.StructField, value string) (any, error) {
	p.mu.RLock()
	cfg := p.cfg
	tmpl := p.tmpl
	p.mu.RUnlock()
	if cfg == nil {
		return nil, fmt.Errorf("config not yet loaded (SetConfig must be called before Resolve)")
	}
	if tmpl == nil {
		return nil, fmt.Errorf("template not yet loaded (SetConfig must be called with a template before Resolve)")
	}

	_, blockName, _ := strings.Cut(value, ":")
	blockName = strings.ToLower(strings.TrimSpace(blockName))
	if blockName == "" {
		return cfg, nil
	}

	// Slice field → collect all matching blocks.
	if field.Type.Kind() == reflect.Slice {
		return decodeBlockSlice(field.Type, findBlocks(cfg.Remain, blockName), tmpl)
	}

	// Scalar field → first matching block, or zero value if absent.
	targetType := indirectType(field.Type)
	target := reflect.New(targetType).Interface()

	blocks := findBlocks(cfg.Remain, blockName)
	if len(blocks) > 0 {
		injectLabels(reflect.ValueOf(target).Elem(), blocks[0].Labels)
		if diags := gohcl.DecodeBody(blocks[0].Body, tmpl.Eval(), target); diags.HasErrors() {
			return nil, fmt.Errorf("config block %q: %s", blockName, diags.Error())
		}
	}

	if field.Type.Kind() == reflect.Pointer {
		return target, nil
	}
	return reflect.ValueOf(target).Elem().Interface(), nil
}

// decodeBlockSlice decodes each block into an element of sliceType ([]T or []*T).
func decodeBlockSlice(sliceType reflect.Type, blocks []*hclsyntax.Block, tmpl *template.Template) (any, error) {
	elemType := sliceType.Elem()
	isPtr := elemType.Kind() == reflect.Pointer
	baseType := indirectType(elemType)

	result := reflect.MakeSlice(sliceType, 0, len(blocks))
	for _, block := range blocks {
		elem := reflect.New(baseType).Interface()
		injectLabels(reflect.ValueOf(elem).Elem(), block.Labels)
		if diags := gohcl.DecodeBody(block.Body, tmpl.Eval(), elem); diags.HasErrors() {
			return nil, fmt.Errorf("failed to decode block %q: %s", block.Type, diags.Error())
		}
		if isPtr {
			result = reflect.Append(result, reflect.ValueOf(elem))
		} else {
			result = reflect.Append(result, reflect.ValueOf(elem).Elem())
		}
	}
	return result.Interface(), nil
}

// findBlocks returns all blocks matching name in body.
func findBlocks(body hcl.Body, name string) []*hclsyntax.Block {
	syn, ok := body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	var result []*hclsyntax.Block
	for _, block := range syn.Blocks {
		if strings.EqualFold(block.Type, name) {
			result = append(result, block)
		}
	}
	return result
}

// injectLabels sets struct fields tagged hcl:"...,label" from labels in order.
func injectLabels(v reflect.Value, labels []string) {
	t := v.Type()
	idx := 0
	for i := 0; i < t.NumField() && idx < len(labels); i++ {
		_, kind, _ := strings.Cut(t.Field(i).Tag.Get("hcl"), ",")
		if strings.TrimSpace(kind) == "label" {
			v.Field(i).SetString(labels[idx])
			idx++
		}
	}
}

// indirectType unwraps one pointer level: *T → T, T → T.
func indirectType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Pointer {
		return t.Elem()
	}
	return t
}
