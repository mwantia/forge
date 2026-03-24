package errors

import (
	"errors"
	"sync"
)

type Errors struct {
	mutex  sync.Mutex
	errors []error
}

func (e *Errors) Add(err error) {
	if err == nil {
		return
	}

	e.mutex.Lock()
	e.errors = append(e.errors, err)
	e.mutex.Unlock()
}

func (e *Errors) Errors() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if len(e.errors) == 0 {
		return nil
	}

	return errors.Join(e.errors...)
}

var (
	ErrPluginNotYetImplemented       = errors.New("plugin feature not yet implemented")
	ErrPluginNotSupported            = errors.New("plugin type not supported by this driver")
	ErrPluginCapabilityNotSupported  = errors.New("plugin capability not supported")
	ErrSkillNotFound                 = errors.New("skill not found")
	ErrInvalidSkillPath              = errors.New("invalid skill path")
)
