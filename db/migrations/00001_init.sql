-- +goose Up
-- Portable subset of SQLite and Postgres: TEXT, BIGINT, BOOLEAN, TIMESTAMP.
CREATE TABLE nutzer (
  id TEXT PRIMARY KEY,
  anzeigename TEXT NOT NULL,
  email TEXT,
  benutzername TEXT NOT NULL,
  passwort_hash TEXT,
  ist_admin_lokal BOOLEAN NOT NULL DEFAULT FALSE,
  admin_ueber_gruppe BOOLEAN NOT NULL DEFAULT FALSE,
  sprache TEXT NOT NULL DEFAULT 'de',
  personalnummer TEXT,
  ki_erlaubt BOOLEAN NOT NULL DEFAULT FALSE,
  aktiv BOOLEAN NOT NULL DEFAULT TRUE,
  letzte_anmeldung TIMESTAMP,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (sprache IN ('de', 'en'))
);

CREATE UNIQUE INDEX nutzer_benutzername_uidx ON nutzer (benutzername);
CREATE UNIQUE INDEX nutzer_email_uidx ON nutzer (email);

CREATE TABLE nutzer_identitaet (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  art TEXT NOT NULL,
  aussteller TEXT NOT NULL,
  subjekt TEXT NOT NULL,
  zuletzt_gesehen TIMESTAMP,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  UNIQUE (art, aussteller, subjekt),
  CHECK (art IN ('oidc', 'header'))
);

CREATE INDEX nutzer_identitaet_nutzer_idx ON nutzer_identitaet (nutzer_id);

CREATE TABLE session (
  token TEXT PRIMARY KEY,
  data TEXT NOT NULL,
  expiry TIMESTAMP NOT NULL,
  nutzer_id TEXT,
  user_agent TEXT,
  ip TEXT,
  oeffentlich_id TEXT NOT NULL,
  erstellt_am TIMESTAMP NOT NULL,
  letzte_nutzung TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX session_oeffentlich_id_uidx ON session (oeffentlich_id);
CREATE INDEX session_nutzer_idx ON session (nutzer_id);
CREATE INDEX session_expiry_idx ON session (expiry);

CREATE TABLE audit_ereignis (
  id TEXT PRIMARY KEY,
  zeitpunkt TIMESTAMP NOT NULL,
  akteur_nutzer_id TEXT,
  akteur_art TEXT NOT NULL,
  aktion TEXT NOT NULL,
  objekt_typ TEXT NOT NULL,
  objekt_id TEXT NOT NULL,
  vorher TEXT,
  nachher TEXT,
  grund TEXT,
  ip TEXT,
  vorgaenger_hash TEXT NOT NULL,
  hash TEXT NOT NULL,
  CHECK (akteur_art IN ('nutzer', 'admin', 'system'))
);

CREATE INDEX audit_ereignis_objekt_idx ON audit_ereignis (objekt_typ, objekt_id);

CREATE TABLE job (
  id TEXT PRIMARY KEY,
  art TEXT NOT NULL,
  status TEXT NOT NULL,
  payload TEXT,
  versuche BIGINT NOT NULL DEFAULT 0,
  naechster_versuch_am TIMESTAMP,
  fehler TEXT,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  CHECK (status IN ('pending', 'running', 'done', 'failed'))
);

CREATE INDEX job_status_idx ON job (status, naechster_versuch_am);

-- +goose Down
DROP TABLE job;
DROP TABLE audit_ereignis;
DROP TABLE session;
DROP TABLE nutzer_identitaet;
DROP TABLE nutzer;
