package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

func TestHealthEndpointReturnsOK(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	expected := `{"status":"ok"}`
	if rec.Body.String() != expected+"\n" && rec.Body.String() != expected {
		t.Fatalf("expected body %s, got %s", expected, rec.Body.String())
	}
}

func TestHumaErrorsUseStableEnvelope(t *testing.T) {
	server := NewServer()
	huma.Register(server.API, huma.Operation{
		OperationID: "get-test-forbidden",
		Method:      http.MethodGet,
		Path:        "/test/forbidden",
		Errors:      []int{http.StatusForbidden},
	}, func(ctx context.Context, input *struct{}) (*struct{}, error) {
		return nil, huma.Error403Forbidden("没有权限")
	})

	req := httptest.NewRequest(http.MethodGet, "/test/forbidden", nil)
	req.Header.Set("X-Request-ID", "req_test")
	rec := httptest.NewRecorder()

	server.Echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d with body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected content type application/json, got %q", ct)
	}

	var body struct {
		Code      string         `json:"code"`
		Message   string         `json:"message"`
		RequestID string         `json:"requestId"`
		Details   map[string]any `json:"details"`
		Type      string         `json:"type"`
		Title     string         `json:"title"`
		Status    int            `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal stable error envelope: %v", err)
	}

	if body.Code != "IAM_PERMISSION_DENIED" {
		t.Fatalf("expected code IAM_PERMISSION_DENIED, got %q", body.Code)
	}
	if body.Message != "没有权限" {
		t.Fatalf("expected message 没有权限, got %q", body.Message)
	}
	if body.RequestID != "req_test" {
		t.Fatalf("expected request id req_test, got %q", body.RequestID)
	}
	if body.Details == nil {
		t.Fatalf("expected details object, got nil")
	}
	if body.Type != "" || body.Title != "" || body.Status != 0 {
		t.Fatalf("expected no Huma problem fields, got type=%q title=%q status=%d", body.Type, body.Title, body.Status)
	}
}
