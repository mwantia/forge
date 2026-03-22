package plugins

import "errors"

var (
	ErrPluginNotSupported = errors.New("plugin type not supported by this driver")
	ErrSkillNotFound       = errors.New("skill not found")
	ErrInvalidSkillPath    = errors.New("invalid skill path")
)