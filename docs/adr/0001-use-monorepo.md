# ADR 0001: Use a Monorepo

## Status

Accepted

## Context

TalentPilot needs a frontend app, backend API, generated API contract, generated frontend client, shared documentation, CI, and Agent-friendly project memory. The repository currently starts from a PRD-only baseline.

## Decision

Use a monorepo with:

- `apps/web`
- `apps/api`
- `packages/api-contract`
- `packages/api-client`
- `packages/shared`
- `docs/`
- root-level CI, Makefile, Docker Compose, and `AGENTS.md`

## Consequences

- Frontend and backend remain independently runnable while sharing one review and CI surface.
- Generated API artifacts can be checked for drift in one place.
- Agent memory and status tracking stay close to the code.
- The repository must maintain clear boundaries to avoid accidental cross-app coupling.

