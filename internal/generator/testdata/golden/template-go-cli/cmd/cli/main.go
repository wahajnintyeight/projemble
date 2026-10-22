package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		fmt.Println("Usage: cli [--help]")
		return
	}
	fmt.Println("Project is ready.")
}
