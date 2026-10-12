-- One row per expired order whose stock has been given back. RestoreStock
-- inserts here in the same transaction as stock + 1, so a retried call (the
-- expiry job retries until it gets an answer) finds the row and changes nothing.
CREATE TABLE stock_restore (
    order_id    bigint      PRIMARY KEY,
    voucher_id  bigint      NOT NULL,
    create_time timestamptz NOT NULL DEFAULT now()
);
