CREATE TABLE users (
    id TEXT PRIMARY KEY,
    telegram_conversation_id INTEGER NOT NULL UNIQUE,
    name TEXT,
    email TEXT,
    email_verified_at INTEGER,
    teller_account_id TEXT UNIQUE,
    teller_enrollment_id TEXT,
    teller_user_id TEXT,
    teller_access_token_ciphertext BLOB,
    teller_access_token_nonce BLOB,
    teller_environment TEXT,
    baseline_completed_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE teller_connect_sessions (
    token_hash BLOB PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nonce TEXT NOT NULL UNIQUE,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE email_verification_codes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    code_hash BLOB NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER,
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX email_verification_codes_user_email_idx
ON email_verification_codes(user_id, email, created_at DESC);

CREATE TABLE transactions (
    transaction_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    amount TEXT NOT NULL,
    transaction_date TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL,
    transaction_type TEXT NOT NULL,
    running_balance TEXT,
    processing_status TEXT NOT NULL,
    category TEXT,
    counterparty_name TEXT,
    counterparty_type TEXT,
    self_link TEXT NOT NULL,
    account_link TEXT NOT NULL,
    baseline INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX transactions_user_processing_date_idx
ON transactions(user_id, processing_status, transaction_date);
