package auth

import (
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
			want:     "HTTP 400: authorization code was rejected or expired; start a fresh ChatGPT sign-in",
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
	for _, expected := range []string{"workspace restriction", "Local templates", "OpenAI API key", "eligible Plus or Pro"} {
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
