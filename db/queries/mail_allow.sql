-- name: ListMailErlaubt :many
SELECT nutzer_id, wert, erstellt_am FROM mail_erlaubt
WHERE nutzer_id = sqlc.arg(nutzer_id)
ORDER BY wert
LIMIT sqlc.arg(limit_n);

-- name: FindMailErlaubtByWert :one
SELECT nutzer_id FROM mail_erlaubt
WHERE wert = sqlc.arg(wert)
LIMIT 1;

-- name: InsertMailErlaubt :exec
INSERT INTO mail_erlaubt (nutzer_id, wert, erstellt_am)
VALUES (sqlc.arg(nutzer_id), sqlc.arg(wert), sqlc.arg(erstellt_am));

-- name: DeleteMailErlaubt :execrows
DELETE FROM mail_erlaubt
WHERE nutzer_id = sqlc.arg(nutzer_id) AND wert = sqlc.arg(wert);
