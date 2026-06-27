package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
