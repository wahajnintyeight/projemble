package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"projemble/internal/llm"
)

const maxProviderResponse = 2 << 20

type adapter struct {
	config llm.Config
}

func (base adapter) post(ctx context.Context, endpoint string, headers map[string]string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode provider request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := base.config.Client.Do(request)
	if err != nil {
		return fmt.Errorf("call %s: %w", base.config.Provider, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxProviderResponse+1))
	if err != nil {
		return fmt.Errorf("read %s response: %w", base.config.Provider, err)
	}
	if len(data) > maxProviderResponse {
		return fmt.Errorf("%s response exceeds %d bytes", base.config.Provider, maxProviderResponse)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s returned HTTP %d: %s", base.config.Provider, response.StatusCode, clipped(string(data), 2048))
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode %s response: %w", base.config.Provider, err)
	}
	return nil
}

func endpoint(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func bearer(key string) map[string]string {
	if key == "" {
		return nil
	}
	return map[string]string{"Authorization": "Bearer " + key}
}

func clipped(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "...[truncated]"
}
