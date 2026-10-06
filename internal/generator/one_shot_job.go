package generator

import (
	"fmt"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

type oneShotJobRenderer struct{}

func (oneShotJobRenderer) Files(project projectstore.Project, template catalog.Template, module string) map[string]string {
	files := baseProjectFiles(project, template, module)
	files["cmd/job/main.go"] = oneShotMain(module)
	files["internal/job/pipeline.go"] = oneShotPipeline()
	return files
}

func oneShotMain(module string) string {
	return fmt.Sprintf(`package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"

	"%s/internal/job"
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
`, module)
}

func oneShotPipeline() string {
	return `package job

import (
	"context"
	"fmt"
)

type Step interface {
	Name() string
	Run(context.Context) error
}

type StepFunc struct {
	StepName string
	RunFunc  func(context.Context) error
}

func (step StepFunc) Name() string { return step.StepName }

func (step StepFunc) Run(ctx context.Context) error {
	if step.RunFunc == nil {
		return fmt.Errorf("step %q has no implementation", step.Name())
	}
	return step.RunFunc(ctx)
}

type Pipeline struct {
	steps []Step
}

func NewPipeline(steps ...Step) Pipeline {
	return Pipeline{steps: append([]Step(nil), steps...)}
}

func (pipeline Pipeline) Run(ctx context.Context) error {
	for _, step := range pipeline.steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := step.Run(ctx); err != nil {
			return fmt.Errorf("%s: %w", step.Name(), err)
		}
	}
	return ctx.Err()
}
`
}
