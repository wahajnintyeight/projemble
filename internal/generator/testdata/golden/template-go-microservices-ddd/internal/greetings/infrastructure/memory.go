package infrastructure

import "example.com/sample-app/internal/greetings/domain"

type Repository struct{}

func New() Repository {
	return Repository{}
}

func (Repository) Greeting() domain.Greeting {
	return domain.NewGreeting("Project is running.")
}
