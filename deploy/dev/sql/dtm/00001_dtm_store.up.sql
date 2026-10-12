CREATE SEQUENCE IF NOT EXISTS trans_global_seq;
CREATE TABLE IF NOT EXISTS trans_global (
  id bigint NOT NULL DEFAULT NEXTVAL ('trans_global_seq'),
  gid varchar(128) NOT NULL,
  trans_type varchar(45) NOT NULL,
  status varchar(45) NOT NULL,
  query_prepared varchar(1024) NOT NULL,
  protocol varchar(45) NOT NULL,
  create_time timestamp(0) with time zone DEFAULT NULL,
  update_time timestamp(0) with time zone DEFAULT NULL,
  finish_time timestamp(0) with time zone DEFAULT NULL,
  rollback_time timestamp(0) with time zone DEFAULT NULL,
  options varchar(1024) DEFAULT '',
  custom_data varchar(1024) DEFAULT '',
  next_cron_interval int DEFAULT NULL,
  next_cron_time timestamp(0) with time zone DEFAULT NULL,
  owner varchar(128) NOT NULL DEFAULT '',
  ext_data text,
  result varchar(1024) DEFAULT '',
  rollback_reason varchar(1024) DEFAULT '',
  PRIMARY KEY (id),
  -- This is what makes a deterministic gid work: a second Submit() of the same
  -- gid violates it, and DTM answers the duplicate instead of starting a second
  -- global transaction.
  CONSTRAINT gid UNIQUE (gid)
);
CREATE INDEX IF NOT EXISTS owner ON trans_global (owner);
CREATE INDEX IF NOT EXISTS status_next_cron_time ON trans_global (status, next_cron_time);

CREATE SEQUENCE IF NOT EXISTS trans_branch_op_seq;
CREATE TABLE IF NOT EXISTS trans_branch_op (
  id bigint NOT NULL DEFAULT NEXTVAL ('trans_branch_op_seq'),
  gid varchar(128) NOT NULL,
  url varchar(1024) NOT NULL,
  data TEXT,
  bin_data bytea,
  branch_id VARCHAR(128) NOT NULL,
  op varchar(45) NOT NULL,
  status varchar(45) NOT NULL,
  finish_time timestamp(0) with time zone DEFAULT NULL,
  rollback_time timestamp(0) with time zone DEFAULT NULL,
  create_time timestamp(0) with time zone DEFAULT NULL,
  update_time timestamp(0) with time zone DEFAULT NULL,
  PRIMARY KEY (id),
  CONSTRAINT gid_branch_uniq UNIQUE (gid, branch_id, op)
);

CREATE SEQUENCE IF NOT EXISTS kv_seq;
CREATE TABLE IF NOT EXISTS kv (
    id bigint NOT NULL DEFAULT NEXTVAL ('kv_seq'),
    cat varchar(45) NOT NULL,
    k varchar(128) NOT NULL,
    v TEXT,
    version bigint DEFAULT 1,
    create_time timestamp(0) with time zone DEFAULT NULL,
    update_time timestamp(0) with time zone DEFAULT  NULL,
    PRIMARY KEY (id),
    CONSTRAINT uniq_k UNIQUE (cat, k)
);
