-- +goose Up
-- Audit rows are chained in write order, so the chain needs an order that both
-- SQLite and PostgreSQL understand. ids are millisecond UUIDv7, which is not
-- enough on its own because two rows can share a millisecond. nr carries the
-- write sequence explicitly; it backfills from the existing id order.
ALTER TABLE audit_ereignis ADD COLUMN nr BIGINT NOT NULL DEFAULT 0;
UPDATE audit_ereignis SET nr = (SELECT COUNT(*) FROM audit_ereignis e2 WHERE e2.id <= audit_ereignis.id);
CREATE INDEX audit_ereignis_nr_idx ON audit_ereignis (nr);

-- +goose Down
DROP INDEX audit_ereignis_nr_idx;
ALTER TABLE audit_ereignis DROP COLUMN nr;
