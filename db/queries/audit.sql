-- name: LastAuditHash :one
SELECT hash FROM audit_ereignis ORDER BY rowid DESC LIMIT 1;

-- name: InsertAudit :exec
INSERT INTO audit_ereignis (
  id, zeitpunkt, akteur_nutzer_id, akteur_art, aktion, objekt_typ, objekt_id,
  vorher, nachher, grund, ip, vorgaenger_hash, hash
) VALUES (
  sqlc.arg(id), sqlc.arg(zeitpunkt), sqlc.narg(akteur_nutzer_id), sqlc.arg(akteur_art),
  sqlc.arg(aktion), sqlc.arg(objekt_typ), sqlc.arg(objekt_id), sqlc.narg(vorher),
  sqlc.narg(nachher), sqlc.narg(grund), sqlc.arg(ip), sqlc.arg(vorgaenger_hash), sqlc.arg(hash)
);

-- name: ListAudit :many
SELECT * FROM audit_ereignis ORDER BY rowid ASC;

-- name: ListAuditFiltered :many
SELECT * FROM audit_ereignis
WHERE (sqlc.narg(objekt_typ) IS NULL OR objekt_typ = sqlc.narg(objekt_typ))
  AND (sqlc.narg(objekt_id) IS NULL OR objekt_id = sqlc.narg(objekt_id))
ORDER BY zeitpunkt DESC, id DESC
LIMIT sqlc.arg(limit_n);

-- name: ListAuditForNutzer :many
SELECT * FROM audit_ereignis
WHERE akteur_nutzer_id = sqlc.arg(nutzer_id)
  AND (sqlc.narg(objekt_typ) IS NULL OR objekt_typ = sqlc.narg(objekt_typ))
  AND (sqlc.narg(objekt_id) IS NULL OR objekt_id = sqlc.narg(objekt_id))
ORDER BY zeitpunkt DESC, id DESC
LIMIT sqlc.arg(limit_n);
