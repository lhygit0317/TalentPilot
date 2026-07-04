# TalentPilot

TalentPilot is a frontend/backend separated recruiting intelligence assistant for the Computing Power Business Unit.

## Current Phase

E1 auth and IAM runtime implementation are complete. E4 Resume Library implementation is the next active business slice. See:

- `PRD.md`
- `AGENTS.md`
- `docs/specs/000-foundation-architecture.md`
- `docs/specs/001-auth-session-w3.md`
- `docs/specs/002-iam-permission-model.md`
- `docs/superpowers/plans/2026-07-03-iam-permission-model-implementation.md`
- `docs/specs/003-resume-library-import.md`
- `docs/superpowers/plans/2026-07-04-e4-resume-library-implementation.md`
- `docs/project-status.md`

## Commands

```bash
make setup
make dev
make dev-web
make dev-api
make test
make test-api
make test-web
make test-e2e
make lint
make typecheck
make build
make migrate-up
make migrate-down
make openapi-generate
make openapi-check
make client-generate
make ci
```

## Architecture

- Frontend: `apps/web`
- Backend: `apps/api`
- OpenAPI contract: `packages/api-contract`
- Generated frontend client: `packages/api-client`

The backend is the authoritative boundary for authentication, authorization, data filtering, transactions, audit logging, file safety checks, and business rules. The frontend owns UI, interaction, form pre-validation, local state, i18n, accessibility, and generated client consumption.
