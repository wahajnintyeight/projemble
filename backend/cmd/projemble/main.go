package main

import (
	"log"
	"os"

	"projemble/internal/cli"
	"projemble/internal/tui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "project" {
		if err := cli.RunProjects(os.Args[2:], os.Stdout, os.Stderr); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := tui.Run(); err != nil {
		log.Fatal(err)
	}
}
