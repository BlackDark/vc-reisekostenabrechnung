-- name: InsertToken :exec
INSERT INTO api_token (id, nutzer_id, name, scope, token_hash, erstellt_am)
VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(name), sqlc.arg(scope),
  sqlc.arg(token_hash), sqlc.arg(erstellt_am)
);

-- name: ListTokensByNutzer :many
SELECT * FROM api_token
WHERE nutzer_id = sqlc.arg(nutzer_id)
ORDER BY erstellt_am DESC, id DESC
LIMIT sqlc.arg(limit_n);

-- name: FindTokenByHash :one
SELECT * FROM api_token WHERE token_hash = sqlc.arg(token_hash);

-- name: TouchToken :exec
UPDATE api_token SET zuletzt_benutzt_am = sqlc.arg(am) WHERE id = sqlc.arg(id);

-- name: RevokeToken :execrows
UPDATE api_token SET widerrufen_am = sqlc.arg(am)
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND widerrufen_am IS NULL;
