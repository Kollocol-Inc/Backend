CREATE TABLE IF NOT EXISTS instance_questions (
    instance_id VARCHAR(255) NOT NULL,
    question_id VARCHAR(255) NOT NULL,
    order_index INTEGER NOT NULL,
    PRIMARY KEY (instance_id, question_id),
    FOREIGN KEY (instance_id) REFERENCES quiz_instances(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_instance_questions_instance_id ON instance_questions(instance_id);
