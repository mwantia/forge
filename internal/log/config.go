package log

type LogConfig struct {
	Level          string `hcl:"level,optional"`
	File           string `hcl:"file,optional"`
	RotateBytes    int    `hcl:"rotate_bytes,optional"`
	RotateMaxFiles int    `hcl:"rotate_max_files,optional"`
	RotateDuration string `hcl:"rotate_duration,optional"`
}

func NewDefaultConfig() LogConfig {
	return LogConfig{
		Level:          "INFO",
		File:           "",
		RotateBytes:    0,
		RotateMaxFiles: 5,
		RotateDuration: "24h",
	}
}
