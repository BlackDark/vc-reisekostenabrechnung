-- name: UpsertSession :exec
INSERT INTO session (
  token, data, expiry, nutzer_id, user_agent, ip, oeffentlich_id, erstellt_am, letzte_nutzung
) VALUES (
  sqlc.arg(token), sqlc.arg(data), sqlc.arg(expiry), sqlc.narg(nutzer_id), sqlc.narg(user_agent),
  sqlc.narg(ip), sqlc.arg(oeffentlich_id), sqlc.arg(erstellt_am), sqlc.arg(letzte_nutzung)
)
ON CONFLICT (token) DO UPDATE SET
  data = excluded.data,
  expiry = excluded.expiry,
  nutzer_id = COALESCE(excluded.nutzer_id, session.nutzer_id),
  user_agent = COALESCE(excluded.user_agent, session.user_agent),
  ip = COALESCE(excluded.ip, session.ip),
  letzte_nutzung = excluded.letzte_nutzung;

-- name: GetSession :one
SELECT * FROM session WHERE token = sqlc.arg(token);

-- name: DeleteSession :exec
DELETE FROM session WHERE token = sqlc.arg(token);

-- name: DeleteSessionByPublicID :exec
DELETE FROM session WHERE oeffentlich_id = sqlc.arg(oeffentlich_id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: DeleteSessionsForNutzer :exec
DELETE FROM session WHERE nutzer_id = sqlc.arg(nutzer_id);

-- name: ListSessionsByNutzer :many
SELECT * FROM session
WHERE nutzer_id = sqlc.arg(nutzer_id) AND expiry > sqlc.arg(now)
ORDER BY letzte_nutzung DESC;
