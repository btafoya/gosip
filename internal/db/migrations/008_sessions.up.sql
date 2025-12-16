-- Sessions table for persistent authentication sessions
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    last_activity DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_agent TEXT,
    ip_address TEXT
);

-- Index for fast token lookups
CREATE INDEX idx_sessions_token ON sessions(token);

-- Index for finding user sessions
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

-- Index for expiration cleanup
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
