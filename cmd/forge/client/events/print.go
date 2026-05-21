package events

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/mwantia/forge-sdk/pkg/api/v2/events"
)

func printEventStatus(ev events.EventStatus) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)

	kv(w, "ID", ev.ID)
	kv(w, "Session", ev.Session)
	if ev.Description != "" {
		kv(w, "Description", ev.Description)
	}
	kv(w, "State", string(ev.State))
	if ev.LastBranch != "" {
		kv(w, "Last Branch", ev.LastBranch)
	}

	if ev.Options != nil {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Options")
		if ev.Options.Timespan != "" {
			kv(w, "  Timespan", ev.Options.Timespan)
		}
		if ev.Options.MaxQueue > 0 {
			kv(w, "  Max Queue", fmt.Sprintf("%d", ev.Options.MaxQueue))
		}
	}

	if ev.Queue != nil {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Queue")
		kv(w, "  Size", fmt.Sprintf("%d", ev.Queue.Size))
		if ev.Queue.WindowExpiresAt != nil {
			kv(w, "  Window Expires", formatTime(*ev.Queue.WindowExpiresAt))
		}
	}

	return w.Flush()
}

func printFireResponse(r events.FireResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)

	kv(w, "Event ID", r.EventID)
	kv(w, "Status", r.Status)
	kv(w, "Fired At", r.FiredAt.Local().Format(time.RFC3339))
	if r.Branch != "" {
		kv(w, "Branch", r.Branch)
	}
	if r.QueueCapacity > 0 {
		kv(w, "Queue", fmt.Sprintf("%d / %d", r.QueueSize, r.QueueCapacity))
	}
	if r.Evicted {
		kv(w, "Evicted", "true")
	}
	if r.WindowExpiresAt != nil {
		kv(w, "Window Expires", formatTime(*r.WindowExpiresAt))
	}

	return w.Flush()
}

func kv(w *tabwriter.Writer, key, value string) {
	fmt.Fprintf(w, "%s\t= %s\n", key, value)
}

func formatTime(t time.Time) string {
	d := time.Until(t)
	if d < 0 {
		return t.Local().Format(time.RFC3339) + " (expired)"
	}
	return fmt.Sprintf("%s (%s remaining)", t.Local().Format(time.RFC3339), d.Round(time.Second))
}
