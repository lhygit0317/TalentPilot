# ADR 0002: Use React, Vite, Tailwind, and shadcn/ui

## Status

Accepted

## Context

The PRD requires a polished, high-density enterprise recruiting decision UI. The user requested React, shadcn/ui, Vercel AI SDK, Vite, Tailwind CSS, stable modern versions, and a matching motion library.

## Decision

Use:

- React with TypeScript.
- Vite for frontend build tooling.
- Tailwind CSS for styling.
- shadcn/ui as the component foundation.
- Vercel AI SDK for AI-facing frontend flows.
- lucide-react for icons.
- A React-compatible motion library for state feedback.

Business pages must use shadcn/ui or project-wrapped components for interactive elements rather than raw interactive HTML.

## Consequences

- The frontend has a stable, modern stack with strong ecosystem support.
- UI consistency can be enforced through project wrappers and Tailwind tokens.
- Additional lint or review rules are needed to prevent raw interactive element drift in business pages.

