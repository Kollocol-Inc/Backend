CREATE TABLE IF NOT EXISTS quiz_deadline_reminders_sent (
    instance_id      TEXT        NOT NULL,
    user_id          TEXT        NOT NULL,
    reminder_offset  TEXT        NOT NULL,
    sent_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (instance_id, user_id, reminder_offset)
);
