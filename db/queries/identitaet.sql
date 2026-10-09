-- name: CreateIdentitaet :one
INSERT INTO nutzer_identitaet (
  id, nutzer_id, art, aussteller, subjekt, zuletzt_gesehen, erstellt_am, geaendert_am
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(art), sqlc.arg(aussteller), sqlc.arg(subjekt),
  sqlc.arg(zuletzt_gesehen), sqlc.arg(erstellt_am), sqlc.arg(geaendert_am)
)
RETURNING *;

-- name: GetIdentitaet :one
SELECT * FROM nutzer_identitaet
WHERE art = sqlc.arg(art) AND aussteller = sqlc.arg(aussteller) AND subjekt = sqlc.arg(subjekt);

-- name: ListIdentitaetenByNutzer :many
SELECT * FROM nutzer_identitaet WHERE nutzer_id = sqlc.arg(nutzer_id) ORDER BY id;

-- name: GetIdentitaetByID :one
SELECT * FROM nutzer_identitaet WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: DeleteIdentitaet :exec
DELETE FROM nutzer_identitaet WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: TouchIdentitaet :exec
UPDATE nutzer_identitaet SET
  zuletzt_gesehen = sqlc.arg(zuletzt_gesehen),
  geaendert_am = sqlc.arg(geaendert_am)
WHERE id = sqlc.arg(id);
