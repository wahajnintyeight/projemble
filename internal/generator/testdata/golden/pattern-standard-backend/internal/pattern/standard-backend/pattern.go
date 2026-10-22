package standard_backend

import "context"

type Handler interface {
	Handle(context.Context, string) (string, error)
}

type Service struct {
	handler Handler
}

func New(handler Handler) Service {
	return Service{handler: handler}
}

func (s Service) Handle(ctx context.Context, input string) (string, error) {
	return s.handler.Handle(ctx, input)
}
