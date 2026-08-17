CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    openid VARCHAR(100) UNIQUE NOT NULL,
    nickname VARCHAR(100) NOT NULL,
    avatar_url VARCHAR(500) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    mood_input TEXT NOT NULL,
    literature_text TEXT NOT NULL,
    book_name VARCHAR(200) NOT NULL,
    author VARCHAR(100) NOT NULL,
    style VARCHAR(20) NOT NULL,
    image_url VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_history_user_id ON history(user_id);
CREATE INDEX IF NOT EXISTS idx_history_created_at ON history(created_at);
