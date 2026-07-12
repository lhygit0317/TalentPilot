package main

import (
	"log"

	"github.com/talentpilot/talentpilot/apps/api/internal/app"
	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/auth"
	authw3 "github.com/talentpilot/talentpilot/apps/api/internal/auth/w3"
	"github.com/talentpilot/talentpilot/apps/api/internal/config"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/matching"
	"github.com/talentpilot/talentpilot/apps/api/internal/organization"
	"github.com/talentpilot/talentpilot/apps/api/internal/platform/db"
	"github.com/talentpilot/talentpilot/apps/api/internal/resume"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(db.Config{Driver: cfg.DatabaseDriver, DSN: cfg.DatabaseDSN})
	if err != nil {
		log.Fatal(err)
	}

	authStore := auth.NewSQLStore(database)
	authService := auth.NewService(auth.ServiceConfig{
		W3:    newW3Adapter(cfg),
		Store: authStore,
	})
	iamStore := iam.NewSQLStore(database)
	iamService := iam.NewService(iamStore)
	auditRecorder := audit.NewSQLRecorder(database)
	resumeStore := resume.NewSQLStore(database)
	resumeService := resume.NewService(resumeStore, resume.NewPDFParser(), auditRecorder)
	organizationStore := organization.NewSQLStore(database)
	organizationService := organization.NewService(organizationStore, auditRecorder)
	matchingStore := matching.NewSQLStore(database)
	matchingService := matching.NewService(matchingStore, auditRecorder, matching.NewRuleQuestionGenerator())
	server := app.NewServerWithOptions(app.Options{
		AuthService:         authService,
		IAMService:          iamService,
		ResumeService:       resumeService,
		OrganizationService: organizationService,
		MatchingService:     matchingService,
		FrontendOrigin:      cfg.FrontendOrigin,
		RequireHTTPS:        cfg.Env == "production",
		SecureCookies:       cfg.SecureCookies,
		TrustForwardedProto: cfg.TrustForwardedProto,
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
