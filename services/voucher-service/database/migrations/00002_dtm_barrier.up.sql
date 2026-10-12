-- DTM's sub-transaction barrier. Two things here are NOT free choices.
--
-- 1. The default table name is the literal string "dtm_barrier.barrier", which
--    Postgres reads as SCHEMA.TABLE. So it is a SCHEMA named dtm_barrier inside
--    the service's own database -- not a database of that name.
-- 2. The unique constraint must be named exactly uniq_barrier, because DTM
--    generates
--      insert ... on conflict ON CONSTRAINT uniq_barrier do nothing
--    with that name hard-coded. Rename it and every branch fails with
--      constraint "uniq_barrier" for table "barrier" does not exist
CREATE SCHEMA IF NOT EXISTS dtm_barrier;

CREATE TABLE dtm_barrier.barrier (
    id          bigserial    PRIMARY KEY,
    trans_type  varchar(45)  NOT NULL,
    gid         varchar(128) NOT NULL,
    branch_id   varchar(128) NOT NULL,
    op          varchar(45)  NOT NULL,
    barrier_id  varchar(45)  NOT NULL,
    reason      varchar(45)  NOT NULL,
    create_time timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT uniq_barrier UNIQUE (gid, branch_id, op, barrier_id)
);
