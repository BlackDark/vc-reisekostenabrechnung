-- +goose Up
CREATE TABLE api_token (
  id TEXT PRIMARY KEY,
  nutzer_id TEXT NOT NULL,
  name TEXT NOT NULL,
  scope TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  erstellt_am TEXT NOT NULL,
  zuletzt_benutzt_am TEXT,
  widerrufen_am TEXT
);
CREATE INDEX api_token_nutzer_idx ON api_token (nutzer_id);

-- +goose Down
DROP INDEX api_token_nutzer_idx;
DROP TABLE api_token;
