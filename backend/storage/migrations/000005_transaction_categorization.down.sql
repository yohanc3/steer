DROP INDEX IF EXISTS transactions_categorization_queue_idx;
DROP INDEX IF EXISTS transaction_categories_user_active_idx;
DROP TABLE merchant_category_rules;

ALTER TABLE transactions DROP COLUMN categorized_at;
ALTER TABLE transactions DROP COLUMN categorization_probabilities;
ALTER TABLE transactions DROP COLUMN categorization_model;
ALTER TABLE transactions DROP COLUMN categorization_confidence;
ALTER TABLE transactions DROP COLUMN categorization_source;
ALTER TABLE transactions DROP COLUMN categorization_status;
ALTER TABLE transactions DROP COLUMN user_category_id;

DROP TABLE transaction_categories;
