package app

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humaecho"
	"github.com/labstack/echo/v4"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
)

type Server struct {
	Echo *echo.Echo
	API  huma.API
}

type healthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

func NewServer() *Server {
	apperror.InstallHumaErrorFactory()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	cfg := huma.DefaultConfig("TalentPilot API", "0.1.0")
	cfg.CreateHooks = nil
	cfg.Transformers = append(cfg.Transformers, apperror.RequestIDTransformer)

	api := humaecho.New(e, cfg)

	huma.Register(api, huma.Operation{
		OperationID: "get-healthz",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Health check",
		Tags:        []string{"system"},
	}, func(ctx context.Context, input *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})

	return &Server{Echo: e, API: api}
}
