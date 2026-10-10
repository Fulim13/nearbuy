-- Say your code does `shop.sold = shop.sold + 1` for every "order paid" message.
--
--   * Message arrives            -> sold goes from 10 to 11 ✅
--   * Same message arrives again -> sold goes to 12 ❌
--
-- No error appears. The number is just quietly wrong.
--
-- The fix: a "done list"
-- This table is a list of events you've already handled. Think of a stamp card:
-- before you act, check whether it's already stamped.
--
--   event_key                    | topic               | applied_at
--   "order 1001 reached paid"    | order.state_changed | 2026-10-10 20:30
--
-- Each time a message arrives:
--   1. Try to add its key to `applied_event`.
--   2. Already there? It's a duplicate, so skip it.
--   3. New? Do the real work (`sold + 1`).
CREATE TABLE applied_event (                       -- make the table
    event_key  text        PRIMARY KEY,            -- event's unique name, e.g. "order 1001 reached paid"; no repeats allowed
    topic      text        NOT NULL,               -- which Kafka topic it came from (for debugging)
    applied_at timestamptz NOT NULL DEFAULT now()  -- when we handled it, filled in automatically
);                                                 -- end of table

CREATE INDEX idx_applied_event_applied_at ON applied_event (applied_at);  -- fast lookup by time, so cleanup can delete old rows quickly
