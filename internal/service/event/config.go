package event

type EventConfig struct {
	ID          string        `hcl:"id,label"`
	Description string        `hcl:"description,optional"`
	Session     string        `hcl:"session"`
	Branch      string        `hcl:"branch,optional"`
	Model       string        `hcl:"model,optional"`
	Prompt      string        `hcl:"prompt,optional"`
	Options     *EventOptions `hcl:"options,block"`
}

type EventOptions struct {
	Timespan string `hcl:"timespan,optional"`
	MaxQueue int    `hcl:"max_queue,optional"`
}
