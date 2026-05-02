ALTER TABLE notifications
    ADD COLUMN related_entity_id VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_notifications_user_type_entity
    ON notifications(user_id, type, related_entity_id)
    WHERE related_entity_id IS NOT NULL;
