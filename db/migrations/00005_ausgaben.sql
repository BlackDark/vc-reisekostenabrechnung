-- +goose Up
-- Portable subset of SQLite and Postgres: TEXT, BIGINT, BOOLEAN, TIMESTAMP.
CREATE TABLE ausgabe (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  reise_id TEXT NOT NULL REFERENCES reise (id) ON DELETE CASCADE,
  kostenart TEXT NOT NULL,
  datum TEXT NOT NULL,
  leistender TEXT NOT NULL DEFAULT '',
  beschreibung TEXT,
  waehrung TEXT NOT NULL DEFAULT 'EUR',
  betrag BIGINT NOT NULL,
  kurs TEXT,
  kurs_quelle TEXT,
  kurs_datum TEXT,
  kurs_grund TEXT,
  betrag_eur BIGINT NOT NULL DEFAULT 0,
  rechnungsart TEXT NOT NULL DEFAULT 'kleinbetragsrechnung',
  rechnung_auf_arbeitgeber BOOLEAN NOT NULL DEFAULT FALSE,
  empfaenger TEXT,
  verkehrsmittel TEXT,
  uebernachtung_naechte TEXT NOT NULL DEFAULT '[]',
  fruehstueck_enthalten BOOLEAN NOT NULL DEFAULT FALSE,
  mahlzeit TEXT,
  bewirtung TEXT,
  tse_beleg BOOLEAN NOT NULL DEFAULT FALSE,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (kostenart IN ('fahrtkosten', 'verpflegung', 'uebernachtung', 'reisenebenkosten', 'bewirtung')),
  CHECK (betrag > 0),
  CHECK (rechnungsart IN ('kleinbetragsrechnung', 'rechnung', 'fahrausweis', 'e_rechnung', 'eigenbeleg')),
  CHECK (kurs_quelle IS NULL OR kurs_quelle IN ('ezb', 'manuell', 'belastung')),
  CHECK (mahlzeit IS NULL OR mahlzeit IN ('fruehstueck', 'mittag', 'abend')),
  CHECK (verkehrsmittel IS NULL OR verkehrsmittel IN ('bahn', 'flug', 'oepnv', 'taxi', 'mietwagen', 'dienstwagen_kraftstoff', 'sonstiges'))
);

CREATE INDEX ausgabe_reise_idx ON ausgabe (reise_id, datum);
CREATE INDEX ausgabe_nutzer_idx ON ausgabe (nutzer_id, datum, betrag);

CREATE TABLE steueranteil (
  id TEXT PRIMARY KEY,
  ausgabe_id TEXT NOT NULL REFERENCES ausgabe (id) ON DELETE CASCADE,
  position BIGINT NOT NULL,
  satz BIGINT NOT NULL,
  steuerland TEXT NOT NULL DEFAULT 'DE',
  netto BIGINT NOT NULL,
  steuer BIGINT NOT NULL,
  brutto BIGINT NOT NULL,
  netto_eur BIGINT NOT NULL DEFAULT 0,
  steuer_eur BIGINT NOT NULL DEFAULT 0,
  brutto_eur BIGINT NOT NULL DEFAULT 0,
  UNIQUE (ausgabe_id, position)
);

CREATE TABLE ausgabe_beleg (
  ausgabe_id TEXT NOT NULL REFERENCES ausgabe (id) ON DELETE CASCADE,
  beleg_id TEXT NOT NULL REFERENCES beleg (id) ON DELETE CASCADE,
  PRIMARY KEY (ausgabe_id, beleg_id)
);

CREATE INDEX ausgabe_beleg_beleg_idx ON ausgabe_beleg (beleg_id);

CREATE TABLE eigenbeleg (
  ausgabe_id TEXT PRIMARY KEY REFERENCES ausgabe (id) ON DELETE CASCADE,
  grund TEXT NOT NULL,
  zahlungsempfaenger TEXT NOT NULL,
  art TEXT NOT NULL,
  erstellt_am TIMESTAMP NOT NULL,
  bestaetigt_am TIMESTAMP,
  bestaetigt_von TEXT
);

CREATE TABLE wechselkurs (
  datum TEXT NOT NULL,
  waehrung TEXT NOT NULL,
  kurs TEXT NOT NULL,
  quelle TEXT NOT NULL DEFAULT 'ezb',
  abgerufen_am TIMESTAMP NOT NULL,
  PRIMARY KEY (datum, waehrung)
);

CREATE TABLE ust_monatskurs (
  jahr BIGINT NOT NULL,
  monat BIGINT NOT NULL,
  waehrung TEXT NOT NULL,
  kurs TEXT NOT NULL,
  abgerufen_am TIMESTAMP NOT NULL,
  PRIMARY KEY (jahr, monat, waehrung)
);

CREATE TABLE vorschuss (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL REFERENCES nutzer (id),
  arbeitgeber_id TEXT NOT NULL REFERENCES arbeitgeber (id),
  datum TEXT NOT NULL,
  betrag BIGINT NOT NULL,
  notiz TEXT,
  abrechnung_id TEXT,
  erstellt_am TIMESTAMP NOT NULL,
  geaendert_am TIMESTAMP NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  CHECK (betrag > 0)
);

CREATE INDEX vorschuss_nutzer_idx ON vorschuss (nutzer_id, datum);

-- +goose Down
DROP TABLE vorschuss;
DROP TABLE ust_monatskurs;
DROP TABLE wechselkurs;
DROP TABLE eigenbeleg;
DROP TABLE ausgabe_beleg;
DROP TABLE steueranteil;
DROP TABLE ausgabe;
