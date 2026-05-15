-- +goose Up
ALTER TABLE usage_events ADD COLUMN cache_read_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_events ADD COLUMN cache_creation_tokens INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE usage_events DROP COLUMN cache_creation_tokens;
ALTER TABLE usage_events DROP COLUMN cache_read_tokens;
