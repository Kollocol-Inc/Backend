-- Drop any existing duplicates (keep the earliest by ctid), then add the unique index.
DELETE FROM game_sessions a
USING game_sessions b
WHERE a.ctid > b.ctid
  AND a.instance_id = b.instance_id
  AND a.user_id = b.user_id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_game_sessions_instance_user
  ON game_sessions (instance_id, user_id);
