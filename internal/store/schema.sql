CREATE TABLE IF NOT EXISTS kv_store (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Note: 'memories', 'sessions', and 'events' tables are removed in Lite Core 
-- to favor ADK's in-memory services and reduce complexity/context saturation.
