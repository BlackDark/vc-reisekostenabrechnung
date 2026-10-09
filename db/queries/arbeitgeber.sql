-- name: InsertArbeitgeber :one
INSERT INTO arbeitgeber (
  id, nutzer_id, name, anschrift, ust_id, steuernummer, logo_datei_id,
  ist_standard, konstellation, abrechnungsnummer_praefix, archiviert,
  erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(name), sqlc.arg(anschrift),
  sqlc.narg(ust_id), sqlc.narg(steuernummer), sqlc.narg(logo_datei_id),
  sqlc.arg(ist_standard), sqlc.arg(konstellation), sqlc.arg(abrechnungsnummer_praefix),
  sqlc.arg(archiviert), sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: GetArbeitgeber :one
SELECT * FROM arbeitgeber
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: ListArbeitgeber :many
SELECT * FROM arbeitgeber
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND (sqlc.narg(cursor) IS NULL OR id > sqlc.narg(cursor))
ORDER BY id
LIMIT sqlc.arg(limit_n);

-- name: CountAktiveArbeitgeber :one
SELECT CAST(COUNT(*) AS BIGINT) FROM arbeitgeber
WHERE nutzer_id = sqlc.arg(nutzer_id) AND archiviert = FALSE;

-- name: UpdateArbeitgeber :one
UPDATE arbeitgeber SET
  name = sqlc.arg(name),
  anschrift = sqlc.arg(anschrift),
  ust_id = sqlc.narg(ust_id),
  steuernummer = sqlc.narg(steuernummer),
  logo_datei_id = sqlc.narg(logo_datei_id),
  ist_standard = sqlc.arg(ist_standard),
  konstellation = sqlc.arg(konstellation),
  abrechnungsnummer_praefix = sqlc.arg(abrechnungsnummer_praefix),
  archiviert = sqlc.arg(archiviert),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: ClearAndereStandards :exec
UPDATE arbeitgeber SET
  ist_standard = FALSE,
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND ist_standard = TRUE
  AND archiviert = FALSE
  AND id <> sqlc.arg(id);

-- name: FirstOtherAktiverArbeitgeber :one
SELECT * FROM arbeitgeber
WHERE nutzer_id = sqlc.arg(nutzer_id) AND archiviert = FALSE AND id <> sqlc.arg(id)
ORDER BY erstellt_am, id
LIMIT 1;

-- name: InsertDatei :one
INSERT INTO datei (
  id, nutzer_id, mime, bytes, sha256, speicher_schluessel, erstellt_am
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(mime), sqlc.arg(bytes),
  sqlc.arg(sha256), sqlc.arg(speicher_schluessel), sqlc.arg(erstellt_am)
)
RETURNING *;

-- name: GetDatei :one
SELECT * FROM datei WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);
