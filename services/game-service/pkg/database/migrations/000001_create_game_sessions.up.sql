CREATE TABLE IF NOT EXISTS game_sessions (
    instance_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'joined',
    current_question_index INTEGER NOT NULL DEFAULT 0,
    score INTEGER NOT NULL DEFAULT 0,
    answers JSONB NOT NULL DEFAULT '[]',
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP,
    PRIMARY KEY (instance_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_game_sessions_instance_id ON game_sessions(instance_id);
CREATE INDEX IF NOT EXISTS idx_game_sessions_user_id ON game_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_game_sessions_status ON game_sessions(status);
