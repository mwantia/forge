package ui

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type pendingJob struct {
	SessionID string
	Ref       string
	Content   string
	Mode      string
}

var streamJobs sync.Map

func newStreamToken(sessionID, ref, content, mode string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	token := hex.EncodeToString(b)
	streamJobs.Store(token, pendingJob{
		SessionID: sessionID,
		Ref:       ref,
		Content:   content,
		Mode:      mode,
	})

	go func() {
		time.Sleep(2 * time.Minute)
		streamJobs.Delete(token)
	}()

	return token
}

func claimStreamJob(token string) (pendingJob, bool) {
	v, ok := streamJobs.LoadAndDelete(token)

	if !ok {
		return pendingJob{}, false
	}

	return v.(pendingJob), true
}
