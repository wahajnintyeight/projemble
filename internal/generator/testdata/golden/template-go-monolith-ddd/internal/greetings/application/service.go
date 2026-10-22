package application

import "example.com/sample-app/internal/greetings/domain"

type Repository interface {
	Greeting() domain.Greeting
}

type Service struct {
	repository Repository
}

func New(repository Repository) Service {
	return Service{repository: repository}
}

func (s Service) Greet() domain.Greeting {
	return s.repository.Greeting()
}

func Health() string {
	return "ok"
}
