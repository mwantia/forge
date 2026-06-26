package pipeline

import (
	"fmt"
	"time"
)

const (
	DefaultPipelineRecallBudget           = 3
	DefaultPipelineRecallThreshold        = 0.35
	DefaultPipelineRecallMinLength        = 15
	DefaultPipelineRetryAttempts          = 3
	DefaultPipelineBackoffBase            = 250
	DefaultPipelineBackoffMax             = 5000
	DefaultPipelineOutputMode             = "block"
	DefaultPipelineOutputCodeFence        = "atomic"
	DefaultPipelineOutputMaxChunkBytes    = 0
	DefaultPipelinePacingEnabled          = false
	DefaultPipelinePacingCPS              = 60
	DefaultPipelinePacingJitter           = 1
	DefaultPipelinePacingPunctuationPause = 0
)

type PipelineConfig struct {
	MaxToolIterations int                   `hcl:"max_tool_iterations,optional"`
	System            string                `hcl:"system,optional"`
	Output            *PipelineOutputConfig `hcl:"output,block"`
	Retry             *PipelineRetryConfig  `hcl:"retry,block"`
	Recall            *PipelineRecallConfig `hcl:"recall,block"`
}

type PipelineRecallConfig struct {
	Budget    *int     `hcl:"budget,optional"`     // max messages that consume budget (default 3, -1 = disabled)
	Threshold *float64 `hcl:"threshold,optional"`  // minimum score gate (default 0.1; 0 = no gate, agent decides)
	MinLength *int     `hcl:"min_length,optional"` // minimum rune count for pre-filter (default 15)
}

func (c *PipelineRecallConfig) GetBudget() int {
	if c == nil || c.Budget == nil {
		return DefaultPipelineRecallBudget
	}

	budget := *c.Budget
	if budget < 0 {
		return 0
	}

	return budget
}

func (c *PipelineRecallConfig) GetThreshold() float64 {
	if c == nil || c.Threshold == nil {
		return DefaultPipelineRecallThreshold
	}

	threshold := *c.Threshold
	if threshold < 0 {
		return 0
	}

	return threshold
}

func (c *PipelineRecallConfig) GetMinLength() int {
	if c == nil || c.MinLength == nil {
		return DefaultPipelineRecallMinLength
	}

	minLength := *c.MinLength
	if minLength < 0 {
		return 0
	}

	return minLength
}

type PipelineRetryConfig struct {
	Attempts *int                   `hcl:"attempts,optional"` // total attempts incl. first try (default 3)
	Backoff  *PipelineBackoffConfig `hcl:"backoff,block"`
}

func (c *PipelineRetryConfig) GetAttempts() int {
	if c == nil || c.Attempts == nil {
		return DefaultPipelineRetryAttempts
	}

	attempts := *c.Attempts
	if attempts < 0 {
		return 0
	}

	return attempts
}

type PipelineBackoffConfig struct {
	Base string `hcl:"base,optional"` // duration string, doubled per attempt (default "250ms")
	Max  string `hcl:"max,optional"`  // duration string; cap on per-attempt wait (default "5s")
}

func (c *PipelineBackoffConfig) GetBase() (time.Duration, error) {
	if c == nil || c.Base == "" {
		return DefaultPipelineBackoffBase * time.Millisecond, nil
	}

	duration, err := time.ParseDuration(c.Base)
	if err != nil {
		return duration, fmt.Errorf("failed to parse 'base' duration: %w", err)
	}

	if duration < 0 {
		return 0, nil
	}

	return duration, err
}

func (c *PipelineBackoffConfig) GetMax() (time.Duration, error) {
	if c == nil || c.Max == "" {
		return DefaultPipelineBackoffMax * time.Millisecond, nil
	}

	duration, err := time.ParseDuration(c.Max)
	if err != nil {
		return duration, fmt.Errorf("failed to parse 'base' duration: %w", err)
	}

	if duration < 0 {
		return 0, nil
	}

	return duration, err
}

