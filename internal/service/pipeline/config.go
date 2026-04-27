package pipeline

type PipelineConfig struct {
	MaxToolIterations int           `hcl:"max_tool_iterations,optional"`
	System            string        `hcl:"system,optional"`
	Output            *OutputConfig `hcl:"output,block"`
}

// OutputConfig controls how the server chunks and paces pipeline output.
// Rendering is always a per-channel concern; this block only dictates
// what boundaries are emitted and how fast.
type OutputConfig struct {
	Mode          string        `hcl:"mode,optional"`            // token | sentence | block | final
	CodeFence     string        `hcl:"code_fence,optional"`      // atomic | split
	MaxChunkBytes int           `hcl:"max_chunk_bytes,optional"` // 0 = no cap
	Pacing        *PacingConfig `hcl:"pacing,block"`
}

type PacingConfig struct {
	Enabled            bool    `hcl:"enabled,optional"`
	CPS                int     `hcl:"cps,optional"`
	Jitter             float64 `hcl:"jitter,optional"`
	PunctuationPauseMs int     `hcl:"punctuation_pause_ms,optional"`
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

// resolvedOutput is the per-request effective output policy.
// Derived from OutputConfig with defaults and any request-level overrides.
type resolvedOutput struct {
	Mode          string
	CodeFence     string
	MaxChunkBytes int
	Pacing        resolvedPacing
}

type resolvedPacing struct {
	Enabled            bool
	CPS                int
	Jitter             float64
	PunctuationPauseMs int
}

// resolve applies defaults to an OutputConfig and returns the effective policy.
// A nil receiver yields the default policy.
func (o *OutputConfig) resolve() resolvedOutput {
	out := resolvedOutput{
		Mode:      OutputModeBlock,
		CodeFence: CodeFenceAtomic,
	}
	if o == nil {
		return out
	}
	if o.Mode != "" {
		out.Mode = o.Mode
	}
	if o.CodeFence != "" {
		out.CodeFence = o.CodeFence
	}
	if o.MaxChunkBytes > 0 {
		out.MaxChunkBytes = o.MaxChunkBytes
	}
	if o.Pacing != nil && o.Pacing.Enabled {
		out.Pacing = resolvedPacing{
			Enabled:            true,
			CPS:                o.Pacing.CPS,
			Jitter:             o.Pacing.Jitter,
			PunctuationPauseMs: o.Pacing.PunctuationPauseMs,
		}
		if out.Pacing.CPS <= 0 {
			out.Pacing.CPS = 60
		}
		if out.Pacing.Jitter < 0 {
			out.Pacing.Jitter = 0
		}
		if out.Pacing.Jitter > 1 {
			out.Pacing.Jitter = 1
		}
	}
	return out
}

// rawOverride returns the policy used when a request sets ?raw=true:
// token-mode, no pacing, no code-fence protection.
func rawOverride() resolvedOutput {
	return resolvedOutput{
		Mode:      OutputModeToken,
		CodeFence: CodeFenceSplit,
	}
}
