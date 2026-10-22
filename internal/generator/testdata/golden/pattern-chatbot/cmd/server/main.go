package main

import (
	"log"
	"net/http"

	"example.com/sample-app/internal/adapters/memory"
	"example.com/sample-app/internal/application"
	"example.com/sample-app/internal/transport/httpapi"
)

func main() {
	service := application.New(memory.New())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", httpapi.New(service))
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
