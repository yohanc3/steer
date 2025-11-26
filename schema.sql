CREATE TABLE IF NOT EXISTS user (
    id INTEGER AUTO INCREMENT NOT NULL,
    user_guid TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    picture TEXT
);

CREATE INDEX IF NOT EXISTS email_idx on user (email);