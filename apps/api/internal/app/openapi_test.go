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

func TestOpenAPIDocumentIncludesAuthEndpoints(t *testing.T) {
	server := NewServer()

	raw, err := json.Marshal(server.API.OpenAPI())
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}

	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal openapi: %v", err)
	}

	assertOperation(t, doc.Paths, "/auth/csrf", "get", "get-auth-csrf")
	assertOperation(t, doc.Paths, "/auth/w3/login", "post", "post-auth-w3-login")
	assertOperation(t, doc.Paths, "/me", "get", "get-me")
	assertOperation(t, doc.Paths, "/auth/logout", "post", "post-auth-logout")
}

func TestOpenAPIAuthResponseArraysAreNotNullable(t *testing.T) {
	server := NewServer()

	raw, err := json.Marshal(server.API.OpenAPI())
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}

	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal openapi: %v", err)
	}

	authResponse, ok := doc.Components.Schemas["AuthResponse"]
	if !ok {
		t.Fatalf("expected AuthResponse schema")
	}
	for _, property := range []string{"roleBindings", "roleLabels", "pageAccess"} {
		if schemaPropertyAllowsNull(t, authResponse.Properties[property]) {
			t.Fatalf("expected AuthResponse.%s not to allow null", property)
		}
	}
}

func TestOpenAPIDocumentUsesStableErrorEnvelope(t *testing.T) {
	server := NewServer()

	raw, err := json.Marshal(server.API.OpenAPI())
	if err != nil {
		t.Fatalf("marshal openapi: %v", err)
	}

	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Type       string                     `json:"type"`
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Content map[string]struct {
					Schema struct {
						Ref string `json:"$ref"`
					} `json:"schema"`
				} `json:"content"`
			} `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal openapi: %v", err)
	}

	if _, ok := doc.Components.Schemas["ErrorModel"]; ok {
		t.Fatalf("expected OpenAPI document not to expose Huma ErrorModel")
	}

	foundStableEnvelope := false
	for name, schema := range doc.Components.Schemas {
		props := schema.Properties
		if schemaPropertyHasType(t, props["code"], "string") &&
			schemaPropertyHasType(t, props["message"], "string") &&
			schemaPropertyHasType(t, props["requestId"], "string") &&
			schemaPropertyHasType(t, props["details"], "object") {
			foundStableEnvelope = true
			if name == "" {
				t.Fatalf("stable error envelope schema name must not be empty")
			}
		}
	}
	if !foundStableEnvelope {
		t.Fatalf("expected OpenAPI components to include stable error envelope properties")
	}

	defaultResponse := doc.Paths["/healthz"]["get"].Responses["default"]
	if _, ok := defaultResponse.Content["application/json"]; !ok {
		t.Fatalf("expected default error response to use application/json content")
	}
	if _, ok := defaultResponse.Content["application/problem+json"]; ok {
		t.Fatalf("expected default error response not to use Huma problem+json content")
	}
}

func schemaPropertyHasType(t *testing.T, raw json.RawMessage, expected string) bool {
	t.Helper()
	if len(raw) == 0 {
		return false
	}

	var property struct {
		Type any `json:"type"`
	}
	if err := json.Unmarshal(raw, &property); err != nil {
		t.Fatalf("unmarshal schema property: %v", err)
	}

	switch actual := property.Type.(type) {
	case string:
		return actual == expected
	case []any:
		for _, item := range actual {
			if value, ok := item.(string); ok && value == expected {
				return true
			}
		}
	}
	return false
}

func schemaPropertyAllowsNull(t *testing.T, raw json.RawMessage) bool {
	t.Helper()
	if len(raw) == 0 {
		t.Fatalf("expected schema property")
	}

	var property struct {
		Type any `json:"type"`
	}
	if err := json.Unmarshal(raw, &property); err != nil {
		t.Fatalf("unmarshal schema property: %v", err)
	}

	switch actual := property.Type.(type) {
	case string:
		return actual == "null"
	case []any:
		for _, item := range actual {
			if value, ok := item.(string); ok && value == "null" {
				return true
			}
		}
	}
	return false
}

func assertOperation(t *testing.T, paths map[string]map[string]struct {
	OperationID string `json:"operationId"`
}, path string, method string, operationID string) {
	t.Helper()
	pathItem, ok := paths[path]
	if !ok {
		t.Fatalf("expected OpenAPI path %s", path)
	}
	operation, ok := pathItem[method]
	if !ok {
		t.Fatalf("expected OpenAPI %s %s", method, path)
	}
	if operation.OperationID != operationID {
		t.Fatalf("expected operationId %s, got %s", operationID, operation.OperationID)
	}
}
