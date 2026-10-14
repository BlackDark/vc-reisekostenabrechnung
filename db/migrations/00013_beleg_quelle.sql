-- +goose Up
-- Where a Beleg came from (SPEC 21, design note d). Mail-origin receipts also
-- get a beleg_mail row with the sender, subject, date and the archived original
-- .eml, so the Belegeingang can show where a receipt came from and the .eml stays
-- retrievable for GoBD.
ALTER TABLE beleg ADD COLUMN quelle TEXT NOT NULL DEFAULT 'upload';

CREATE TABLE beleg_mail (
  beleg_id TEXT PRIMARY KEY REFERENCES beleg (id) ON DELETE CASCADE,
  absender TEXT NOT NULL,
  betreff TEXT NOT NULL,
  mail_datum TEXT,
  eml_schluessel TEXT NOT NULL,
  erstellt_am TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE beleg_mail;
ALTER TABLE beleg DROP COLUMN quelle;
