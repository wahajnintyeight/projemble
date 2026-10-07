package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestRejectedRegistrationCodeRetriesWithIssuedClientAndStopsAfterTwoAttempts(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	initial := chooseRegistration(true, "oaiapp_saved", "oaiapp_stale")
	calls := 0
	rejected := &tokenGrantError{errors.New("invalid_grant")}
	err := signInWithRetry(context.Background(), io.Discard, initial, func(_ context.Context, _ io.Writer, attempt registrationAttempt) error {
		calls++
		if calls == 1 {
			if !attempt.newRegistration || attempt.clientID != "dynamic_agent_client" {
				t.Fatalf("fresh selection reused old client: %+v", attempt)
			}
			if err := savePendingRegistration(pendingRegistration{ClientID: "oaiapp_issued"}); err != nil {
				t.Fatal(err)
			}
		} else if attempt.clientID != "oaiapp_issued" || attempt.newRegistration || !attempt.retryingPending {
			t.Fatalf("code retry lost issued registration: %+v", attempt)
		}
		return fmt.Errorf("exchange: %w", rejected)
	})
	if calls != 2 || !errors.Is(err, rejected) {
		t.Fatalf("calls=%d err=%v; expected two attempts and original rejection", calls, err)
	}
}

func TestWorkspaceDeniedDoesNotRetrySignIn(t *testing.T) {
	calls := 0
	denied := chatGPTAuthorizationError("3p_login_workspace_scope_denied", "")
	err := signInWithRetry(context.Background(), io.Discard, chooseRegistration(true, "", ""), func(context.Context, io.Writer, registrationAttempt) error {
		calls++
		return denied
	})
	if calls != 1 || err != denied {
		t.Fatalf("workspace denial retried: calls=%d err=%v", calls, err)
	}
}

func TestSavedAccountCodeRetryRetainsAccountSelection(t *testing.T) {
	selected := chooseRegistration(false, "oaiapp_saved", "oaiapp_stale")
	calls := 0
	err := signInWithRetry(context.Background(), io.Discard, selected, func(_ context.Context, _ io.Writer, attempt registrationAttempt) error {
		calls++
		if attempt != selected {
			t.Fatalf("retry changed selected account: %+v", attempt)
		}
		if calls == 1 {
			return &tokenGrantError{errors.New("invalid_grant")}
		}
		return nil
	})
	if calls != 2 || err != nil {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
