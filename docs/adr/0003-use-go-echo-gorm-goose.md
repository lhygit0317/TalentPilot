# ADR 0003: Use Go, Echo, GORM, and goose

## Status

Accepted

## Context

The backend should be container friendly, resource efficient, CI friendly, and suitable for a frontend/backend separated application. The user selected Echo plus an ORM and agreed to GORM plus goose.

## Decision

Use:

- Go for the backend.
- Echo for HTTP routing and middleware.
- GORM for ORM and repository implementation.
- goose for database migrations.

SQLite is used for local automated tests. PostgreSQL is used for production.

## Consequences

- The backend remains lightweight and easy to containerize.
- GORM provides mature support for SQLite and PostgreSQL.
- goose provides auditable migrations and rollback paths.
- Production schema changes must not rely on GORM AutoMigrate.
- Complex queries must be written carefully to avoid hidden ORM behavior.

