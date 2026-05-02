DROP TABLE IF EXISTS group_invitations;

ALTER TABLE groups DROP COLUMN IF EXISTS avatar_url;
ALTER TABLE groups DROP COLUMN IF EXISTS description;
