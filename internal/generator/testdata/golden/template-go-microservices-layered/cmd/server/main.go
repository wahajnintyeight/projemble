package main

import (
	"context"
	"log"

	"go-micro.dev/v6"
	_ "go-micro.dev/v6/cmd/defaults"

	app "example.com/sample-app/internal/service"
)

type HealthRequest struct{}

type HealthResponse struct {
	Status string
}

type HealthService struct{}

func (*HealthService) Check(_ context.Context, _ *HealthRequest, response *HealthResponse) error {
	response.Status = app.Health()
	return nil
}

func main() {
	service := micro.NewService("projemble.sample.app")
	service.Init()
	if err := service.Handle(&HealthService{}); err != nil {
		log.Fatal(err)
	}
	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}
