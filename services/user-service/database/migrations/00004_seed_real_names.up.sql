-- Readable names for the seeded test users, so screens and logs show a person
-- rather than user_abc123. A new migration instead of an edit to 00002_seed:
-- that one is already applied everywhere, and editing it would only take
-- effect after a migrate-reset.
UPDATE app_user SET nick_name = 'Ahmad Faizal' WHERE id = 1 AND phone = '0123456789';
UPDATE app_user SET nick_name = 'Tan Mei Ling' WHERE id = 2 AND phone = '0198765432';
UPDATE app_user SET nick_name = 'Ravi Kumar'   WHERE id = 3 AND phone = '01123456780';
