-- name: InsertTaetigkeitsstaette :one
INSERT INTO taetigkeitsstaette (
  id, nutzer_id, bezeichnung, anschrift, land_iso, satzort, kunde,
  erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(bezeichnung), sqlc.arg(anschrift),
  sqlc.arg(land_iso), sqlc.arg(satzort), sqlc.narg(kunde),
  sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: GetTaetigkeitsstaette :one
SELECT * FROM taetigkeitsstaette
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: ListTaetigkeitsstaetten :many
SELECT * FROM taetigkeitsstaette
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND (sqlc.narg(cursor) IS NULL OR id > sqlc.narg(cursor))
ORDER BY id
LIMIT sqlc.arg(limit_n);

-- name: UpdateTaetigkeitsstaette :one
UPDATE taetigkeitsstaette SET
  bezeichnung = sqlc.arg(bezeichnung),
  anschrift = sqlc.arg(anschrift),
  land_iso = sqlc.arg(land_iso),
  satzort = sqlc.arg(satzort),
  kunde = sqlc.narg(kunde),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DeleteTaetigkeitsstaette :exec
DELETE FROM taetigkeitsstaette
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);
