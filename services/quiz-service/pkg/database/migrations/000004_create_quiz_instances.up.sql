CREATE TABLE IF NOT EXISTS quiz_instances (
    id VARCHAR(255) PRIMARY KEY,
    template_id VARCHAR(255),
    title VARCHAR(255) NOT NULL,
    access_code VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    group_id VARCHAR(255),
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    start_time TIMESTAMP,
    deadline TIMESTAMP,
    quiz_type VARCHAR(50) NOT NULL DEFAULT 'sync',
    settings JSONB NOT NULL DEFAULT '{}',
    total_time INTEGER NOT NULL,
    total_questions INTEGER NOT NULL,
    FOREIGN KEY (template_id) REFERENCES quiz_templates(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_quiz_instances_template_id ON quiz_instances(template_id);
CREATE INDEX IF NOT EXISTS idx_quiz_instances_access_code ON quiz_instances(access_code);
CREATE INDEX IF NOT EXISTS idx_quiz_instances_group_id ON quiz_instances(group_id);
