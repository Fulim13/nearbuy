-- Set once voucher-service has given back the stock of a cancelled order. The
-- expiry job cancels in one step and restores in the next; an order that is
-- cancelled but not yet restored is retried on every tick until this is set.
ALTER TABLE voucher_order ADD COLUMN stock_restored_at timestamptz;

-- Orders cancelled before this column existed were restored by the old Saga S3.
UPDATE voucher_order SET stock_restored_at = update_time WHERE status = 'cancelled';

-- The retry sweep only ever looks at this small set.
CREATE INDEX idx_order_restore_pending ON voucher_order (create_time)
    WHERE status = 'cancelled' AND stock_restored_at IS NULL;
