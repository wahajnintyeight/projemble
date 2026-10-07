package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"projemble/internal/llm"
)

const (
	issuer            = "https://auth.openai.com"
	authorizeEndpoint = issuer + "/api/accounts/authorize"
	tokenEndpoint     = issuer + "/api/accounts/oauth/token"
	resource          = "https://api.openai.com/v1"
	requiredScope     = "chatgpt.tokens.use.direct"
)

type credentials struct {
	ClientID     string    `json:"client_id"`
	HostID       string    `json:"ext_agent_host_id"`
	Subject      string    `json:"subject"`
	Email        string    `json:"email,omitempty"`
	IDToken      string    `json:"id_token"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Scopes       []string  `json:"scopes"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type pendingRegistration struct {
	ClientID string `json:"client_id"`
}

// Login uses OpenAI's documented public-client Sign in with ChatGPT flow.
func Login(ctx context.Context, output io.Writer) error { return login(ctx, output, false) }

// LoginNewRegistration starts a fresh ChatGPT client registration, allowing
// the user to choose a different account or workspace than the saved client.
func LoginNewRegistration(ctx context.Context, output io.Writer) error {
	return login(ctx, output, true)
}

func EnsureChatGPT(ctx context.Context, output io.Writer) (*ChatGPTTokenSource, error) {
	source := &ChatGPTTokenSource{}
	if _, err := loadCredential(); err == nil {
		if _, err := source.AccessToken(ctx); err != nil {
			return nil, err
		}
		return source, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := Login(ctx, output); err != nil {
		return nil, err
	}
	if _, err := source.AccessToken(ctx); err != nil {
		return nil, err
	}
	return source, nil
}

func loginAttempt(ctx context.Context, output io.Writer, attempt registrationAttempt) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start local sign-in callback: %w", err)
	}
	defer listener.Close()
	redirect := "http://127.0.0.1:" + fmt.Sprint(listener.Addr().(*net.TCPAddr).Port) + "/auth/callback"
	state, err := randomToken(32)
	if err != nil {
		return err
	}
	nonce, err := randomToken(32)
	if err != nil {
		return err
	}
	verifier, err := randomToken(48)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(verifier))
	hostID, err := loadOrCreateHostID()
	if err != nil {
		return err
	}
	previous, _ := loadCredential()
	clientID, newRegistration := attempt.clientID, attempt.newRegistration

	callback := make(chan url.Values, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.RemoteAddr == "" || !strings.HasPrefix(r.RemoteAddr, "127.0.0.1:") {
			http.Error(w, "Local callback only", http.StatusForbidden)
			return
		}
		values := r.URL.Query()
		if values.Get("state") != state {
			http.Error(w, "Sign-in state mismatch", http.StatusBadRequest)
			return
		}
		select {
		case callback <- values:
		default:
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Projemble sign-in received. You can close this tab and return to the terminal."))
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer server.Shutdown(context.Background())

	query := url.Values{
		"client_id": {clientID}, "ext_agent_host_id": {hostID},
		"response_type": {"code"}, "redirect_uri": {redirect},
		"scope":    {"openid profile email offline_access resource.invoke chatgpt.tokens.use.direct"},
		"resource": {resource}, "state": {state}, "nonce": {nonce},
		"code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])},
	}
	if newRegistration {
		query.Set("agent_name_hint", "Projemble")
	} else if !attempt.retryingPending && previous.IDToken != "" {
		query.Set("id_token_hint", previous.IDToken)
	}
	if err := openBrowser(authorizeEndpoint + "?" + query.Encode()); err != nil {
		return fmt.Errorf("open browser for ChatGPT sign-in: %w", err)
	}
	var values url.Values
	select {
	case values = <-callback:
	case <-ctx.Done():
		return ctx.Err()
	}
	if providerErr := values.Get("error"); providerErr != "" {
		return chatGPTAuthorizationError(providerErr, values.Get("error_description"))
	}
	code, returnedClientID := values.Get("code"), values.Get("client_id")
	if code == "" {
		return errors.New("ChatGPT sign-in callback is missing its authorization code")
	}
	if newRegistration && returnedClientID == "" {
		return errors.New("ChatGPT registration callback is missing its issued client ID")
	}
	if !newRegistration && returnedClientID != "" && returnedClientID != clientID {
		return errors.New("ChatGPT returned a different client ID than the selected account")
	}
	if returnedClientID != "" {
		clientID = returnedClientID
	}
	if newRegistration {
		if err := savePendingRegistration(pendingRegistration{ClientID: clientID}); err != nil {
			return fmt.Errorf("save ChatGPT registration for retry: %w", err)
		}
	}
	tokens, err := exchange(ctx, url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code}, "code_verifier": {verifier}, "redirect_uri": {redirect}, "resource": {resource}})
	if err != nil {
		return fmt.Errorf("exchange ChatGPT sign-in code: %w", err)
	}
	if tokens.IDToken == "" || tokens.RefreshToken == "" {
		return errors.New("ChatGPT token response was missing an ID token or refresh token")
	}
	if !hasScope(tokens.Scope, requiredScope) {
		return errors.New("ChatGPT did not grant chatgpt.tokens.use.direct; enable ChatGPT plan usage and sign in again")
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return fmt.Errorf("load OpenAI identity provider: %w", err)
	}
	verifierOIDC := provider.Verifier(&oidc.Config{ClientID: clientID})
	idToken, err := verifierOIDC.Verify(ctx, tokens.IDToken)
	if err != nil {
		return fmt.Errorf("validate ChatGPT identity token: %w", err)
	}
	if idToken.Nonce != nonce {
		return errors.New("ChatGPT identity token nonce mismatch")
	}
	if !newRegistration && !attempt.retryingPending && previous.Subject != "" && previous.Subject != idToken.Subject {
		return errors.New("ChatGPT account changed during reauthorization; existing credentials were preserved")
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return fmt.Errorf("read ChatGPT identity claims: %w", err)
	}
	credential := credentials{ClientID: clientID, HostID: hostID, Subject: idToken.Subject, Email: claims.Email, IDToken: tokens.IDToken, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, Scopes: strings.Fields(tokens.Scope), ExpiresAt: time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)}
	if err := saveCredential(credential); err != nil {
		return err
	}
	_ = clearPendingRegistration()
	_, _ = fmt.Fprintf(output, "Signed in to ChatGPT%s.\n", accountHint(credential.Email))
	return nil
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	IDToken          string `json:"id_token"`
	Scope            string `json:"scope"`
	ExpiresIn        int64  `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func exchange(ctx context.Context, values url.Values) (tokenResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return tokenResponse{}, err
	}
	defer response.Body.Close()
	var tokens tokenResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&tokens); err != nil {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return tokenResponse{}, fmt.Errorf("HTTP %d: token endpoint returned an unreadable error response", response.StatusCode)
		}
		return tokenResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return tokenResponse{}, formatTokenError(response.StatusCode, tokens)
	}
	if tokens.AccessToken == "" || tokens.ExpiresIn <= 0 {
		return tokenResponse{}, errors.New("token response was incomplete")
	}
	return tokens, nil
}

func formatTokenError(status int, response tokenResponse) error {
	code := cleanProviderError(response.Error)
	description := cleanProviderError(response.ErrorDescription)
	if code == "invalid_grant" {
		if description != "" {
			return &tokenGrantError{fmt.Errorf("HTTP %d: invalid_grant: %s; discard this code and retry ChatGPT sign-in", status, description)}
		}
		return &tokenGrantError{fmt.Errorf("HTTP %d: invalid_grant: authorization code was rejected or expired; discard this code and retry ChatGPT sign-in", status)}
	}
	if code != "" && description != "" {
		return fmt.Errorf("HTTP %d: %s: %s", status, code, description)
	}
	if code != "" {
		return fmt.Errorf("HTTP %d: %s", status, code)
	}
	if description != "" {
		return fmt.Errorf("HTTP %d: %s", status, description)
	}
	return fmt.Errorf("HTTP %d: token endpoint rejected the request without an error description", status)
}

func chatGPTAuthorizationError(code, description string) error {
	code = cleanProviderError(code)
	description = cleanProviderError(description)
	if code == "access_denied" {
		if description != "" {
			return fmt.Errorf("sign-in was declined or cancelled: %s", description)
		}
		return errors.New("sign-in was declined or cancelled; choose another provider or retry")
	}
	if code == "3p_login_workspace_scope_denied" {
		return errors.New("sign-in was blocked by a workspace restriction; this is separate from plan eligibility. If you need another workspace, run `projemble auth login --provider openai-web --new-registration` and select it during sign-in. Otherwise use an account in the authorized workspace, Local templates, an OpenAI API key (separate Platform billing), or another provider. ChatGPT plan usage requires an eligible account")
	}
	if code == "" {
		return errors.New(description)
	}
	if description == "" {
		return fmt.Errorf("authorization failed (%s)", code)
	}
	return fmt.Errorf("authorization failed (%s): %s", code, description)
}

func cleanProviderError(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 300 {
		value = string(runes[:300]) + "…"
	}
	return value
}

type ChatGPTTokenSource struct{ mu sync.Mutex }

func (source *ChatGPTTokenSource) AccessToken(ctx context.Context) (string, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	credential, err := loadCredential()
	if err != nil {
		return "", err
	}
	if credential.ExpiresAt.After(time.Now().Add(2 * time.Minute)) {
		return credential.AccessToken, nil
	}
	values := url.Values{"grant_type": {"refresh_token"}, "client_id": {credential.ClientID}, "refresh_token": {credential.RefreshToken}, "resource": {resource}}
	tokens, err := exchange(ctx, values)
	if err != nil {
		return "", fmt.Errorf("refresh ChatGPT token: %w", err)
	}
	if tokens.Scope != "" {
		credential.Scopes = strings.Fields(tokens.Scope)
	}
	if !contains(credential.Scopes, requiredScope) {
		return "", errors.New("refreshed ChatGPT credential lacks plan usage permission; sign in again")
	}
	credential.AccessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		credential.RefreshToken = tokens.RefreshToken
	}
	if tokens.IDToken != "" {
		credential.IDToken = tokens.IDToken
	}
	credential.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	if err := saveCredential(credential); err != nil {
		return "", err
	}
	return credential.AccessToken, nil
}

var _ llm.TokenSource = (*ChatGPTTokenSource)(nil)

func credentialPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "projemble", "auth", "openai-chatgpt.json"), nil
}
func hostIDPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "projemble", "auth", "host-id"), nil
}
func pendingRegistrationPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "projemble", "auth", "openai-chatgpt-pending.json"), nil
}
func loadCredential() (credentials, error) {
	path, err := credentialPath()
	if err != nil {
		return credentials{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return credentials{}, fmt.Errorf("ChatGPT is not signed in; run `projemble auth login --provider openai-web`: %w", err)
	}
	var value credentials
	err = json.Unmarshal(data, &value)
	return value, err
}
func loadPendingRegistration() (pendingRegistration, error) {
	path, err := pendingRegistrationPath()
	if err != nil {
		return pendingRegistration{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return pendingRegistration{}, err
	}
	var value pendingRegistration
	err = json.Unmarshal(data, &value)
	return value, err
}
func saveCredential(value credentials) error {
	path, err := credentialPath()
	if err != nil {
		return err
	}
	return writePrivate(path, value)
}
func savePendingRegistration(value pendingRegistration) error {
	path, err := pendingRegistrationPath()
	if err != nil {
		return err
	}
	return writePrivate(path, value)
}
func clearPendingRegistration() error {
	path, err := pendingRegistrationPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func loadOrCreateHostID() (string, error) {
	path, err := hostIDPath()
	if err != nil {
		return "", err
	}
	if data, err := os.ReadFile(path); err == nil && strings.HasPrefix(string(data), "urn:uuid:") {
		return string(data), nil
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	id := fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(id), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

func writePrivate(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".credentials-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
func hasScope(scope, wanted string) bool { return contains(strings.Fields(scope), wanted) }
func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
func accountHint(email string) string {
	if email == "" {
		return ""
	}
	return " as " + email
}
func openBrowser(address string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)
	case "darwin":
		command = exec.Command("open", address)
	default:
		command = exec.Command("xdg-open", address)
	}
	return command.Start()
}
