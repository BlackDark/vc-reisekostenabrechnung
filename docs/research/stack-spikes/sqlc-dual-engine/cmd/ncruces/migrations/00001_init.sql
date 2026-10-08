-- +goose Up
CREATE TABLE trips (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  title TEXT NOT NULL,
  start_date TEXT NOT NULL,
  end_date TEXT NOT NULL,
  km_total BIGINT NOT NULL DEFAULT 0,
  amount_cents BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX trips_user_idx ON trips(user_id, end_date);
-- +goose Down
DROP TABLE trips;
