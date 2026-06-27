# ADR 0004: Backend Code Generates OpenAPI

## Status

Accepted

## Context

The user prefers code as fact: backend code should generate OpenAPI, and the frontend should consume a generated client. The system still needs a clear contract for frontend/backend separation and CI drift checks.

## Decision

Backend Go route definitions, DTOs, and annotations generate OpenAPI. The generated OpenAPI artifact is stored under `packages/api-contract`. The frontend TypeScript client is generated from that OpenAPI artifact and stored under `packages/api-client`.

## Consequences

- Backend implementation remains the contract source of truth.
- The frontend avoids handwritten DTOs and manual fetch logic.
- CI must check generated OpenAPI and client drift.
- Handler annotations and DTO definitions must stay disciplined, because they define external contracts.

