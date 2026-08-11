ALTER TABLE prototype_budget_transactions
ADD COLUMN classification_source TEXT NOT NULL DEFAULT 'unmatched';
