-- name: CountNutzer :one
SELECT CAST(COUNT(*) AS BIGINT) FROM nutzer;

-- name: CountAktiveAdmins :one
SELECT CAST(COUNT(*) AS BIGINT) FROM nutzer
WHERE aktiv = TRUE AND (ist_admin_lokal = TRUE OR admin_ueber_gruppe = TRUE);

-- name: CreateNutzer :one
INSERT INTO nutzer (
  id, anzeigename, email, benutzername, passwort_hash,
  ist_admin_lokal, admin_ueber_gruppe, sprache, personalnummer,
  ki_erlaubt, aktiv, letzte_anmeldung, erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(anzeigename), sqlc.narg(email), sqlc.arg(benutzername), sqlc.narg(passwort_hash),
  sqlc.arg(ist_admin_lokal), sqlc.arg(admin_ueber_gruppe), sqlc.arg(sprache), sqlc.narg(personalnummer),
  sqlc.arg(ki_erlaubt), sqlc.arg(aktiv), sqlc.narg(letzte_anmeldung), sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: GetNutzerByID :one
SELECT * FROM nutzer WHERE id = sqlc.arg(id);

-- name: GetNutzerByBenutzername :one
SELECT * FROM nutzer WHERE benutzername = sqlc.arg(benutzername);

-- name: GetNutzerByEmail :one
SELECT * FROM nutzer WHERE email = sqlc.arg(email);

-- name: ListNutzer :many
SELECT * FROM nutzer
WHERE sqlc.narg(cursor) IS NULL OR id > sqlc.narg(cursor)
ORDER BY id
LIMIT sqlc.arg(limit_n);

-- name: UpdateNutzerProfil :one
UPDATE nutzer SET
  anzeigename = sqlc.arg(anzeigename),
  sprache = sqlc.arg(sprache),
  personalnummer = sqlc.narg(personalnummer),
  ki_erlaubt = sqlc.arg(ki_erlaubt),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: UpdateNutzerPasswort :one
UPDATE nutzer SET
  passwort_hash = sqlc.arg(passwort_hash),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: UpdateNutzerAdmin :one
UPDATE nutzer SET
  aktiv = sqlc.arg(aktiv),
  ist_admin_lokal = sqlc.arg(ist_admin_lokal),
  passwort_hash = COALESCE(sqlc.narg(passwort_hash), passwort_hash),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version)
RETURNING *;

-- name: TouchAnmeldung :exec
UPDATE nutzer SET letzte_anmeldung = sqlc.arg(letzte_anmeldung) WHERE id = sqlc.arg(id);

-- name: SetAdminUeberGruppe :exec
UPDATE nutzer SET
  admin_ueber_gruppe = sqlc.arg(admin_ueber_gruppe),
  geaendert_am = sqlc.arg(geaendert_am)
WHERE id = sqlc.arg(id);
