-- +goose Up
-- Portable subset of SQLite and Postgres: TEXT, TIMESTAMP.
-- Retention metadata stays after the files are removed (SPEC 12.4).
ALTER TABLE beleg ADD COLUMN inhalt_geloescht_am TIMESTAMP;
ALTER TABLE beleg ADD COLUMN loesch_grund TEXT;

ALTER TABLE export ADD COLUMN aufbewahren_bis TEXT;
ALTER TABLE export ADD COLUMN inhalt_geloescht_am TIMESTAMP;
ALTER TABLE export ADD COLUMN loesch_grund TEXT;

-- +goose Down
ALTER TABLE export DROP COLUMN loesch_grund;
ALTER TABLE export DROP COLUMN inhalt_geloescht_am;
ALTER TABLE export DROP COLUMN aufbewahren_bis;
ALTER TABLE beleg DROP COLUMN loesch_grund;
ALTER TABLE beleg DROP COLUMN inhalt_geloescht_am;
