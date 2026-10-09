-- name: ListRetentionBelege :many
SELECT id, nutzer_id,
  COALESCE(belegnummer, id) AS bezeichnung,
  COALESCE(aufbewahren_bis, '') AS aufbewahren_bis,
  sha256_original,
  inhalt_geloescht_am
FROM beleg
WHERE status IN ('bestaetigt', 'storniert')
  AND aufbewahren_bis IS NOT NULL
ORDER BY aufbewahren_bis, id;

-- name: ListRetentionExporte :many
SELECT export.id, export.nutzer_id, export.version,
  COALESCE(abrechnung.abrechnungsnummer, export.abrechnung_id) AS bezeichnung,
  COALESCE(export.aufbewahren_bis, '') AS aufbewahren_bis,
  COALESCE(export.pdf_sha256, '') AS sha256,
  export.inhalt_geloescht_am,
  export.pdf_schluessel, export.zip_schluessel
FROM export
JOIN abrechnung ON abrechnung.id = export.abrechnung_id
WHERE export.status = 'fertig'
ORDER BY export.aufbewahren_bis, export.id;

-- name: ExtendBelegFrist :exec
UPDATE beleg SET
  aufbewahren_bis = sqlc.arg(aufbewahren_bis),
  geaendert_am = sqlc.arg(geaendert_am)
WHERE aufbewahren_bis IS NOT NULL
  AND aufbewahren_bis < sqlc.arg(aufbewahren_bis)
  AND id IN (
    SELECT ausgabe_beleg.beleg_id
    FROM ausgabe_beleg
    JOIN ausgabe ON ausgabe.id = ausgabe_beleg.ausgabe_id
    JOIN abrechnung_reise ON abrechnung_reise.reise_id = ausgabe.reise_id
    WHERE abrechnung_reise.abrechnung_id = sqlc.arg(abrechnung_id)
  );

-- name: MaxBelegFristOfAbrechnung :one
SELECT CAST(COALESCE(MAX(beleg.aufbewahren_bis), '') AS TEXT) AS bis
FROM beleg
JOIN ausgabe_beleg ON ausgabe_beleg.beleg_id = beleg.id
JOIN ausgabe ON ausgabe.id = ausgabe_beleg.ausgabe_id
JOIN abrechnung_reise ON abrechnung_reise.reise_id = ausgabe.reise_id
WHERE abrechnung_reise.abrechnung_id = sqlc.arg(abrechnung_id);

-- name: MarkBelegInhaltGeloescht :exec
UPDATE beleg SET
  inhalt_geloescht_am = sqlc.arg(jetzt),
  loesch_grund = sqlc.arg(grund),
  geaendert_am = sqlc.arg(jetzt),
  version = version + 1
WHERE id = sqlc.arg(id) AND inhalt_geloescht_am IS NULL;

-- name: DeleteBelegdateienByBeleg :exec
DELETE FROM belegdatei WHERE beleg_id = sqlc.arg(beleg_id);

-- name: DeleteBelegtexteByBeleg :exec
DELETE FROM belegtext WHERE beleg_id = sqlc.arg(beleg_id);

-- name: MarkExportInhaltGeloescht :exec
UPDATE export SET
  inhalt_geloescht_am = sqlc.arg(jetzt),
  loesch_grund = sqlc.arg(grund),
  snapshot = '{"geloescht":true}',
  pdf_schluessel = NULL,
  zip_schluessel = NULL
WHERE id = sqlc.arg(id) AND inhalt_geloescht_am IS NULL;

-- name: CountOpenJobsByArt :one
SELECT CAST(COUNT(*) AS BIGINT) AS n
FROM job
WHERE art = sqlc.arg(art) AND status IN ('pending', 'running');

-- name: CountDoneJobsSince :one
SELECT CAST(COUNT(*) AS BIGINT) AS n
FROM job
WHERE art = sqlc.arg(art) AND status = 'done' AND geaendert_am >= sqlc.arg(seit);
