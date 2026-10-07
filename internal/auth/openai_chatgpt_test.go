package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatTokenError(t *testing.T) {
	tests := []struct {
		name     string
		response tokenResponse
		want     string
	}{
		{
			name:     "includes provider error code and description",
			response: tokenResponse{Error: "invalid_client", ErrorDescription: "client registration failed"},
			want:     "HTTP 400: invalid_client: client registration failed",
		},
		{
			name:     "guides expired authorization code recovery",
			response: tokenResponse{Error: "invalid_grant"},
			want:     "HTTP 400: invalid_grant: authorization code was rejected or expired; discard this code and retry ChatGPT sign-in",
		},
		{
			name:     "preserves provider rejection detail",
			response: tokenResponse{Error: "invalid_grant", ErrorDescription: "the code verifier did not match"},
			want:     "HTTP 400: invalid_grant: the code verifier did not match; discard this code and retry ChatGPT sign-in",
		},
		{
			name:     "handles empty error body",
			response: tokenResponse{},
			want:     "HTTP 400: token endpoint rejected the request without an error description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTokenError(400, tt.response).Error(); got != tt.want {
				t.Fatalf("formatTokenError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkspaceScopeErrorExplainsAvailablePaths(t *testing.T) {
	err := chatGPTAuthorizationError("3p_login_workspace_scope_denied", "")
	message := err.Error()
	for _, expected := range []string{"workspace restriction", "new-registration", "Local templates", "OpenAI API key", "eligible account"} {
		if !strings.Contains(message, expected) {
			t.Errorf("workspace error %q does not explain %q", message, expected)
		}
	}
}

func TestAccessDeniedExplainsRetryAndKeepsErrorPrefixSingular(t *testing.T) {
	err := chatGPTAuthorizationError("access_denied", "")
	if got, want := err.Error(), "sign-in was declined or cancelled; choose another provider or retry"; got != want {
		t.Fatalf("chatGPTAuthorizationError() = %q, want %q", got, want)
	}
	if got := chatGPTAuthorizationError("access_denied", "workspace policy").Error(); got != "sign-in was declined or cancelled: workspace policy" {
		t.Fatalf("chatGPTAuthorizationError() dropped provider details: %q", got)
	}
}

func TestPendingRegistrationPersistsClientIDForRetry(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	want := pendingRegistration{ClientID: "oaiapp_test-registration"}
	if err := savePendingRegistration(want); err != nil {
		t.Fatalf("savePendingRegistration() error = %v", err)
	}

	got, err := loadPendingRegistration()
	if err != nil {
		t.Fatalf("loadPendingRegistration() error = %v", err)
	}
	if got != want {
		t.Fatalf("loadPendingRegistration() = %#v, want %#v", got, want)
	}

	path, err := pendingRegistrationPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(info.Name()) != "openai-chatgpt-pending.json" {
		t.Fatalf("unexpected pending registration file: %s", info.Name())
	}

	if err := clearPendingRegistration(); err != nil {
		t.Fatalf("clearPendingRegistration() error = %v", err)
	}
	if _, err := loadPendingRegistration(); !os.IsNotExist(err) {
		t.Fatalf("load after clear error = %v, want os.ErrNotExist", err)
	}
}

func TestRegistrationRetryReusesIssuedClientID(t *testing.T) {
	tests := []struct {
		name                  string
		forceNew              bool
		previousID, pendingID string
		want                  registrationAttempt
	}{
		{
			name:     "new registration starts with dynamic client",
			forceNew: true,
			want:     registrationAttempt{clientID: "dynamic_agent_client", newRegistration: true},
		},
		{
			name:       "explicit new account ignores workspace bound pending registration",
			forceNew:   true,
			previousID: "oaiapp_saved",
			pendingID:  "oaiapp_pending",
			want:       registrationAttempt{clientID: "dynamic_agent_client", newRegistration: true},
		},
		{
			name:       "saved registration stays selected",
			previousID: "oaiapp_saved",
			pendingID:  "oaiapp_pending",
			want:       registrationAttempt{clientID: "oaiapp_saved"},
		},
		{
			name:      "pending registration resumes without saved account",
			pendingID: "oaiapp_pending",
			want:      registrationAttempt{clientID: "oaiapp_pending", retryingPending: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := chooseRegistration(test.forceNew, test.previousID, test.pendingID); got != test.want {
				t.Fatalf("chooseRegistration() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestEnsureChatGPTDoesNotRestartLoginForCorruptSavedCredentials(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	path, err := credentialPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = EnsureChatGPT(ctx, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("EnsureChatGPT() error = %v, want credential parse error", err)
	}
}
