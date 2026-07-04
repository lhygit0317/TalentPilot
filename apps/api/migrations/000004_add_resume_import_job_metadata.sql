-- +goose Up
ALTER TABLE jobs ADD COLUMN created_by_user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN result_json TEXT NOT NULL DEFAULT '{}';
CREATE INDEX idx_jobs_created_by_user ON jobs(created_by_user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_created_by_user;
ALTER TABLE jobs DROP COLUMN result_json;
ALTER TABLE jobs DROP COLUMN created_by_user_id;
