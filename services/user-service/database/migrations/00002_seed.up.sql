INSERT INTO app_user (id, phone, nick_name, icon, balance) VALUES
    (1, '0123456789', 'user_abc123', '/icons/1.png', 100000),
    (2, '0198765432', 'user_def456', '/icons/2.png', 100000),
    (3, '01123456780', 'user_ghi789', '/icons/3.png',      0);

SELECT setval(pg_get_serial_sequence('app_user', 'id'), (SELECT max(id) FROM app_user));
