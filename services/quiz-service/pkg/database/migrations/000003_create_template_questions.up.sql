CREATE TABLE IF NOT EXISTS template_questions (
    template_id VARCHAR(255) NOT NULL,
    question_id VARCHAR(255) NOT NULL,
    order_index INTEGER NOT NULL,
    PRIMARY KEY (template_id, question_id),
    FOREIGN KEY (template_id) REFERENCES quiz_templates(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_template_questions_template_id ON template_questions(template_id);
