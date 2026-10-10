CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL,
    name          TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    home_lat      DOUBLE PRECISION,
    home_lng      DOUBLE PRECISION,
    home_city     TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vehicles (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    year      INT NOT NULL,
    make      TEXT NOT NULL,
    model     TEXT NOT NULL,
    mpg       DOUBLE PRECISION NOT NULL,
    fuel_type TEXT NOT NULL DEFAULT 'regular'
);

CREATE TABLE IF NOT EXISTS trips (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    origin      TEXT NOT NULL,
    destination TEXT NOT NULL,
    start_date  DATE NOT NULL,
    end_date    DATE NOT NULL,
    travelers   INT NOT NULL DEFAULT 1,
    budget_usd  NUMERIC(10,2),
    mode        TEXT NOT NULL DEFAULT 'auto',   -- auto | drive | fly
    status      TEXT NOT NULL DEFAULT 'draft',  -- draft | planned | booked
    plan        JSONB,                          -- full itinerary + pricing from the planner
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS trips_user_idx ON trips(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS payments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_id       UUID REFERENCES trips(id) ON DELETE SET NULL,
    provider      TEXT NOT NULL DEFAULT 'jackhenry',
    provider_ref  TEXT,
    amount_usd    NUMERIC(10,2) NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending', -- pending | settled | failed
    memo          TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
