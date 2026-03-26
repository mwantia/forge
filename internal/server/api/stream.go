package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/pkg/plugins"
)

// streamChat streams a ChatStream to the client using Server-Sent Events.
// The caller must not close the stream before calling this; streamChat closes it.
func streamChat(c *gin.Context, stream plugins.ChatStream) {
	defer stream.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		if err != nil {
			data, _ := json.Marshal(errorResponse{Error: errorDetail{Code: "stream_error", Message: err.Error()}})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()
			return
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		if chunk.Done {
			fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
	}
}
