package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"

	"example.com/sample-app/internal/job"
)

func extract(context.Context) error {
	log.Println("TODO: implement extraction")
	return nil
}

func transform(context.Context) error {
	log.Println("TODO: implement transformation")
	return nil
}

func load(context.Context) error {
	log.Println("TODO: implement loading")
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pipeline := job.NewPipeline(
		job.StepFunc{StepName: "extract", RunFunc: extract},
		job.StepFunc{StepName: "transform", RunFunc: transform},
		job.StepFunc{StepName: "load", RunFunc: load},
	)
	if err := pipeline.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
