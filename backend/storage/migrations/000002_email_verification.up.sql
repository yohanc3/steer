ALTER TABLE users ADD COLUMN email_verified_at INTEGER;

CREATE TABLE telegram_accounts (
    telegram_user_id INTEGER PRIMARY KEY,
    telegram_chat_id INTEGER NOT NULL UNIQUE,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(telegram_user_id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

INSERT INTO telegram_accounts (telegram_user_id, telegram_chat_id, user_id)
SELECT telegram_user_id, telegram_chat_id, telegram_user_id
FROM users;

CREATE TABLE email_verification_codes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(telegram_user_id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    code_hash BLOB NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER,
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX email_verification_codes_user_email_idx
ON email_verification_codes(user_id, email, created_at DESC);
