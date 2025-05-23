-- db/migrations/001_create_tenants_table.up.sql

CREATE TABLE IF NOT EXISTS tenants (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    apartment TEXT NOT NULL,
    payment_date DATE NOT NULL
);

CREATE TABLE IF NOT EXISTS telegram_users (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT UNIQUE NOT NULL
);
