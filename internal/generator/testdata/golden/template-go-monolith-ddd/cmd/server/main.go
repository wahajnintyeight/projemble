package main

import (
	"log"
	"net/http"

	"example.com/sample-app/internal/greetings/application"
	"example.com/sample-app/internal/greetings/infrastructure"
	"example.com/sample-app/internal/greetings/interfaces/httpapi"
)

func main() {
	service := application.New(infrastructure.New())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", httpapi.New(service))
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
