package config

import (
	"github.com/hashicorp/hcl/v2"
)

type MetaConfig struct {
	Body hcl.Body `hcl:",remain"`
}
