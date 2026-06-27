package app

import (
	"encoding/json"
	"testing"
)

func TestOpenAPIDocumentIncludesHealthEndpoint(t *testing.T) {
	server := NewServer()

	raw, err := json.Marshal(server.API.OpenAPI())
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}

	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string                     `json:"operationId"`
			Responses   map[string]json.RawMessage `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal openapi: %v", err)
	}

	healthPath, ok := doc.Paths["/healthz"]
	if !ok {
		t.Fatalf("expected OpenAPI document to contain /healthz path")
	}

	getOperation, ok := healthPath["get"]
	if !ok {
		t.Fatalf("expected /healthz path to contain get operation")
	}
	if getOperation.OperationID != "get-healthz" {
		t.Fatalf("expected /healthz get operationId get-healthz, got %q", getOperation.OperationID)
	}
	if _, ok := getOperation.Responses["200"]; !ok {
		t.Fatalf("expected /healthz get operation to contain 200 response")
	}
}
