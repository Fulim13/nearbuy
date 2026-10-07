CREATE OR REPLACE FUNCTION set_update_time() RETURNS trigger AS $$
BEGIN
    NEW.update_time = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TYPE order_status AS ENUM
    ('unpaid', 'paid', 'redeemed', 'cancelled', 'refunding', 'refunded');
CREATE TYPE pay_type AS ENUM ('balance', 'card');

CREATE TABLE voucher_order (
    -- NOT an identity column. The id comes from pkg/idgen (lesson 16) because the
    -- gateway returns it to the client in a 202 before this row exists.
    id          bigint       PRIMARY KEY,
    user_id     bigint       NOT NULL,
    voucher_id  bigint       NOT NULL,
    pay_value   bigint       NOT NULL,
    pay_type    pay_type     NOT NULL DEFAULT 'balance',
    status      order_status NOT NULL DEFAULT 'unpaid',
    create_time timestamptz  NOT NULL DEFAULT now(),
    pay_time    timestamptz,
    use_time    timestamptz,
    refund_time timestamptz,
    update_time timestamptz  NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_user_id    ON voucher_order (user_id);
CREATE INDEX idx_order_voucher_id ON voucher_order (voucher_id);
CREATE INDEX idx_order_status_create ON voucher_order (status, create_time);

-- One LIVE order per user per voucher, enforced by the database rather than by
-- Redis alone. A PARTIAL unique index - so that a cancelled order does not permanently block the user from buying again, which a
-- plain UNIQUE (user_id, voucher_id) would.
CREATE UNIQUE INDEX uniq_live_user_voucher
    ON voucher_order (user_id, voucher_id)
    WHERE status NOT IN ('cancelled', 'refunded');

CREATE TRIGGER voucher_order_set_update_time BEFORE UPDATE ON voucher_order
    FOR EACH ROW EXECUTE FUNCTION set_update_time();
