ALTER TABLE transactions RENAME COLUMN status TO provider_status;

ALTER TABLE transactions
ADD COLUMN delivery_status TEXT NOT NULL DEFAULT 'settled'
CHECK (delivery_status IN ('settled', 'delivered'));

CREATE INDEX transactions_delivery_status_idx
ON transactions(delivery_status, created_at);
