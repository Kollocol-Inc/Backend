CREATE TABLE IF NOT EXISTS questions (
    id VARCHAR(255) PRIMARY KEY,
    text TEXT NOT NULL,
    type VARCHAR(50) NOT NULL,
    correct_answer JSONB NOT NULL DEFAULT '{}',
    max_score INTEGER NOT NULL DEFAULT 0,
    time_limit_sec INTEGER NOT NULL DEFAULT 0
);
