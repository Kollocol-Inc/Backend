DROP INDEX IF EXISTS idx_notifications_user_type_entity;
ALTER TABLE notifications DROP COLUMN IF EXISTS related_entity_id;
