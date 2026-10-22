package application

import (
	"example.com/sample-app/internal/domain"
	"example.com/sample-app/internal/ports"
)

type Service struct {
	greeter ports.Greeter
}

func New(greeter ports.Greeter) Service {
	return Service{greeter: greeter}
}

func (s Service) Greet() domain.Greeting {
	return s.greeter.Greet()
}

func Health() string {
	return "ok"
}
