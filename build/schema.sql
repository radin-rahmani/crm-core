-- This file documents the schema exactly as created at runtime by
-- initDB() in cmd/server/main.go. It is not executed automatically by
-- the application (main.go runs these CREATE TABLE IF NOT EXISTS
-- statements itself on startup) — keep the two in sync if you change one.
--
-- Note: customer/log IDs are generated in Go (uuid.New().String() /
-- SERIAL) rather than via Postgres defaults, so no DEFAULT expressions
-- or extensions are required here.

CREATE TABLE IF NOT EXISTS customers (
    id         VARCHAR(36) PRIMARY KEY,
    name       VARCHAR(100),
    email      VARCHAR(100) UNIQUE,
    created_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS logs (
    id         SERIAL PRIMARY KEY,
    user_id    VARCHAR(36),
    action     VARCHAR(50),
    details    TEXT,
    created_at TIMESTAMP
);
