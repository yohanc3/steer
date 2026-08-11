CREATE TABLE prototype_budgets (
    client_id TEXT PRIMARY KEY,
    monthly_total_cents INTEGER NOT NULL,
    categories_json TEXT NOT NULL,
    version INTEGER NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE prototype_budget_mappings (
    client_id TEXT NOT NULL,
    budget_version INTEGER NOT NULL,
    source_signature TEXT NOT NULL,
    budget_category TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (client_id, budget_version, source_signature)
);

CREATE TABLE prototype_budget_transactions (
    id TEXT PRIMARY KEY,
    client_id TEXT NOT NULL,
    merchant TEXT NOT NULL,
    provider_category TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    occurred_at INTEGER NOT NULL,
    budget_version INTEGER NOT NULL,
    budget_category TEXT,
    classification_status TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX prototype_budget_transactions_client_created_idx
ON prototype_budget_transactions(client_id, created_at DESC);
