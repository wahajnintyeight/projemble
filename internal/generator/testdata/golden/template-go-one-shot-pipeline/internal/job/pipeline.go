package job

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
