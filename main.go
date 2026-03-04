package main

import (
	"go-devops/cmd"
	"os"
)

func main() {
	app := cmd.NewApp()
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
