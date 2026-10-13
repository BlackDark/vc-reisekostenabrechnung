-- +goose Up
CREATE TABLE mail_erlaubt (
  nutzer_id TEXT NOT NULL,
  wert TEXT NOT NULL,
  erstellt_am TEXT NOT NULL,
  PRIMARY KEY (nutzer_id, wert)
);
CREATE INDEX mail_erlaubt_wert_idx ON mail_erlaubt (wert);

-- +goose Down
DROP INDEX mail_erlaubt_wert_idx;
DROP TABLE mail_erlaubt;
