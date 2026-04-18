CREATE TABLE IF NOT EXISTS quiz_templates (
    id VARCHAR(255) PRIMARY KEY,
    owner_id VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    quiz_type VARCHAR(50) NOT NULL,
    settings JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_quiz_templates_owner_id ON quiz_templates(owner_id);
