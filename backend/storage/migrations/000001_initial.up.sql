CREATE TABLE users (
    telegram_user_id INTEGER PRIMARY KEY,
    telegram_chat_id INTEGER NOT NULL UNIQUE,
    name TEXT,
    email TEXT,
    timezone TEXT,
    onboarding_state TEXT NOT NULL DEFAULT 'name',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE budget_limits (
    user_id INTEGER NOT NULL REFERENCES users(telegram_user_id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    currency TEXT NOT NULL DEFAULT 'USD' CHECK (currency = 'USD'),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (user_id, category)
);

CREATE TABLE connect_tokens (
    token_hash BLOB PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(telegram_user_id) ON DELETE CASCADE,
    nonce TEXT NOT NULL UNIQUE,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX connect_tokens_user_idx ON connect_tokens(user_id);

CREATE TABLE enrollments (
    enrollment_id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(telegram_user_id) ON DELETE CASCADE,
    teller_user_id TEXT NOT NULL,
    access_token_ciphertext BLOB NOT NULL,
    access_token_nonce BLOB NOT NULL,
    environment TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'baselining',
    disconnect_reason TEXT,
    baseline_completed_at INTEGER,
    last_synced_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE accounts (
    account_id TEXT PRIMARY KEY,
    enrollment_id TEXT NOT NULL REFERENCES enrollments(enrollment_id) ON DELETE CASCADE,
    institution_id TEXT NOT NULL,
    institution_name TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    subtype TEXT NOT NULL,
    currency TEXT NOT NULL,
    last_four TEXT NOT NULL,
    status TEXT NOT NULL,
    supports_transactions INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX accounts_enrollment_idx ON accounts(enrollment_id);

CREATE TABLE transactions (
    transaction_id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
    amount_cents INTEGER NOT NULL,
    transaction_date TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT,
    counterparty_name TEXT,
    counterparty_type TEXT,
    status TEXT NOT NULL,
    processing_status TEXT NOT NULL,
    type TEXT NOT NULL,
    fingerprint BLOB NOT NULL,
    supersedes_transaction_id TEXT REFERENCES transactions(transaction_id),
    baseline INTEGER NOT NULL DEFAULT 0,
    first_seen_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX transactions_account_date_idx ON transactions(account_id, transaction_date);

CREATE TABLE transaction_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    transaction_id TEXT NOT NULL REFERENCES transactions(transaction_id) ON DELETE CASCADE,
    fingerprint BLOB NOT NULL,
    payload_json TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE (transaction_id, fingerprint)
);

CREATE TABLE webhook_events (
    webhook_id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    enrollment_id TEXT,
    teller_timestamp TEXT NOT NULL,
    status TEXT NOT NULL,
    received_at INTEGER NOT NULL,
    processed_at INTEGER
);

CREATE TABLE jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL,
    deduplication_key TEXT NOT NULL UNIQUE,
    payload_json TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    run_after INTEGER NOT NULL DEFAULT (unixepoch()),
    last_error TEXT,
    telegram_message_id INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX jobs_ready_idx ON jobs(status, run_after);
