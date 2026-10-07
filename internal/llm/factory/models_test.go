package factory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"projemble/internal/llm"
	"reflect"
	"testing"
)

func TestModelCatalogPaginationAuthenticationAndFiltering(t *testing.T) {
	for _, provider := range []llm.ProviderID{llm.Mistral, llm.Claude, llm.Gemini} {
		t.Run(string(provider), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch provider {
				case llm.Claude:
					if r.Header.Get("x-api-key") != "secret" || r.Header.Get("anthropic-version") == "" {
						t.Error("missing Claude authentication")
					}
				case llm.Gemini:
					if r.Header.Get("x-goog-api-key") != "secret" {
						t.Error("missing Gemini authentication")
					}
				default:
					if r.Header.Get("Authorization") != "Bearer secret" {
						t.Error("missing bearer authentication")
					}
				}
				if provider == llm.Gemini {
					if r.URL.Query().Get("pageToken") == "" {
						w.Write([]byte(`{"models":[{"name":"models/chat-b","supportedGenerationMethods":["generateContent"]},{"name":"models/embedding","supportedGenerationMethods":["embedContent"]}],"nextPageToken":"next"}`))
					} else {
						w.Write([]byte(`{"models":[{"name":"models/chat-a","supportedGenerationMethods":["generateContent"]}]}`))
					}
					return
				}
				if r.URL.Query().Get("after_id") == "" {
					w.Write([]byte(`{"data":[{"id":"chat-b"},{"id":"embedding","capabilities":{"completion_chat":false}}],"has_more":true,"last_id":"next"}`))
				} else {
					w.Write([]byte(`{"data":[{"id":"chat-a"},{"id":"chat-b"}]}`))
				}
			}))
			defer server.Close()
			got, err := ListModels(context.Background(), llm.Config{Provider: provider, APIKey: "secret", BaseURL: server.URL, Client: server.Client()})
			if err != nil || !reflect.DeepEqual(got, []string{"chat-a", "chat-b"}) || calls != 2 {
				t.Fatalf("models=%v calls=%d error=%v", got, calls, err)
			}
		})
	}
}

func TestModelCatalogErrorsDoNotExposeCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret", http.StatusUnauthorized) }))
	defer server.Close()
	if _, err := ListModels(context.Background(), llm.Config{Provider: llm.Mistral, APIKey: "secret", BaseURL: server.URL}); err == nil {
		t.Fatal("accepted unauthorized catalog")
	}
	if _, err := ListModels(context.Background(), llm.Config{Provider: llm.OpenAIWeb}); err == nil {
		t.Fatal("claimed ChatGPT discovery")
	}
}

func TestChatGPTModelCatalogUsesVisibleModelsAndDisplayNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer chatgpt-token" {
			t.Fatalf("request path/auth = %q/%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"slug":"gpt-test","display_name":"GPT Test","visibility":"list"},{"slug":"internal-model","display_name":"Internal","visibility":"hide"},{"slug":"gpt-next","display_name":"","visibility":"list"}]}`))
	}))
	defer server.Close()

	got, err := ListChatGPTModels(context.Background(), llm.Config{
		Provider:    llm.OpenAIWeb,
		BaseURL:     server.URL + "/v1",
		Client:      server.Client(),
		Credentials: modelToken("chatgpt-token"),
	})
	want := []ModelOption{{ID: "gpt-test", DisplayName: "GPT Test"}, {ID: "gpt-next", DisplayName: "gpt-next"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("models=%+v error=%v, want %+v", got, err, want)
	}
}

type modelToken string

func (token modelToken) AccessToken(context.Context) (string, error) { return string(token), nil }
