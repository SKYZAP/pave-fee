# AGENTS.md

## Project overview

This repository is an Encore Go application with a React/Vite frontend. It implements a Fees API that creates a bill workflow for a period, records progressive line items, and closes an immutable invoice.

- `fees/` owns the public API, Encore service lifecycle, PostgreSQL projection, Temporal worker, and outbox relay.
- `fees/domain/` owns currency, money, lifecycle, validation, and domain errors.
- `fees/database/` owns typed SQL and transaction/row-lock invariants.
- `fees/temporal/` owns the workflow, updates, activities, timers, and retry policy.
- `frontend/` contains the Fees UI and its Encore static-file serving endpoint.

The Go module path is `encore.app`. Read `.github/copilot-instructions.md` for the full Encore and Go reference; this file adds repository-specific operating rules.

## Repository layout

```text
.
├── encore.app                         Encore application manifest
├── go.mod
├── go.sum                             Go module and dependency checksums
├── docker-compose.temporal.yml        Local Temporal Server, its PostgreSQL store, and UI
├── README.md                          Local setup and API usage
├── AGENTS.md                          Repository-specific agent guidance
├── docs/
│   └── fees-api-tech-spec.md          Implemented API, ERD, workflow, and outbox design
├── fees/                              Encore Fees service
│   ├── api.go                         Public HTTP endpoints and API error mapping
│   ├── service.go                     Service lifecycle, Temporal worker, and database setup
│   ├── contracts.go                   External DTOs and mappings to domain types
│   ├── outbox.go                      bill.closed relay to Encore Pub/Sub
│   ├── migrations/
│   │   └── 1_create_billing_tables.up.sql
│   ├── config/
│   │   └── config.go                  Temporal runtime configuration
│   ├── database/
│   │   ├── repository.go              PostgreSQL reads, writes, locks, and outbox transaction
│   │   └── repository_test.go
│   ├── domain/
│   │   ├── models.go                  Money, currency, validation, totals, and request hashing
│   │   ├── errors.go
│   │   └── models_test.go
│   └── temporal/
│       ├── workflow.go                Workflow, update handlers, activities, timers, retries
│       └── workflow_test.go
└── frontend/
    ├── frontend.go                    Embeds dist/ and serves the frontend through Encore
    ├── package.json
    ├── vite.config.ts                 Development proxy for /v1 requests
    ├── src/
    │   ├── App.tsx                    Bills dashboard, creation, details, and invoice UI
    │   ├── api/                       HTTP client and idempotency-key helpers
    │   ├── components/ui/             Reusable UI primitives
    │   ├── lib/                       Money formatting and API error display
    │   └── types/                     API-facing TypeScript types
    └── dist/                          Generated Vite build; never edit manually
```

`fees/` is the only Encore service. Its root package must remain thin because Encore discovers service declarations and endpoint annotations there. Code in `fees/temporal/` may call activities only; it must never query PostgreSQL directly. `fees/database/` is the sole owner of SQL and transaction boundaries.

`frontend/dist/` is a generated artifact required by `frontend/frontend.go` at runtime. Rebuild it with `npm run build` after frontend source changes, but do not hand-edit it.

## Non-negotiable rules

- Write valid Go 1.22+ code and follow normal `gofmt` conventions. The module currently declares Go 1.26.
- Never edit generated Encore artifacts: `encore.gen.go`, `/encore.gen/`, or `/.encore/`. Regenerate them through the Encore CLI when needed.
- Treat `encore.app` as source text, not as a binary file.
- Keep database changes in a new, numbered `migrations/*.up.sql` file owned by the relevant service. Do not rewrite an applied migration. The current single migration is reset-only local-development history.
- Use parameterized SQL. Scan query results into typed fields rather than `interface{}` destinations.
- Keep Encore endpoint annotations, request/response types, and paths consistent with existing service code.
- Do not commit secrets. Temporal credentials and production configuration belong in Encore configuration or secret storage, not source files.
- Avoid unrelated refactors, dependency upgrades, or generated-file churn.

## Backend development

Run commands from the repository root:

```bash
encore check
encore test ./...
```

`encore check` is the preferred compile-and-boot smoke check. To exercise an endpoint after the app boots, use Encore's relative-path curl form, for example:

```bash
encore check 'curl /v1/bills?owner_id=merchant_123'
encore check 'curl /v1/bills -X POST -H "Idempotency-Key: create-001" -d "{\"owner_id\":\"merchant_123\",\"currency\":\"USD\",\"period_start\":\"2026-08-01T00:00:00Z\",\"period_end\":\"2026-09-01T00:00:00Z\"}"'
```

For local development, Docker must be running. Start Temporal dependencies with `docker compose -f docker-compose.temporal.yml up -d`, then start the app with `encore run`. The app is served on port 4000, the Encore dashboard is on port 9400, and Temporal UI is on port 8080.

The `temporal-postgres` Docker container stores Temporal workflow history only. Bills, line items, and outbox events use the Encore-managed `fees` database. When changing workflow or outbox behavior, account for at-least-once delivery and verify the relevant activity, database state, and relay outcome.

## Fees invariants

- A bill has one immutable currency (`GEL` or `USD`).
- Every line item must match its bill currency; never convert or combine currencies.
- Use signed 64-bit minor units only: tetri for GEL and cents for USD.
- A bill transitions from `OPEN` to `CLOSED` once. Reject all later line-item writes.
- Preserve idempotency keys and request hashes for create and line-item retries.
- Keep all SQL in `fees/database/`; Temporal workflow code must not access PostgreSQL directly.
- Preserve the transactional close boundary: invoice snapshot, status transition, and outbox event commit together.

## Frontend development

Run commands from `frontend/`:

```bash
npm ci
npm test -- --runInBand
npm run build
```

The UI source is under `frontend/src/`. `frontend/frontend.go` embeds `frontend/dist`, so frontend changes require a successful `npm run build` before the embedded UI can be verified through Encore. Do not hand-edit build output; regenerate it with the frontend build.

Follow the existing React, TypeScript, Tailwind, and React Query patterns. Keep user-visible loading, error, empty, and mutation states intact when changing UI behavior. The frontend selects the bill currency at creation and must not offer a mismatched line-item currency.

## Change workflow

1. Read the affected service, tests, migrations, and callers before editing.
2. Preserve Fees boundaries: API DTOs and service lifecycle stay in `fees/`; domain rules stay in `fees/domain/`; SQL stays in `fees/database/`; workflow code stays in `fees/temporal/`.
3. Update or add focused tests for new observable behavior, matching existing test conventions.
4. Run the narrowest relevant checks, then run `encore test ./...` or `encore check` for backend changes. For frontend changes, run the Jest suite and `npm run build`.
5. Review the final diff for accidental generated files, secrets, unrelated formatting, and stale tests.
