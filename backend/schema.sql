CREATE TABLE IF NOT EXISTS user (
    id TEXT PRIMARY KEY UNIQUE NOT NULL ,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    picture TEXT,
    created_at INTEGER DEFAULT (unixepoch()) NOT NULL,
    conversation_id TEXT 
);

CREATE TABLE IF NOT EXISTS otp (
    id INTEGER PRIMARY KEY NOT NULL,
    user_id INTEGER UNIQUE NOT NULL,
    code TEXT NOT NULL,
    expires_at INTEGER NOT NULL, 
    created_at INTEGER DEFAULT (unixepoch()) NOT NULL    
);

CREATE INDEX IF NOT EXISTS email_idx on user (email);
CREATE INDEX IF NOT EXISTS email_idx on otp (user_id);
