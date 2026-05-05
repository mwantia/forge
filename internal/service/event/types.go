package event

import "time"

// FireResponse is the JSON body returned by GET/POST /v1/events/:id/fire.
type FireResponse struct {
	EventID         string     `json:"event_id"`
	Status          string     `json:"status"`
	FiredAt         time.Time  `json:"fired_at"`
	Branch          string     `json:"branch,omitempty"`
	QueueSize       int        `json:"queue_size,omitempty"`
	QueueCapacity   int        `json:"queue_capacity,omitempty"`
	Evicted         bool       `json:"evicted,omitempty"`
	WindowExpiresAt *time.Time `json:"window_expires_at,omitempty"`
}

// EventStatus is the JSON body returned by GET /v1/events and GET /v1/events/:id.
type EventStatus struct {
	ID          string      `json:"id"`
	Description string      `json:"description,omitempty"`
	Session     string      `json:"session"`
	Options     *queueOpts  `json:"options,omitempty"`
	Queue       *queueState `json:"queue,omitempty"`
	LastBranch  string      `json:"last_branch,omitempty"`
}

type queueOpts struct {
	Timespan string `json:"timespan,omitempty"`
	MaxQueue int    `json:"max_queue,omitempty"`
}

type queueState struct {
	Size            int        `json:"size"`
	WindowExpiresAt *time.Time `json:"window_expires_at,omitempty"`
}
