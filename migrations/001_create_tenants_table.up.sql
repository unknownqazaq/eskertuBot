-- db/migrations/001_create_tenants_table.up.sql
CREATE TABLE tenants (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT,
    name TEXT NOT NULL,
    apartment TEXT NOT NULL,
    payment_date DATE NOT NULL
);
CREATE TABLE telegram_users (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT UNIQUE NOT NULL
);
