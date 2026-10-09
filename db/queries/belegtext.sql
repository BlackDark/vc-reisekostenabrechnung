-- name: MaxBelegtextVersion :one
SELECT CAST(COALESCE(MAX(version), 0) AS BIGINT) AS version
FROM belegtext
WHERE beleg_id = sqlc.arg(beleg_id);

-- name: InsertBelegtext :one
INSERT INTO belegtext (
  id, beleg_id, version, quelle, felder, volltext, bestaetigt_am, bestaetigt_von
) VALUES (
  sqlc.arg(id), sqlc.arg(beleg_id), sqlc.arg(version), sqlc.arg(quelle),
  sqlc.arg(felder), sqlc.arg(volltext), sqlc.narg(bestaetigt_am), sqlc.narg(bestaetigt_von)
)
RETURNING *;

-- name: ListBelegtexte :many
SELECT id, beleg_id, version, quelle, felder, volltext, bestaetigt_am, bestaetigt_von
FROM belegtext
WHERE beleg_id = sqlc.arg(beleg_id)
ORDER BY version;

-- name: LatestJob :one
SELECT id, art, status, payload, versuche, naechster_versuch_am, fehler, erstellt_am, geaendert_am
FROM job
WHERE art = sqlc.arg(art) AND payload = sqlc.arg(payload)
ORDER BY erstellt_am DESC
LIMIT 1;
