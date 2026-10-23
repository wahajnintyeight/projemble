package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"projemble/internal/catalog"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := refresh(ctx, &http.Client{Timeout: 15 * time.Second}, "https://proxy.golang.org", os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func refresh(ctx context.Context, client *http.Client, proxy string, out io.Writer) error {
	failed := false
	for _, module := range catalog.Modules() {
		path := escape(module.Path)
		latest, err := get(ctx, client, proxy+"/"+path+"/@latest")
		var info struct{ Version string }
		if err == nil {
			err = json.Unmarshal(latest, &info)
		}
		if err == nil && (info.Version == "" || strings.ContainsAny(info.Version, "/\\?#")) {
			err = fmt.Errorf("invalid version response")
		}
		var mod []byte
		if err == nil {
			mod, err = get(ctx, client, proxy+"/"+path+"/@v/"+escape(info.Version)+".mod")
		}
		if err != nil {
			failed = true
			fmt.Fprintf(out, "%s: %v\n", module.Path, err)
			continue
		}
		minimum := goDirective(string(mod))
		fmt.Fprintf(out, "%s: %s -> %s; go %s -> %s\n", module.Path, module.Version, info.Version, module.MinGo, minimum)
	}
	if failed {
		return fmt.Errorf("some module checks failed; registry was not changed")
	}
	return nil
}
func get(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy returned %s", response.Status)
	}
	const limit = 1 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("proxy response exceeds 1 MiB")
	}
	return data, nil
}
func escape(path string) string {
	var result strings.Builder
	for _, r := range path {
		if r >= 'A' && r <= 'Z' {
			result.WriteByte('!')
			r = unicode.ToLower(r)
		}
		result.WriteRune(r)
	}
	return result.String()
}
func goDirective(mod string) string {
	for _, line := range strings.Split(mod, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "go" {
			return fields[1]
		}
	}
	return "1.16 (implicit)"
}
