package main

import (
	"fmt"
	"os"

	"github.com/leganck/go-devops/internal/logger"
)

func main() {
	app := newApp()
	if err := app.Run(os.Args); err != nil {
		logger.Errorf("%v", err)
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
