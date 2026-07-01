package main

import (
	"log"

	"github.com/talentpilot/talentpilot/apps/api/internal/app"
	"github.com/talentpilot/talentpilot/apps/api/internal/auth"
	authw3 "github.com/talentpilot/talentpilot/apps/api/internal/auth/w3"
	"github.com/talentpilot/talentpilot/apps/api/internal/config"
	"github.com/talentpilot/talentpilot/apps/api/internal/platform/db"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(db.Config{Driver: cfg.DatabaseDriver, DSN: cfg.DatabaseDSN})
	if err != nil {
		log.Fatal(err)
	}

	store := auth.NewSQLStore(database)
	authService := auth.NewService(auth.ServiceConfig{
		W3:    newW3Adapter(cfg),
		Store: store,
	})
	server := app.NewServerWithOptions(app.Options{
		AuthService:    authService,
		FrontendOrigin: cfg.FrontendOrigin,
		SecureCookies:  cfg.SecureCookies,
	})

	if err := server.Echo.Start(cfg.APIAddr); err != nil {
		log.Fatal(err)
	}
}

func newW3Adapter(cfg config.Config) auth.W3Adapter {
	if cfg.Env != "production" && cfg.W3Mode == "mock" {
		return authw3.NewMockAdapter()
	}
	return authw3.NewUnavailableAdapter()
}
