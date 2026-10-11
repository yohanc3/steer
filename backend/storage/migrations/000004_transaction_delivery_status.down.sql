DROP INDEX IF EXISTS transactions_delivery_status_idx;

ALTER TABLE transactions DROP COLUMN delivery_status;
ALTER TABLE transactions RENAME COLUMN provider_status TO status;
