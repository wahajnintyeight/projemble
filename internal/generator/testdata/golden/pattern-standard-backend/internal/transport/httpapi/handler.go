package httpapi

import (
	"fmt"
	"net/http"

	"example.com/sample-app/internal/application"
)

func New(service application.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, service.Greet())
	})
	return mux
}
