-- +goose Up
ALTER TABLE usage_events ADD COLUMN requested_model TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_usage_events_requested_model ON usage_events(requested_model);

-- +goose Down
DROP INDEX IF EXISTS idx_usage_events_requested_model;
ALTER TABLE usage_events DROP COLUMN requested_model;
