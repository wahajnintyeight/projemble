package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"projemble/internal/llm"
)

type concurrentWorkerProvider struct {
	active  atomic.Int32
	arrived atomic.Int32
	peak    atomic.Int32
	ready   chan struct{}
	once    sync.Once
}

func (provider *concurrentWorkerProvider) Complete(ctx context.Context, request llm.Request) (llm.Response, error) {
	current := provider.active.Add(1)
	defer provider.active.Add(-1)
	for peak := provider.peak.Load(); current > peak; peak = provider.peak.Load() {
		if provider.peak.CompareAndSwap(peak, current) {
			break
		}
	}
	if provider.arrived.Add(1) == maxParallelAgents {
		provider.once.Do(func() { close(provider.ready) })
	}
	select {
	case <-provider.ready:
	case <-ctx.Done():
		return llm.Response{}, ctx.Err()
	}
	task := "worker"
	for _, message := range request.Messages {
		if message.Role == "user" {
			task = message.Content
			break
		}
	}
	return llm.Response{Message: llm.Message{Role: "assistant", Content: "Verified " + task}, Usage: llm.Usage{Available: true, InputTokens: 7, OutputTokens: 3, TotalTokens: 10}}, nil
}

func TestSwarmRunsThreeIndependentWorkersAndAggregatesUsage(t *testing.T) {
	provider := &concurrentWorkerProvider{ready: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var output strings.Builder
	result, usage, err := runSwarm(ctx, t.TempDir(), provider, Config{ProviderID: llm.OpenAI, Model: "test-model"}, `{"tasks":["test service A","test service B","probe API C"]}`, &output)
	if err != nil {
		t.Fatal(err)
	}
	if got := provider.peak.Load(); got != maxParallelAgents {
		t.Fatalf("peak concurrent workers = %d, want %d", got, maxParallelAgents)
	}
	for _, task := range []string{"test service A", "test service B", "probe API C"} {
		if !strings.Contains(result, "Verified "+task) {
			t.Errorf("result missing %q: %s", task, result)
		}
	}
	if !usage.Available || usage.InputTokens != 21 || usage.OutputTokens != 9 || usage.TotalTokens != 30 {
		t.Fatalf("aggregated usage = %+v", usage)
	}
	if !strings.Contains(output.String(), "Worker 1 · Starting:") || !strings.Contains(output.String(), "Worker 3 · Finished (complete)") {
		t.Fatalf("worker activity missing labels: %s", output.String())
	}
}

func TestAgentDelegatesAndReturnsWorkerResultsToLead(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.com/swarm\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var workers atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode provider request: %v", err)
			return
		}
		worker := strings.Contains(request.Messages[0].Content, "verification worker")
		if worker {
			if len(request.Tools) != 4 {
				t.Errorf("worker tools = %d, want exactly the four read-only tools", len(request.Tools))
			}
			w.Header().Set("Content-Type", "application/json")
			if len(request.Messages) == 2 {
				workers.Add(1)
				if request.Messages[1].Content == "verify service A" {
					_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"bad-check","type":"function","function":{"name":"run_check","arguments":"{\"operation\":\"test\",\"target\":\"../outside\"}"}}]}}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Confirmed assigned check with evidence."}}]}`))
				return
			}
			if !strings.Contains(request.Messages[len(request.Messages)-1].Content, "tool error:") {
				t.Error("failed worker action was not returned to its model")
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"The invalid check was rejected."}}]}`))
			return
		}
		if len(request.Messages) > 1 && request.Messages[len(request.Messages)-1].Role == "user" {
			found := false
			for _, tool := range request.Tools {
				found = found || tool.Function.Name == "delegate_checks"
			}
			if !found {
				t.Error("lead request did not advertise the swarm tool")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"swarm-1","type":"function","function":{"name":"delegate_checks","arguments":"{\"tasks\":[\"verify service A\",\"verify service B\",\"verify API C\"]}"}}]}}]}`))
			return
		}
		found := false
		for _, message := range request.Messages {
			found = found || (message.Role == "tool" && strings.Contains(message.Content, "Confirmed assigned check"))
		}
		if !found {
			t.Error("worker summaries were not returned to the lead")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Three workers verified the project."}}]}`))
	}))
	defer server.Close()
	client, err := New(Config{ProviderID: llm.OpenAI, BaseURL: server.URL, Model: "test-model", APIKey: "test-key", AccessMode: AccessFull, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := client.Run(context.Background(), workspace, "Check the API", &output); err != nil {
		t.Fatal(err)
	}
	if workers.Load() != 3 {
		t.Fatalf("worker requests = %d, want 3", workers.Load())
	}
	if !strings.Contains(output.String(), "Verification swarm finished · 3 workers · 1 failed") || !strings.Contains(output.String(), "WORKER CHECKS FAILED: 1") || !strings.Contains(output.String(), "Three workers verified the project") {
		t.Fatalf("lead did not report swarm results: %s", output.String())
	}
}

func TestSwarmRejectsOutOfBoundsAndUnsafeWorkerTools(t *testing.T) {
	if _, err := decodeSwarmTasks(`{"tasks":["a","b","c","d"]}`); err == nil {
		t.Fatal("swarm accepted more than three workers")
	}
	if _, err := runWorkerTool(context.Background(), t.TempDir(), "run_shell", `{"command":"echo unsafe"}`, "", "", nil); err == nil {
		t.Fatal("worker accepted shell access")
	}
	if _, err := runWorkerTool(context.Background(), t.TempDir(), "write_file", `{"path":"note.txt","content":"unsafe"}`, "", "", nil); err == nil {
		t.Fatal("worker accepted file writes")
	}
	if action, err := permissionForTool("delegate_checks", `{"tasks":["test API"]}`); err != nil || action.Action != "Spawn verification agents" {
		t.Fatalf("swarm permission = %+v, err=%v", action, err)
	}
}

func TestProbeLoopbackHTTPIsBoundedAndReadOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/large" {
			_, _ = fmt.Fprint(w, strings.Repeat("x", maxHTTPProbeBody*2))
			return
		}
		_, _ = fmt.Fprint(w, `{"healthy":true}`)
	}))
	defer server.Close()
	result, err := probeLoopbackHTTP(context.Background(), `{"method":"GET","url":"`+server.URL+`/health?token=hidden"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "200 OK") || !strings.Contains(result, `{"healthy":true}`) || strings.Contains(result, "token=hidden") {
		t.Fatalf("probe result was not safely summarized: %s", result)
	}
	large, err := probeLoopbackHTTP(context.Background(), `{"method":"GET","url":"`+server.URL+`/large"}`)
	if err != nil || !strings.Contains(large, "response body truncated at 16 KiB") {
		t.Fatalf("large response was not capped: len=%d err=%v", len(large), err)
	}
	failedServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	failedURL := failedServer.URL + "/health?token=hidden"
	failedServer.Close()
	if _, err := probeLoopbackHTTP(context.Background(), `{"method":"GET","url":"`+failedURL+`"}`); err == nil || strings.Contains(err.Error(), "token=hidden") {
		t.Fatalf("failed probe did not redact query parameters: err=%v", err)
	}
	for _, raw := range []string{
		`{"method":"POST","url":"` + server.URL + `/change"}`,
		`{"method":"GET","url":"https://example.com/"}`,
	} {
		if _, err := probeLoopbackHTTP(context.Background(), raw); err == nil {
			t.Errorf("unsafe probe was accepted: %s", raw)
		}
	}
}
