package ports

import "example.com/sample-app/internal/domain"

type Greeter interface {
	Greet() domain.Greeting
}
