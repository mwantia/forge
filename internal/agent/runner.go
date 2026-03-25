package agent

import (
	"context"
	"fmt"
)

type Runner interface {
	Setup() (func() error, error)

	Serve(context.Context) error
}

func (a *Agent) serveRunner(ctx context.Context, runner Runner) error {
	cleanup, err := runner.Setup()
	if err != nil {
		return fmt.Errorf("failed to setup runner: %w", err)
	}

	a.wait.Add(1)

	go func() {
		defer a.wait.Done()

		a.logger.Debug("Executing runner goroutine...")
		if err := runner.Serve(ctx); err != nil {
			a.logger.Error("error serving runner: %w", err)
		}
	}()

	a.cleanups = append(a.cleanups, cleanup)
	return nil
}
