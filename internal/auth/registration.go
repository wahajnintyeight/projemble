package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type registrationAttempt struct {
	clientID        string
	newRegistration bool
	retryingPending bool
}

// Explicit account/workspace selection must never resume a workspace-bound client.
func chooseRegistration(forceNew bool, previousID, pendingID string) registrationAttempt {
	if forceNew {
		return registrationAttempt{clientID: "dynamic_agent_client", newRegistration: true}
	}
	if previousID != "" {
		return registrationAttempt{clientID: previousID}
	}
	if pendingID != "" {
		return registrationAttempt{clientID: pendingID, retryingPending: true}
	}
	return registrationAttempt{clientID: "dynamic_agent_client", newRegistration: true}
}

type tokenGrantError struct{ err error }

func (e *tokenGrantError) Error() string { return e.err.Error() }
func (e *tokenGrantError) Unwrap() error { return e.err }

func login(ctx context.Context, output io.Writer, forceNew bool) error {
	if output == nil {
		output = io.Discard
	}
	previous, _ := loadCredential()
	pending, _ := loadPendingRegistration()
	if forceNew {
		if err := clearPendingRegistration(); err != nil {
			return fmt.Errorf("reset pending ChatGPT registration: %w", err)
		}
	}
	attempt := chooseRegistration(forceNew, previous.ClientID, pending.ClientID)
	return signInWithRetry(ctx, output, attempt, loginAttempt)
}

// Retry a rejected code once, using a new listener, state, nonce and PKCE verifier.
// Retain the issued client only inside this sign-in operation; an explicit new
// account/workspace action always starts dynamic registration again.
func signInWithRetry(ctx context.Context, output io.Writer, attempt registrationAttempt, perform func(context.Context, io.Writer, registrationAttempt) error) error {
	err := perform(ctx, output, attempt)
	var rejected *tokenGrantError
	if ctx.Err() != nil || !errors.As(err, &rejected) {
		return err
	}
	if attempt.newRegistration {
		pending, loadErr := loadPendingRegistration()
		if loadErr != nil || pending.ClientID == "" {
			return err
		}
		attempt = registrationAttempt{clientID: pending.ClientID, retryingPending: true}
	}
	return perform(ctx, output, attempt)
}
