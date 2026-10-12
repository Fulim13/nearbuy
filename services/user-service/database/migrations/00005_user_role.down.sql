DELETE FROM app_user WHERE phone = '0100000000';
ALTER TABLE app_user DROP COLUMN role;
DROP TYPE user_role;
