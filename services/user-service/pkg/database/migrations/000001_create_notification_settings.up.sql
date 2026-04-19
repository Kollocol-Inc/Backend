CREATE TABLE IF NOT EXISTS user_notification_settings (
    user_id VARCHAR(255) PRIMARY KEY,
    new_quizzes BOOLEAN NOT NULL DEFAULT true,
    quiz_results BOOLEAN NOT NULL DEFAULT true,
    group_invites BOOLEAN NOT NULL DEFAULT true,
    deadline_reminder VARCHAR(50) NOT NULL DEFAULT '24h',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notification_settings_user_id ON user_notification_settings(user_id);
