-- +goose Up
CREATE TABLE IF NOT EXISTS usage_event_keys (
    event_key TEXT PRIMARY KEY,
    created_at TEXT NOT NULL
) WITHOUT ROWID;

INSERT OR IGNORE INTO usage_event_keys (event_key, created_at)
SELECT event_key, created_at FROM usage_events;

CREATE TABLE IF NOT EXISTS usage_rollups (
    bucket_start TEXT NOT NULL,
    api_group_key TEXT NOT NULL,
    provider TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    auth_type TEXT NOT NULL,
    model TEXT NOT NULL,
    source TEXT NOT NULL,
    auth_index TEXT NOT NULL,
    failed INTEGER NOT NULL,
    request_count INTEGER NOT NULL,
    latency_total_ms INTEGER NOT NULL,
    latency_sample_count INTEGER NOT NULL,
    input_tokens INTEGER NOT NULL,
    output_tokens INTEGER NOT NULL,
    reasoning_tokens INTEGER NOT NULL,
    cached_tokens INTEGER NOT NULL,
    cache_read_tokens INTEGER NOT NULL,
    cache_creation_tokens INTEGER NOT NULL,
    total_tokens INTEGER NOT NULL,
    PRIMARY KEY (bucket_start, api_group_key, provider, endpoint, auth_type, model, source, auth_index, failed)
) WITHOUT ROWID;

INSERT INTO usage_rollups (
    bucket_start, api_group_key, provider, endpoint, auth_type, model, source, auth_index, failed,
    request_count, latency_total_ms, latency_sample_count, input_tokens, output_tokens,
    reasoning_tokens, cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens
)
SELECT
    strftime('%Y-%m-%dT%H:%M:00Z', datetime((CAST(strftime('%s', timestamp) AS INTEGER) / 900) * 900, 'unixepoch')),
    COALESCE(api_group_key, ''), COALESCE(provider, ''), COALESCE(endpoint, ''), COALESCE(auth_type, ''),
    COALESCE(model, ''), COALESCE(source, ''), COALESCE(auth_index, ''), failed,
    COUNT(*), SUM(latency_ms), SUM(CASE WHEN latency_ms > 0 THEN 1 ELSE 0 END),
    SUM(input_tokens), SUM(output_tokens), SUM(reasoning_tokens), SUM(cached_tokens),
    SUM(cache_read_tokens), SUM(cache_creation_tokens), SUM(total_tokens)
FROM usage_events
GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9;

CREATE INDEX IF NOT EXISTS idx_usage_rollups_model ON usage_rollups(model);
CREATE INDEX IF NOT EXISTS idx_usage_rollups_source ON usage_rollups(source);
CREATE INDEX IF NOT EXISTS idx_usage_rollups_auth_index ON usage_rollups(auth_index);

-- +goose Down
DROP INDEX IF EXISTS idx_usage_rollups_auth_index;
DROP INDEX IF EXISTS idx_usage_rollups_source;
DROP INDEX IF EXISTS idx_usage_rollups_model;
DROP TABLE IF EXISTS usage_rollups;
DROP TABLE IF EXISTS usage_event_keys;
