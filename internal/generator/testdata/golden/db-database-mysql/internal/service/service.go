package service

import "example.com/sample-app/internal/repository"

func Hello() string {
	return "Project is running. " + repository.Name()
}

func Health() string {
	return "ok"
}