type PipelineOutputConfig struct {
	Mode          string                `hcl:"mode,optional"`            // token | sentence | block | final
	CodeFence     string                `hcl:"code_fence,optional"`      // atomic | split
	MaxChunkBytes *int                  `hcl:"max_chunk_bytes,optional"` // 0 = no cap
	Pacing        *PipelinePacingConfig `hcl:"pacing,block"`
}

func (c *PipelineOutputConfig) GetMode() string {
	if c == nil || c.Mode == "" {
		return DefaultPipelineOutputMode
	}

	return c.Mode
}

func (c *PipelineOutputConfig) GetCodeFence() string {
	if c == nil || c.CodeFence == "" {
		return DefaultPipelineOutputCodeFence
	}

	return c.CodeFence
}

func (c *PipelineOutputConfig) GetMaxChunkBytes() int {
	if c == nil || c.MaxChunkBytes == nil {
		return DefaultPipelineOutputMaxChunkBytes
	}

	maxChunkBytes := *c.MaxChunkBytes
	if maxChunkBytes < 0 {
		return 0
	}

	return maxChunkBytes
}

type PipelinePacingConfig struct {
	Enabled          *bool    `hcl:"enabled,optional"`
	CPS              *int     `hcl:"cps,optional"`
	Jitter           *float64 `hcl:"jitter,optional"`
	PunctuationPause *int     `hcl:"punctuation_pause,optional"`
}

func (c *PipelinePacingConfig) GetEnabled() bool {
	if c == nil || c.Enabled == nil {
		return DefaultPipelinePacingEnabled
	}

	return *c.Enabled
}

func (c *PipelinePacingConfig) GetCPS() int {
	if c == nil || c.CPS == nil {
		return DefaultPipelinePacingCPS
	}

	cps := *c.CPS
	if cps < 0 {
		return 0
	}

	return cps
}

func (c *PipelinePacingConfig) GetJitter() float64 {
	if c == nil || c.Jitter == nil {
		return DefaultPipelinePacingJitter
	}

	jitter := *c.Jitter
	if jitter < 0 {
		return 0
	}

	return jitter
}

func (c *PipelinePacingConfig) GetPunctuationPause() int {
	if c == nil || c.PunctuationPause == nil {
		return DefaultPipelinePacingPunctuationPause
	}

	punctuationPause := *c.PunctuationPause
	if punctuationPause < 0 {
		return 0
	}

	return punctuationPause
}

// Output modes.
const (
	OutputModeToken    = "token"
	OutputModeSentence = "sentence"
	OutputModeBlock    = "block"
	OutputModeFinal    = "final"
)

// Code fence policies.
const (
	CodeFenceAtomic = "atomic"
	CodeFenceSplit  = "split"
)

// ResolveOutputPolicy is the per-request effective output policy.
// Derived from OutputConfig with defaults and any request-level overrides.
type ResolveOutputPolicy struct {
	Mode          string
	CodeFence     string
	MaxChunkBytes int
	Pacing        ResolvePolicyPacing
}

type ResolvePolicyPacing struct {
	Enabled          bool
	CPS              int
	Jitter           float64
	PunctuationPause int
}

const (
	DefaultResolveOutputPolicyMode      = "block"
	DefaultResolveOutputPolicyCodeFence = "atomic"
)

func (c *PipelineOutputConfig) ResolveOutputPolicy() ResolveOutputPolicy {
	if c == nil {
		return RawOverwriteOutputPolicy()
	}

	policy := ResolveOutputPolicy{
		Mode:          c.GetMode(),
		CodeFence:     c.GetCodeFence(),
		MaxChunkBytes: c.GetMaxChunkBytes(),
	}
	if c.Pacing != nil && c.Pacing.GetEnabled() {
		policy.Pacing = ResolvePolicyPacing{
			CPS:              c.Pacing.GetCPS(),
			Jitter:           c.Pacing.GetJitter(),
			PunctuationPause: c.Pacing.GetPunctuationPause(),
		}
	}

	return policy
}

func RawOverwriteOutputPolicy() ResolveOutputPolicy {
	return ResolveOutputPolicy{
		Mode:      DefaultResolveOutputPolicyMode,
		CodeFence: DefaultResolveOutputPolicyCodeFence,
	}
}
