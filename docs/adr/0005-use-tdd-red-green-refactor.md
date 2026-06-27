# ADR 0005: Use TDD Red-Green-Refactor

## Status

Accepted

## Context

The user explicitly requires test-driven design using the red-green testing principle. The product has high-risk areas: authorization, role inheritance, recommendation deduplication, resume data privacy, migrations, generated contracts, and UI permission states.

## Decision

Use TDD for production behavior:

1. Write a failing test first.
2. Verify it fails for the expected reason.
3. Write the smallest code to pass.
4. Verify tests pass.
5. Refactor while keeping tests green.

Generated code and pure configuration may use generation, drift, or validation checks instead of behavior-first tests, but exceptions must be explicit.

## Consequences

- Tests document expected behavior before implementation bias enters.
- Authorization and business edge cases must be captured early.
- CI must run enough tests to make red-green work reliable.
- Reviewers should reject production behavior that lacks a failing-test-first trail.

