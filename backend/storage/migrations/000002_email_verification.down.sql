DROP INDEX IF EXISTS email_verification_codes_user_email_idx;
DROP TABLE IF EXISTS email_verification_codes;
DROP TABLE IF EXISTS telegram_accounts;
ALTER TABLE users DROP COLUMN email_verified_at;
