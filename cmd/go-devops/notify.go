package main

import (
	"github.com/gen2brain/beeep"
	"github.com/leganck/go-devops/internal/logger"
)

func sendNotification(title, body string) {
	beeep.AppName = "go-devops"
	if err := beeep.Notify(title, body, ""); err != nil {
		logger.Warningf("notification failed: %v", err)
	}
}
