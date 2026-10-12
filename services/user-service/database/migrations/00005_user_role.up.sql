-- A role, not an is_admin flag: an enum leaves room for e.g. 'merchant' later
-- without another boolean column per kind of user.
CREATE TYPE user_role AS ENUM ('user', 'admin');

ALTER TABLE app_user ADD COLUMN role user_role NOT NULL DEFAULT 'user';

-- The one admin. Logs in through the same SMS flow as everyone else; what the
-- account may do is decided by the gateway from this column.
INSERT INTO app_user (phone, nick_name, icon, role) VALUES
    ('0100000000', 'NearBuy Admin', '/icons/admin.png', 'admin')
ON CONFLICT (phone) DO UPDATE SET role = 'admin';
