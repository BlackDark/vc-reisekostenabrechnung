-- +goose Up
-- Sender authentication verdicts the receiving mail server recorded for the
-- original email (design-note security checklist). Intake refuses a mail whose
-- verdict names an unproven sender and keeps the verdict with the Beleg so the
-- Belegeingang can show why a receipt was accepted.
ALTER TABLE beleg_mail ADD COLUMN mail_auth TEXT;

-- +goose Down
ALTER TABLE beleg_mail DROP COLUMN mail_auth;
