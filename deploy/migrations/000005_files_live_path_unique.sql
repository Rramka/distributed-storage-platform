-- Soft-deleted files must not block a new upload at the same path.
-- docs/03-data-model.md (files unique on live rows only).

ALTER TABLE files DROP CONSTRAINT IF EXISTS files_bucket_id_path_key;

CREATE UNIQUE INDEX IF NOT EXISTS files_bucket_path_live
    ON files (bucket_id, path)
    WHERE deleted_at IS NULL;
