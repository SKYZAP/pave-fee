CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE bills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id TEXT NOT NULL,
    currency TEXT NOT NULL CHECK (currency IN ('GEL', 'USD')),
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED')),
    closed_at TIMESTAMPTZ,
    total JSONB,
    workflow_id TEXT NOT NULL UNIQUE,
    idempotent_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (period_end > period_start),
    CHECK ((status = 'OPEN' AND total IS NULL AND closed_at IS NULL)
        OR (status = 'CLOSED' AND total IS NOT NULL AND closed_at IS NOT NULL))
);

CREATE UNIQUE INDEX bills_owner_period_idx
    ON bills (owner_id, period_start, period_end);
CREATE UNIQUE INDEX bills_owner_idempotency_idx
    ON bills (owner_id, idempotent_key);

CREATE TABLE line_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bill_id UUID NOT NULL REFERENCES bills(id),
    transaction_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    description TEXT NOT NULL,
    currency TEXT NOT NULL CHECK (currency IN ('GEL', 'USD')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (bill_id, transaction_id)
);

CREATE INDEX line_items_bill_created_idx ON line_items (bill_id, created_at, id);

CREATE TABLE outbox_events (
    event_id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    event_version TEXT NOT NULL,
    aggregate_version BIGINT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PUBLISHED')),
    attempts INT NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (aggregate_type, aggregate_id, aggregate_version, event_type)
);

CREATE INDEX outbox_pending_idx
    ON outbox_events (status, available_at, created_at);
