-- +goose Up
-- Portable subset of SQLite and Postgres: TEXT, BIGINT, BOOLEAN, TIMESTAMP.
-- vorschuss.abrechnung_id already exists (00005) without a foreign key.
-- SQLite cannot add a constraint to that column; I12 is enforced in the store.
CREATE TABLE abrechnung (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  arbeitgeber_id TEXT NOT NULL REFERENCES arbeitgeber (id),
  abrechnungsnummer TEXT,
  zeitraum_art TEXT NOT NULL,
  von TEXT NOT NULL,
  bis TEXT NOT NULL,
  titel TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'entwurf',
  eingereicht_am TIMESTAMP,
  bezahlt_am TEXT,
  bezahlt_vermerk TEXT,
  export_sprache TEXT NOT NULL DEFAULT 'de',
  aktuelle_export_version BIGINT NOT NULL DEFAULT 0,
  einreichung_laeuft BOOLEAN NOT NULL DEFAULT FALSE,
  einreichung_fehler TEXT,
  quittierte_warnungen TEXT NOT NULL DEFAULT '[]',
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (zeitraum_art IN ('tag', 'woche', 'monat', 'quartal', 'frei')),
  CHECK (status IN ('entwurf', 'eingereicht', 'bezahlt')),
  CHECK (export_sprache IN ('de', 'en')),
  CHECK (von <= bis)
);

CREATE INDEX abrechnung_nutzer_idx ON abrechnung (nutzer_id, id);

CREATE UNIQUE INDEX abrechnung_nummer_uidx
  ON abrechnung (nutzer_id, arbeitgeber_id, abrechnungsnummer)
  WHERE abrechnungsnummer IS NOT NULL;

CREATE TABLE abrechnung_reise (
  abrechnung_id TEXT NOT NULL REFERENCES abrechnung (id) ON DELETE CASCADE,
  reise_id TEXT NOT NULL REFERENCES reise (id),
  PRIMARY KEY (abrechnung_id, reise_id)
);

CREATE UNIQUE INDEX abrechnung_reise_eine_uidx ON abrechnung_reise (reise_id);

CREATE TABLE export (
  id TEXT PRIMARY KEY,
  abrechnung_id TEXT NOT NULL REFERENCES abrechnung (id) ON DELETE CASCADE,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  version BIGINT NOT NULL,
  anlass TEXT NOT NULL,
  erstellt_am TIMESTAMP NOT NULL,
  pdf_schluessel TEXT,
  pdf_sha256 TEXT,
  zip_schluessel TEXT,
  zip_sha256 TEXT,
  snapshot TEXT NOT NULL,
  ersetzt_durch_version BIGINT,
  status TEXT NOT NULL,
  fehler TEXT,
  UNIQUE (abrechnung_id, version),
  CHECK (anlass IN ('einreichung', 'einreichung_nach_entsperrung')),
  CHECK (status IN ('in_erstellung', 'fertig', 'fehlgeschlagen'))
);

CREATE INDEX export_abrechnung_idx ON export (abrechnung_id, version);

CREATE TABLE abrechnungsnummer_folge (
  nutzer_id TEXT NOT NULL,
  arbeitgeber_id TEXT NOT NULL,
  jahr BIGINT NOT NULL,
  naechste BIGINT NOT NULL,
  PRIMARY KEY (nutzer_id, arbeitgeber_id, jahr)
);

-- +goose Down
DROP TABLE abrechnungsnummer_folge;
DROP TABLE export;
DROP TABLE abrechnung_reise;
DROP TABLE abrechnung;
