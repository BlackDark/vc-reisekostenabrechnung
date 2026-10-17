-- +goose Up
-- The session table stored the token as the client sent it, so a copy of the
-- database handed out live sessions for as long as they lasted (idle 168h,
-- lifetime 720h). The column now holds the SHA-256 hex of the token, as the
-- api_token table already does, and the name says so: nobody writes a plaintext
-- token back into it without noticing.
--
-- Rows written before this migration hold the token itself and can never match a
-- hashed lookup, so they are dropped. Those sessions end and their users sign in
-- again; nothing else about them is worth keeping.
ALTER TABLE session RENAME COLUMN token TO token_hash;
DELETE FROM session;

-- +goose Down
-- The deleted rows held bearer tokens of sessions that are over, so they cannot
-- come back. Only the column name is reversible.
ALTER TABLE session RENAME COLUMN token_hash TO token;
