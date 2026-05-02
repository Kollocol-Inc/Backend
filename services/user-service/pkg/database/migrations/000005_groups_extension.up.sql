ALTER TABLE groups
    ADD COLUMN description VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN avatar_url  VARCHAR(500) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS group_invitations (
    group_id   VARCHAR(255) NOT NULL,
    user_id    VARCHAR(255) NOT NULL,
    inviter_id VARCHAR(255) NOT NULL,
    invited_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id),
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_group_invitations_user_id ON group_invitations(user_id);
CREATE INDEX IF NOT EXISTS idx_group_invitations_group_id ON group_invitations(group_id);
