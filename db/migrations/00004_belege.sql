-- +goose Up
-- Portable subset of SQLite and Postgres: TEXT, BIGINT, BOOLEAN, TIMESTAMP.
CREATE TABLE beleg (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  belegnummer TEXT,
  typ TEXT NOT NULL,
  status TEXT NOT NULL,
  seiten BIGINT NOT NULL,
  sha256_original TEXT NOT NULL,
  pipeline_version TEXT NOT NULL DEFAULT '',
  pipeline_parameter TEXT NOT NULL DEFAULT '{}',
  bestaetigt_am TIMESTAMP,
  bestaetigt_von TEXT,
  storno_grund TEXT,
  storniert_am TIMESTAMP,
  aufbewahren_bis TEXT,
  erfassung_loeschen_am TIMESTAMP,
  duplikat_von TEXT,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (typ IN ('foto', 'pdf', 'e_rechnung_xml', 'e_rechnung_hybrid')),
  CHECK (status IN ('hochgeladen', 'in_aufbereitung', 'zur_bestaetigung', 'bestaetigt', 'fehlgeschlagen', 'storniert'))
);

CREATE UNIQUE INDEX beleg_nummer_idx ON beleg (nutzer_id, belegnummer) WHERE belegnummer IS NOT NULL;
CREATE INDEX beleg_sha_idx ON beleg (nutzer_id, sha256_original);
CREATE INDEX beleg_status_idx ON beleg (nutzer_id, status);

CREATE TABLE belegdatei (
  id TEXT PRIMARY KEY,
  beleg_id TEXT NOT NULL REFERENCES beleg (id) ON DELETE CASCADE,
  variante TEXT NOT NULL,
  seite BIGINT NOT NULL,
  speicher_schluessel TEXT NOT NULL,
  mime TEXT NOT NULL,
  bytes BIGINT NOT NULL,
  sha256 TEXT NOT NULL,
  unveraenderbar BOOLEAN NOT NULL DEFAULT FALSE,
  UNIQUE (beleg_id, variante, seite),
  CHECK (variante IN ('erfassung', 'archiv', 'vorschau', 'export_jpeg', 'original'))
);

CREATE INDEX belegdatei_sha_idx ON belegdatei (sha256);

CREATE TABLE belegtext (
  id TEXT PRIMARY KEY,
  beleg_id TEXT NOT NULL REFERENCES beleg (id) ON DELETE CASCADE,
  version BIGINT NOT NULL,
  quelle TEXT NOT NULL,
  felder TEXT NOT NULL DEFAULT '{}',
  volltext TEXT NOT NULL DEFAULT '',
  bestaetigt_am TIMESTAMP,
  bestaetigt_von TEXT,
  UNIQUE (beleg_id, version),
  CHECK (quelle IN ('ki', 'manuell'))
);

CREATE TABLE beleg_nummer (
  nutzer_id TEXT NOT NULL,
  jahr BIGINT NOT NULL,
  naechste BIGINT NOT NULL,
  PRIMARY KEY (nutzer_id, jahr)
);

-- +goose Down
DROP TABLE beleg_nummer;
DROP TABLE belegtext;
DROP TABLE belegdatei;
DROP TABLE beleg;
