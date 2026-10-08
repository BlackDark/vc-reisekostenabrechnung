-- name: CreateTrip :one
INSERT INTO trips (id, user_id, title, start_date, end_date, km_total, amount_cents)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(title), sqlc.arg(start_date), sqlc.arg(end_date), sqlc.arg(km_total), sqlc.arg(amount_cents))
RETURNING *;

-- name: ListTripsInPeriod :many
SELECT * FROM trips
WHERE user_id = sqlc.arg(user_id) AND end_date >= sqlc.arg(from_date) AND end_date <= sqlc.arg(to_date)
ORDER BY end_date DESC
LIMIT sqlc.arg(lim);

-- name: SumAmount :one
SELECT CAST(COALESCE(SUM(amount_cents), 0) AS BIGINT) AS total FROM trips WHERE user_id = sqlc.arg(user_id);
