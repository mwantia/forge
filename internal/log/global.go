package log

import (
	"github.com/hashicorp/go-hclog"
)

var (
	globWriter        *BootstrapWriter
	globInterceptor   hclog.InterceptLogger
	globLevel         hclog.Level
	globDisplayColour bool
)
