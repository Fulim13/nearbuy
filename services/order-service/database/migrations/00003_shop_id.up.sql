-- shop_id rides along from seckill.admitted so order.state_changed can carry it.
-- shop-service cannot look it up itself: voucher lives in voucher_db.
ALTER TABLE voucher_order ADD COLUMN shop_id bigint NOT NULL DEFAULT 0;
