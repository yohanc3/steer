PRAGMA foreign_keys = OFF;

CREATE TABLE users_new (
    id TEXT PRIMARY KEY,
    telegram_conversation_id INTEGER NOT NULL UNIQUE,
    name TEXT,
    email TEXT,
    email_verified_at INTEGER,
    teller_account_id TEXT UNIQUE,
    teller_user_id TEXT,
    teller_access_token_ciphertext BLOB,
    teller_access_token_nonce BLOB,
    teller_environment TEXT,
    baseline_completed_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

INSERT INTO users_new (
    id,
    telegram_conversation_id,
    name,
    email,
    email_verified_at,
    teller_account_id,
    teller_user_id,
    teller_access_token_ciphertext,
    teller_access_token_nonce,
    teller_environment,
    baseline_completed_at,
    created_at,
    updated_at
)
SELECT
    CAST(u.telegram_user_id AS TEXT),
    u.telegram_chat_id,
    u.name,
    u.email,
    u.email_verified_at,
    (
        SELECT a.account_id
        FROM accounts AS a
        WHERE a.enrollment_id = e.enrollment_id
        LIMIT 1
    ),
    e.teller_user_id,
    e.access_token_ciphertext,
    e.access_token_nonce,
    e.environment,
    e.baseline_completed_at,
    u.created_at,
    u.updated_at
FROM users AS u
LEFT JOIN enrollments AS e ON e.user_id = u.telegram_user_id;

CREATE TABLE teller_connect_sessions_new (
    token_hash BLOB PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users_new(id) ON DELETE CASCADE,
    nonce TEXT NOT NULL UNIQUE,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

INSERT INTO teller_connect_sessions_new (token_hash, user_id, nonce, expires_at, consumed_at, created_at)
SELECT token_hash, CAST(user_id AS TEXT), nonce, expires_at, consumed_at, created_at
FROM connect_tokens;

CREATE TABLE email_verification_codes_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL REFERENCES users_new(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    code_hash BLOB NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER,
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

INSERT INTO email_verification_codes_new (id, user_id, email, code_hash, expires_at, consumed_at, attempts, created_at)
SELECT id, CAST(user_id AS TEXT), email, code_hash, expires_at, consumed_at, attempts, created_at
FROM email_verification_codes;

CREATE TABLE transactions_new (
    transaction_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users_new(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    amount TEXT NOT NULL,
    transaction_date TEXT NOT NULL,
    description TEXT NOT NULL,
    -- `status` is Teller's transaction lifecycle status.
    status TEXT NOT NULL,
    transaction_type TEXT NOT NULL,
    running_balance TEXT,
    -- `processing_status` describes whether Teller has finalized processing.
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

INSERT INTO transactions_new (
    transaction_id,
    user_id,
    account_id,
    amount,
    transaction_date,
    description,
    status,
    transaction_type,
    processing_status,
    category,
    counterparty_name,
    counterparty_type,
    self_link,
    account_link,
    baseline,
    created_at,
    updated_at
)
SELECT
    t.transaction_id,
    u.id,
    t.account_id,
    printf('%.2f', t.amount_cents / 100.0),
    t.transaction_date,
    t.description,
    t.status,
    t.type,
    t.processing_status,
    t.category,
    t.counterparty_name,
    t.counterparty_type,
    '',
    '',
    t.baseline,
    t.first_seen_at,
    t.updated_at
FROM transactions AS t
JOIN users_new AS u ON u.teller_account_id = t.account_id;

DROP INDEX IF EXISTS email_verification_codes_user_email_idx;
DROP INDEX IF EXISTS transactions_account_date_idx;
DROP INDEX IF EXISTS accounts_enrollment_idx;
DROP INDEX IF EXISTS connect_tokens_user_idx;
DROP INDEX IF EXISTS jobs_ready_idx;
DROP TABLE transaction_versions;
DROP TABLE webhook_events;
DROP TABLE jobs;
DROP TABLE transactions;
DROP TABLE accounts;
DROP TABLE enrollments;
DROP TABLE connect_tokens;
DROP TABLE email_verification_codes;
DROP TABLE telegram_accounts;
DROP TABLE budget_limits;
DROP TABLE users;

ALTER TABLE users_new RENAME TO users;
ALTER TABLE teller_connect_sessions_new RENAME TO teller_connect_sessions;
ALTER TABLE email_verification_codes_new RENAME TO email_verification_codes;
ALTER TABLE transactions_new RENAME TO transactions;

CREATE INDEX email_verification_codes_user_email_idx
ON email_verification_codes(user_id, email, created_at DESC);
CREATE INDEX transactions_user_processing_date_idx
ON transactions(user_id, account_id, processing_status, transaction_date);

PRAGMA foreign_keys = ON;
