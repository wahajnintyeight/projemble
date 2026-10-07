package tui

import (
	"context"
	"time"

	"projemble/internal/agent"
)

func launchAgentTurn(session *agent.Agent, workspacePath, secret, prompt string) (chan generationUpdate, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	updates := make(chan generationUpdate, 32)
	go func() {
		reporter := &generationReporter{ctx: ctx, updates: updates, secret: secret}
		session.SetPermissionApprover(reporter)
		err := updateProjectStatus(workspacePath, "interrupted")
		if err == nil {
			err = session.Turn(ctx, workspacePath, prompt, reporter)
		}
		if err == nil {
			err = updateProjectStatus(workspacePath, "ready")
		}
		updates <- generationUpdate{err: err, done: true}
	}()
	return updates, cancel
}
