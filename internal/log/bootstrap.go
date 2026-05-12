package log

import (
	"io"
	"os"
	"sync"

	"github.com/hashicorp/go-hclog"
)

type BootstrapWriter struct {
	mu sync.RWMutex

	writer  io.Writer
	buffers [][]byte
}

func Bootstrap(level hclog.Level, displayColour bool) error {
	if displayColour {
		out := NewColouredWriter(os.Stdout)
		globWriter = newBootstrapWriter(out)
	} else {
		globWriter = newBootstrapWriter(os.Stdout)
	}

	globLevel = level
	globDisplayColour = displayColour

	globInterceptor = hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:        "forge",
		Level:       level,
		DisableTime: true,
		Output:      globWriter,
	})
	hclog.SetDefault(globInterceptor)

	return nil
}

func newBootstrapWriter(writer io.Writer) *BootstrapWriter {
	return &BootstrapWriter{
		writer:  writer,
		buffers: make([][]byte, 0),
	}
}

func (w *BootstrapWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	buf := make([]byte, len(p))
	copy(buf, p)
	w.buffers = append(w.buffers, buf)

	return len(buf), nil
}

func (w *BootstrapWriter) Flush() error {
	if len(w.buffers) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffers = make([][]byte, 0)

	return nil
}
