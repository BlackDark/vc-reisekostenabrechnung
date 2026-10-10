-- +goose Up
-- 'bild' is the display rendition (WebP, longest side 1600 px) next to the 320 px
-- 'vorschau'. SQLite cannot change a CHECK, so the table is rebuilt and refilled.
CREATE TABLE belegdatei_2026_2 (
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
  CHECK (variante IN ('erfassung', 'archiv', 'vorschau', 'bild', 'export_jpeg', 'original'))
);

INSERT INTO belegdatei_2026_2 (
  id, beleg_id, variante, seite, speicher_schluessel, mime, bytes, sha256, unveraenderbar
)
SELECT id, beleg_id, variante, seite, speicher_schluessel, mime, bytes, sha256, unveraenderbar
FROM belegdatei;

DROP TABLE belegdatei;
ALTER TABLE belegdatei_2026_2 RENAME TO belegdatei;
CREATE INDEX belegdatei_sha_idx ON belegdatei (sha256);

-- +goose Down
CREATE TABLE belegdatei_2026_1 (
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

INSERT INTO belegdatei_2026_1 (
  id, beleg_id, variante, seite, speicher_schluessel, mime, bytes, sha256, unveraenderbar
)
SELECT id, beleg_id, variante, seite, speicher_schluessel, mime, bytes, sha256, unveraenderbar
FROM belegdatei
WHERE variante != 'bild';

DROP TABLE belegdatei;
ALTER TABLE belegdatei_2026_1 RENAME TO belegdatei;
CREATE INDEX belegdatei_sha_idx ON belegdatei (sha256);