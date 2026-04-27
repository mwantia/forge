package template

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/service"
)

// TemplateRenderer is the narrow interface exposed by the template service.
// Callers inject it via fabric:"inject" and either render text directly or
// clone the base template to layer in scope-specific variables (e.g. session
// vars for prompt rendering).
type TemplateRenderer interface {
	// Base returns the shared base template. Callers that need to use it as an
	// hcl.EvalContext (e.g. gohcl.DecodeBody) can call Base().Eval().
	Base() *Template

	// Render evaluates text against the base template.
	Render(text string) (string, error)

	// Clone returns a new template that inherits from the base with additional
	// options layered on top. Use for per-scope renders that need extra vars.
	Clone(opts ...TemplateOption) (*Template, error)
}

type TemplateService struct {
	service.UnimplementedService

	base *Template

	logger hclog.Logger `fabric:"logger:template"`
}

func init() {
	if err := container.Register[*TemplateService](
		container.AsSingleton(),
		container.With[TemplateRenderer](),
	); err != nil {
		panic(err)
	}
}

func (s *TemplateService) Init(ctx context.Context) error {
	base, err := NewTemplate(
		WithRuntime(),
		WithStdlib(),
		WithEnv(),
		WithFilePath(),
		WithGenerate(),
		WithTime(),
		WithBase64(),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize base template: %w", err)
	}

	s.base = base
	return nil
}

func (s *TemplateService) Base() *Template {
	return s.base
}

func (s *TemplateService) Render(text string) (string, error) {
	return s.base.Render(text)
}

func (s *TemplateService) Clone(opts ...TemplateOption) (*Template, error) {
	return s.base.Clone(opts...)
}

var _ service.Service = (*TemplateService)(nil)
var _ TemplateRenderer = (*TemplateService)(nil)
