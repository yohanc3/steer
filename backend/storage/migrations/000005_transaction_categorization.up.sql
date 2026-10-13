CREATE TABLE transaction_categories (
    category_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL COLLATE NOCASE,
    description TEXT NOT NULL DEFAULT '',
    active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    UNIQUE (user_id, name)
);

CREATE TABLE merchant_category_rules (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    normalized_transaction_name TEXT NOT NULL,
    category_id TEXT NOT NULL REFERENCES transaction_categories(category_id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (user_id, normalized_transaction_name)
);

ALTER TABLE transactions
ADD COLUMN user_category_id TEXT REFERENCES transaction_categories(category_id) ON DELETE SET NULL;

ALTER TABLE transactions
ADD COLUMN categorization_status TEXT NOT NULL DEFAULT 'pending'
CHECK (categorization_status IN ('pending', 'categorized', 'uncategorized'));

ALTER TABLE transactions
ADD COLUMN categorization_source TEXT
CHECK (categorization_source IS NULL OR categorization_source IN (
    'jev',
    'jev_uncertain',
    'jev_error',
    'merchant_rule',
    'user',
    'no_categories'
));

ALTER TABLE transactions
ADD COLUMN categorization_confidence REAL
CHECK (categorization_confidence IS NULL OR (
    categorization_confidence >= 0 AND categorization_confidence <= 1
));

ALTER TABLE transactions ADD COLUMN categorization_model TEXT;
ALTER TABLE transactions ADD COLUMN categorization_probabilities TEXT;
ALTER TABLE transactions ADD COLUMN categorized_at INTEGER;

CREATE INDEX transaction_categories_user_active_idx
ON transaction_categories(user_id, active, name);

CREATE INDEX transactions_categorization_queue_idx
ON transactions(categorization_status, provider_status, created_at, transaction_id);
