ALTER TABLE user_notification_settings
    ADD COLUMN IF NOT EXISTS group_kicked BOOLEAN NOT NULL DEFAULT true;
