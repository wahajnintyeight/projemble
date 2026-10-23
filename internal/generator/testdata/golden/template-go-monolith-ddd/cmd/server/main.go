package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"example.com/sample-app/internal/config"
	"example.com/sample-app/internal/greetings/application"
	"example.com/sample-app/internal/greetings/infrastructure"
	"example.com/sample-app/internal/greetings/interfaces/httpapi"
	"example.com/sample-app/internal/platform/health"
	"example.com/sample-app/internal/platform/httpserver"
	"example.com/sample-app/internal/platform/logging"
	"example.com/sample-app/internal/platform/middleware"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	cfg, err := (config.EnvLoader{Getenv: getenv}).Load()
	if err != nil {
		return err
	}
	log := logging.New(stdout, cfg.LogLevel)
	mux := http.NewServeMux()
	health.Register(mux)
	mux.Handle("/", httpapi.New(application.New(infrastructure.New())))
	server := httpserver.New(cfg, middleware.Wrap(mux, log, cfg.MaxBodyBytes, cfg.RequestTimeout))
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	log.Info("listening", "address", listener.Addr().String())
	return httpserver.Serve(ctx, server, listener, cfg.ShutdownTimeout)
}
