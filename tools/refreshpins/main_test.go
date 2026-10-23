package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRefreshReportsWithoutEditing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/@latest") {
			fmt.Fprint(w, `{"Version":"v9.0.0"}`)
			return
		}
		fmt.Fprint(w, "module example.com/test\n\ngo\t1.27.0\n")
	}))
	defer server.Close()
	var output bytes.Buffer
	if err := refresh(context.Background(), server.Client(), server.URL, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "-> v9.0.0; go 1.26.0 -> 1.27.0") {
		t.Fatal(output.String())
	}
	if escape("example.com/ABC") != "example.com/!a!b!c" {
		t.Fatal("uppercase path not escaped")
	}
}
func TestProxyFailuresAndLimits(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if oversized {
				fmt.Fprint(w, strings.Repeat("x", (1<<20)+1))
			} else {
				w.WriteHeader(500)
			}
		}))
		_, err := get(context.Background(), server.Client(), server.URL)
		server.Close()
		if err == nil {
			t.Fatal("expected proxy failure")
		}
	}
}
