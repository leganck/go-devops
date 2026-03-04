package main

import (
	"fmt"
	"go-devops/cmd"
	"os"
)

func main() {
	app := cmd.NewApp()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
