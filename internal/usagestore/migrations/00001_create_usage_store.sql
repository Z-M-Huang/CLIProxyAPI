-- +goose Up
CREATE TABLE IF NOT EXISTS usage_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	event_key TEXT NOT NULL UNIQUE,
	api_group_key TEXT,
	provider TEXT,
	endpoint TEXT,
	auth_type TEXT,
	request_id TEXT,
	model TEXT NOT NULL,
	timestamp TEXT NOT NULL,
	source TEXT,
	auth_index TEXT,
	failed INTEGER NOT NULL DEFAULT 0,
	latency_ms INTEGER NOT NULL DEFAULT 0,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	reasoning_tokens INTEGER NOT NULL DEFAULT 0,
	cached_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_events_timestamp ON usage_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_usage_events_model ON usage_events(model);
CREATE INDEX IF NOT EXISTS idx_usage_events_source ON usage_events(source);
CREATE INDEX IF NOT EXISTS idx_usage_events_auth_index ON usage_events(auth_index);
CREATE INDEX IF NOT EXISTS idx_usage_events_failed ON usage_events(failed);
CREATE INDEX IF NOT EXISTS idx_usage_events_request_id ON usage_events(request_id);

CREATE TABLE IF NOT EXISTS request_histories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	request_id TEXT NOT NULL UNIQUE,
	log_name TEXT NOT NULL UNIQUE,
	url TEXT,
	method TEXT,
	status_code INTEGER NOT NULL DEFAULT 0,
	force INTEGER NOT NULL DEFAULT 0,
	request_timestamp TEXT,
	api_response_timestamp TEXT,
	log_text_gzip BLOB NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_request_histories_force ON request_histories(force);
CREATE INDEX IF NOT EXISTS idx_request_histories_created_at ON request_histories(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_request_histories_created_at;
DROP INDEX IF EXISTS idx_request_histories_force;
DROP TABLE IF EXISTS request_histories;

DROP INDEX IF EXISTS idx_usage_events_request_id;
DROP INDEX IF EXISTS idx_usage_events_failed;
DROP INDEX IF EXISTS idx_usage_events_auth_index;
DROP INDEX IF EXISTS idx_usage_events_source;
DROP INDEX IF EXISTS idx_usage_events_model;
DROP INDEX IF EXISTS idx_usage_events_timestamp;
DROP TABLE IF EXISTS usage_events;
