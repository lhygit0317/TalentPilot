# ADR 0006: Use Huma for OpenAPI Generation

## Status

Accepted

## Context

The backend uses Echo and must remain the source of truth for API contracts. The project needs OpenAPI generated from Go route and DTO definitions so the frontend can consume a generated TypeScript client and CI can detect contract drift.

## Decision

Use Huma v2 with the Echo adapter for backend OpenAPI generation. Define API operations through Huma registrations on the Echo server, emit the OpenAPI document from the API runtime, and store the generated artifact in `packages/api-contract`.

## Consequences

- Echo remains the HTTP framework while Huma owns route metadata, request/response schemas, and OpenAPI output.
- Backend handlers must follow Huma input/output conventions for documented endpoints.
- OpenAPI drift checks can regenerate the contract directly from backend code.
- Frontend client generation stays downstream of the committed OpenAPI artifact.
- Huma/Echo integration becomes part of the backend foundation and should be revisited only through a future ADR.
