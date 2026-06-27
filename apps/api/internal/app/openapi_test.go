package app

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestOpenAPIDocumentIncludesHealthEndpoint(t *testing.T) {
	server := NewServer()

	raw, err := json.Marshal(server.API.OpenAPI())
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}

	if !bytes.Contains(raw, []byte(`"/healthz"`)) {
		t.Fatalf("expected OpenAPI document to contain /healthz, got %s", string(raw))
	}
	if !bytes.Contains(raw, []byte(`"get-healthz"`)) {
		t.Fatalf("expected OpenAPI document to contain get-healthz operation, got %s", string(raw))
	}
}
