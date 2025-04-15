-- db/migrations/001_create_tenants_table.up.sql
CREATE TABLE tenants (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    apartment TEXT NOT NULL,
    payment_date DATE NOT NULL
);