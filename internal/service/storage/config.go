package storage

import "github.com/hashicorp/hcl/v2"

type StorageConfig struct {
	Type string   `hcl:"type,label"`
	Body hcl.Body `hcl:",remain"`
}

type FileConfig struct {
	Path string `hcl:"path,optional"`
}
