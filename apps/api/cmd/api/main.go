package main

import (
	"log"

	"github.com/talentpilot/talentpilot/apps/api/internal/app"
	"github.com/talentpilot/talentpilot/apps/api/internal/config"
)

func main() {
	cfg := config.Load()
	server := app.NewServer()

	if err := server.Echo.Start(cfg.APIAddr); err != nil {
		log.Fatal(err)
	}
}
