-- +goose Up
-- Portable subset of SQLite and Postgres: TEXT, BIGINT, BOOLEAN, TIMESTAMP.
CREATE TABLE vorlage (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  name TEXT NOT NULL,
  art TEXT NOT NULL,
  daten TEXT NOT NULL,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (art IN ('reise', 'fahrt'))
);

CREATE INDEX vorlage_nutzer_idx ON vorlage (nutzer_id, id);

CREATE TABLE reise (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  arbeitgeber_id TEXT NOT NULL REFERENCES arbeitgeber (id),
  anlass TEXT NOT NULL,
  projekt TEXT,
  beginn TIMESTAMP NOT NULL,
  beginn_zone TEXT NOT NULL,
  ende TIMESTAMP NOT NULL,
  ende_zone TEXT NOT NULL,
  notiz TEXT,
  vorlage_id TEXT REFERENCES vorlage (id),
  status TEXT NOT NULL DEFAULT 'offen',
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (status IN ('offen', 'in_entwurf', 'gesperrt'))
);

CREATE INDEX reise_nutzer_idx ON reise (nutzer_id, id);
CREATE INDEX reise_zeit_idx ON reise (nutzer_id, beginn);

CREATE TABLE ortswechsel (
  id TEXT PRIMARY KEY,
  reise_id TEXT NOT NULL REFERENCES reise (id) ON DELETE CASCADE,
  reihenfolge BIGINT NOT NULL,
  abfahrt TIMESTAMP,
  abfahrt_zone TEXT,
  ankunft TIMESTAMP NOT NULL,
  ankunft_zone TEXT NOT NULL,
  verkehrsmittel TEXT NOT NULL,
  land_iso TEXT NOT NULL,
  satzort TEXT NOT NULL DEFAULT '',
  ort TEXT NOT NULL DEFAULT '',
  taetigkeitsstaette_id TEXT REFERENCES taetigkeitsstaette (id),
  zwischenlandung_mit_uebernachtung BOOLEAN NOT NULL DEFAULT FALSE,
  UNIQUE (reise_id, reihenfolge),
  CHECK (verkehrsmittel IN ('pkw', 'bahn', 'flug', 'schiff', 'bus', 'sonstiges'))
);

CREATE INDEX ortswechsel_reise_idx ON ortswechsel (reise_id, reihenfolge);

CREATE TABLE reisetag (
  id TEXT PRIMARY KEY,
  reise_id TEXT NOT NULL REFERENCES reise (id) ON DELETE CASCADE,
  datum TEXT NOT NULL,
  land_manuell TEXT,
  satzort_manuell TEXT,
  land_begruendung TEXT,
  fruehstueck_gestellt BOOLEAN NOT NULL DEFAULT FALSE,
  mittag_gestellt BOOLEAN NOT NULL DEFAULT FALSE,
  abend_gestellt BOOLEAN NOT NULL DEFAULT FALSE,
  zuzahlung_fruehstueck BIGINT NOT NULL DEFAULT 0,
  zuzahlung_mittag BIGINT NOT NULL DEFAULT 0,
  zuzahlung_abend BIGINT NOT NULL DEFAULT 0,
  mahlzeit_quelle TEXT NOT NULL DEFAULT '',
  unterkunft TEXT NOT NULL DEFAULT 'keine',
  verpflegung_ausgeschlossen BOOLEAN NOT NULL DEFAULT FALSE,
  ausschluss_grund TEXT,
  UNIQUE (reise_id, datum),
  CHECK (unterkunft IN ('keine', 'beleg', 'pauschale', 'gestellt', 'verkehrsmittel'))
);

CREATE INDEX reisetag_reise_idx ON reisetag (reise_id, datum);

CREATE TABLE fahrt (
  id TEXT PRIMARY KEY,
  reise_id TEXT NOT NULL REFERENCES reise (id) ON DELETE CASCADE,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  datum TEXT NOT NULL,
  start_ort TEXT NOT NULL,
  ziel TEXT NOT NULL,
  zweck TEXT,
  fahrzeugart TEXT NOT NULL,
  km BIGINT NOT NULL,
  hin_und_zurueck BOOLEAN NOT NULL DEFAULT FALSE,
  vorlage_id TEXT REFERENCES vorlage (id),
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (fahrzeugart IN ('kraftwagen', 'anderes_motorfahrzeug')),
  CHECK (km > 0)
);

CREATE INDEX fahrt_nutzer_idx ON fahrt (nutzer_id, id);
CREATE INDEX fahrt_reise_idx ON fahrt (reise_id, datum);

-- +goose Down
DROP TABLE fahrt;
DROP TABLE reisetag;
DROP TABLE ortswechsel;
DROP TABLE reise;
DROP TABLE vorlage;
