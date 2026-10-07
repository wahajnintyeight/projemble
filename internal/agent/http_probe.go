package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxHTTPProbeBody = 16 << 10

func probeLoopbackHTTP(ctx context.Context, raw string) (string, error) {
	args, err := decodeToolArgs(raw, []string{"method", "url"}, []string{"method", "url"})
	if err != nil {
		return "", err
	}
	method := strings.ToUpper(args["method"])
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		return "", errors.New("HTTP probes only allow GET, HEAD, or OPTIONS")
	}
	if len(args["url"]) > 2048 {
		return "", errors.New("HTTP probe URL exceeds 2 KiB")
	}
	parsed, err := url.ParseRequestURI(args["url"])
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return "", errors.New("HTTP probe requires an absolute HTTP(S) URL without credentials")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return "", errors.New("HTTP probes are restricted to localhost and loopback IP addresses")
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	transport := &http.Transport{Proxy: nil, MaxConnsPerHost: 1}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Timeout:       8 * time.Second,
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "", fmt.Errorf("HTTP probe to %s was cancelled", safeEndpoint(parsed.String()))
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("HTTP probe to %s timed out", safeEndpoint(parsed.String()))
		}
		return "", fmt.Errorf("HTTP probe to %s failed; query parameters were redacted", safeEndpoint(parsed.String()))
	}
	defer response.Body.Close()
	var body []byte
	if method != http.MethodHead {
		body, err = io.ReadAll(io.LimitReader(response.Body, maxHTTPProbeBody+1))
		if err != nil {
			return "", fmt.Errorf("read HTTP probe response: %w", err)
		}
	}
	truncated := len(body) > maxHTTPProbeBody
	if truncated {
		body = body[:maxHTTPProbeBody]
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	result := fmt.Sprintf("HTTP %s %s://%s%s -> %s", method, parsed.Scheme, parsed.Host, path, response.Status)
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		result += " (" + contentType + ")"
	}
	if len(body) > 0 {
		result += "\n" + string(body)
	}
	if truncated {
		result += "\n[response body truncated at 16 KiB]"
	}
	return result, nil
}
