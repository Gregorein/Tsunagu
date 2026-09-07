UPDATE media
SET added_at = CURRENT_TIMESTAMP
WHERE added_at IS NULL
  AND id IN (SELECT media_id FROM media_folders);
