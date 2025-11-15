-- PostgreSQL initialization script for RhinoBox
-- This script runs automatically when the container starts for the first time

-- Ensure UTF8 encoding
SET client_encoding = 'UTF8';

-- Create extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm"; -- For text search optimization

-- Create a metadata table to track ingested batches
CREATE TABLE IF NOT EXISTS ingest_batches (
    id SERIAL PRIMARY KEY,
    namespace VARCHAR(255) NOT NULL,
    engine VARCHAR(10) NOT NULL CHECK (engine IN ('sql', 'nosql')),
    table_name VARCHAR(255),
    documents_count INTEGER NOT NULL,
    batch_path TEXT NOT NULL,
    schema_hash VARCHAR(64),
    ingested_at TIMESTAMP NOT NULL DEFAULT NOW(),
    metadata JSONB
);

CREATE INDEX idx_ingest_batches_namespace ON ingest_batches(namespace);
CREATE INDEX idx_ingest_batches_engine ON ingest_batches(engine);
CREATE INDEX idx_ingest_batches_ingested_at ON ingest_batches(ingested_at DESC);

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE rhinobox TO rhinobox;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO rhinobox;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO rhinobox;

-- Log initialization
DO $$
BEGIN
    RAISE NOTICE 'RhinoBox PostgreSQL database initialized successfully';
END $$;
