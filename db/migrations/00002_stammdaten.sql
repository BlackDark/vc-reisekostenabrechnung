-- +goose Up
-- Portable subset of SQLite and Postgres: TEXT, BIGINT, BOOLEAN, TIMESTAMP.
CREATE TABLE datei (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  mime TEXT NOT NULL,
  bytes BIGINT NOT NULL,
  sha256 TEXT NOT NULL,
  speicher_schluessel TEXT NOT NULL,
  erstellt_am TIMESTAMP NOT NULL
);

CREATE INDEX datei_nutzer_idx ON datei (nutzer_id);

CREATE TABLE arbeitgeber (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  name TEXT NOT NULL,
  anschrift TEXT NOT NULL,
  ust_id TEXT,
  steuernummer TEXT,
  logo_datei_id TEXT REFERENCES datei (id),
  ist_standard BOOLEAN NOT NULL DEFAULT FALSE,
  konstellation TEXT NOT NULL DEFAULT 'arbeitgebererstattung',
  abrechnungsnummer_praefix TEXT NOT NULL DEFAULT 'RK',
  archiviert BOOLEAN NOT NULL DEFAULT FALSE,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (konstellation IN ('arbeitgebererstattung', 'werbungskosten', 'betriebsausgaben'))
);

CREATE INDEX arbeitgeber_nutzer_idx ON arbeitgeber (nutzer_id, id);

-- I3: at most one non-archived Standard per Nutzer. The transaction keeps one
-- whenever a non-archived Arbeitgeber exists.
CREATE UNIQUE INDEX arbeitgeber_ein_standard_uidx
  ON arbeitgeber (nutzer_id)
  WHERE ist_standard = TRUE AND archiviert = FALSE;

CREATE TABLE taetigkeitsstaette (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  bezeichnung TEXT NOT NULL,
  anschrift TEXT NOT NULL DEFAULT '',
  land_iso TEXT NOT NULL,
  satzort TEXT NOT NULL DEFAULT '',
  kunde TEXT,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1
);

CREATE INDEX taetigkeitsstaette_nutzer_idx ON taetigkeitsstaette (nutzer_id, id);

CREATE TABLE satztabelle (
  jahr BIGINT PRIMARY KEY,
  status TEXT NOT NULL,
  quelle TEXT NOT NULL,
  vma_inland_24h BIGINT NOT NULL,
  vma_inland_8h BIGINT NOT NULL,
  kuerzung_fruehstueck_pct BIGINT NOT NULL,
  kuerzung_hauptmahlzeit_pct BIGINT NOT NULL,
  uebernachtung_inland_pauschale BIGINT NOT NULL,
  km_kraftwagen BIGINT NOT NULL,
  km_anderes_motorfahrzeug BIGINT NOT NULL,
  sachbezug_fruehstueck BIGINT NOT NULL,
  sachbezug_hauptmahlzeit BIGINT NOT NULL,
  uebliche_mahlzeit_grenze BIGINT NOT NULL,
  kleinbetragsgrenze BIGINT NOT NULL,
  bewirtung_abzug_pct BIGINT NOT NULL,
  ust_saetze TEXT NOT NULL,
  aufbewahrung_jahre BIGINT NOT NULL,
  ersatzlaender TEXT NOT NULL,
  flug_zwischentage_land TEXT NOT NULL,
  schiff_land TEXT NOT NULL,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (status IN ('entwurf', 'aktiv'))
);

CREATE TABLE auslandssatz (
  id TEXT PRIMARY KEY,
  jahr BIGINT NOT NULL REFERENCES satztabelle (jahr),
  land_iso TEXT NOT NULL,
  land_name_de TEXT NOT NULL,
  satzort TEXT NOT NULL DEFAULT '',
  ort_name TEXT NOT NULL DEFAULT '',
  vma_24h BIGINT NOT NULL,
  vma_8h BIGINT NOT NULL,
  uebernachtung BIGINT NOT NULL,
  UNIQUE (jahr, land_iso, satzort)
);

CREATE INDEX auslandssatz_jahr_idx ON auslandssatz (jahr, land_name_de);

CREATE TABLE satz_override (
  id TEXT PRIMARY KEY,
  jahr BIGINT NOT NULL REFERENCES satztabelle (jahr),
  land_iso TEXT,
  satzort TEXT,
  feld TEXT NOT NULL,
  alter_wert TEXT NOT NULL,
  neuer_wert TEXT NOT NULL,
  grund TEXT NOT NULL,
  admin_id TEXT NOT NULL,
  zeitpunkt TIMESTAMP NOT NULL
);

CREATE INDEX satz_override_jahr_idx ON satz_override (jahr, zeitpunkt);

-- +goose Down
DROP TABLE satz_override;
DROP TABLE auslandssatz;
DROP TABLE satztabelle;
DROP TABLE taetigkeitsstaette;
DROP TABLE arbeitgeber;
DROP TABLE datei;
