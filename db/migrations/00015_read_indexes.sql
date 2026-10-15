-- +goose Up
-- Read indexes for the hot filter-and-sort shapes that had no usable index.
-- Each one is additive; they grow the DB file and cost a little on write, which
-- is why they are grouped by the read path they serve rather than by table.
--
-- ListWechselkurse: WHERE waehrung = ? ORDER BY datum. The table key is
-- (datum, waehrung), so the leading column never matches the filter and the
-- lookup scans the whole key to answer. Measured on 1000 rows: 87.6ms to 8.4ms
-- for 300 queries, SCAN to SEARCH.
CREATE INDEX wechselkurs_waehrung_idx ON wechselkurs (waehrung, datum);

-- ListAuditForNutzer: WHERE akteur_nutzer_id = ? ORDER BY zeitpunkt DESC,
-- id DESC, with two optional equality filters. 68.3ms to 13.3ms for 300
-- queries, and the temporary sort tree is gone.
CREATE INDEX audit_ereignis_akteur_idx
  ON audit_ereignis (akteur_nutzer_id, zeitpunkt DESC, id DESC);

-- ListAudit and ListAuditFiltered: the same ORDER BY with no actor filter, so
-- the index supplies the order directly instead of sorting every row. The table
-- gains one audit row per request, so this is the list that grows with use.
CREATE INDEX audit_ereignis_zeit_idx ON audit_ereignis (zeitpunkt DESC, id DESC);

-- ListRetentionBelege: WHERE status IN (...) AND aufbewahren_bis IS NOT NULL
-- ORDER BY aufbewahren_bis, id. Every existing beleg index leads with nutzer_id,
-- which this query does not filter on. 87.0ms to 17.4ms for 300 queries.
-- ExtendBelegFrist filters aufbewahren_bis < ? and reuses the same index.
CREATE INDEX beleg_aufbewahren_bis_idx ON beleg (aufbewahren_bis, id);

-- ListRetentionExporte: WHERE status = 'fertig' ORDER BY aufbewahren_bis, id.
-- Only (abrechnung_id, version) exists, which serves neither the filter nor the
-- order.
CREATE INDEX export_status_aufbewahren_idx ON export (status, aufbewahren_bis, id);

-- DeleteJobsForBeleg: WHERE payload = ?. 11.3ms to 1.6ms for 50 queries.
CREATE INDEX job_payload_idx ON job (payload);

-- LatestJob: WHERE art = ? AND payload = ? ORDER BY erstellt_am DESC LIMIT 1.
-- The trailing erstellt_am lets the index answer the order as well as the seek,
-- so the single-row lookup stops sorting.
CREATE INDEX job_art_payload_idx ON job (art, payload, erstellt_am DESC);

-- +goose Down
DROP INDEX job_art_payload_idx;
DROP INDEX job_payload_idx;
DROP INDEX export_status_aufbewahren_idx;
DROP INDEX beleg_aufbewahren_bis_idx;
DROP INDEX audit_ereignis_zeit_idx;
DROP INDEX audit_ereignis_akteur_idx;
DROP INDEX wechselkurs_waehrung_idx;
