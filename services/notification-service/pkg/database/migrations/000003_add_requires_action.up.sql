ALTER TABLE notifications
    ADD COLUMN requires_action BOOLEAN NOT NULL DEFAULT false;

UPDATE notifications
SET requires_action = true
WHERE type = 'group_invite'
  AND is_read = false;
