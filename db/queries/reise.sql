-- name: InsertReise :one
INSERT INTO reise (
  id, nutzer_id, arbeitgeber_id, anlass, projekt, beginn, beginn_zone, ende, ende_zone,
  notiz, vorlage_id, status, erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(arbeitgeber_id), sqlc.arg(anlass), sqlc.narg(projekt),
  sqlc.arg(beginn), sqlc.arg(beginn_zone), sqlc.arg(ende), sqlc.arg(ende_zone),
  sqlc.narg(notiz), sqlc.narg(vorlage_id), sqlc.arg(status),
  sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: GetReise :one
SELECT * FROM reise
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: ListReisen :many
SELECT * FROM reise
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND (sqlc.arg(status) = '' OR status = sqlc.arg(status))
  AND (sqlc.arg(arbeitgeber_id) = '' OR arbeitgeber_id = sqlc.arg(arbeitgeber_id))
  AND (sqlc.arg(projekt) = '' OR projekt = sqlc.arg(projekt))
  AND (
    sqlc.arg(q) = ''
    OR anlass LIKE sqlc.arg(q_like)
    OR COALESCE(projekt, '') LIKE sqlc.arg(q_like)
  )
  AND (sqlc.narg(von) IS NULL OR ende >= sqlc.narg(von))
  AND (sqlc.narg(bis) IS NULL OR beginn <= sqlc.narg(bis))
  AND (sqlc.narg(cursor) IS NULL OR id > sqlc.narg(cursor))
ORDER BY id
LIMIT sqlc.arg(limit_n);

-- name: ListReisenAll :many
SELECT * FROM reise
WHERE nutzer_id = sqlc.arg(nutzer_id)
ORDER BY id;

-- name: UpdateReise :one
UPDATE reise SET
  arbeitgeber_id = sqlc.arg(arbeitgeber_id),
  anlass = sqlc.arg(anlass),
  projekt = sqlc.narg(projekt),
  beginn = sqlc.arg(beginn),
  beginn_zone = sqlc.arg(beginn_zone),
  ende = sqlc.arg(ende),
  ende_zone = sqlc.arg(ende_zone),
  notiz = sqlc.narg(notiz),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: TouchReise :one
UPDATE reise SET
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DeleteReise :execrows
DELETE FROM reise
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: InsertOrtswechsel :one
INSERT INTO ortswechsel (
  id, reise_id, reihenfolge, abfahrt, abfahrt_zone, ankunft, ankunft_zone,
  verkehrsmittel, land_iso, satzort, ort, taetigkeitsstaette_id, zwischenlandung_mit_uebernachtung
) VALUES (
  sqlc.arg(id), sqlc.arg(reise_id), sqlc.arg(reihenfolge), sqlc.narg(abfahrt), sqlc.narg(abfahrt_zone),
  sqlc.arg(ankunft), sqlc.arg(ankunft_zone), sqlc.arg(verkehrsmittel), sqlc.arg(land_iso),
  sqlc.arg(satzort), sqlc.arg(ort), sqlc.narg(taetigkeitsstaette_id), sqlc.arg(zwischenlandung_mit_uebernachtung)
)
RETURNING *;

-- name: ListOrtswechsel :many
SELECT * FROM ortswechsel
WHERE reise_id = sqlc.arg(reise_id)
ORDER BY reihenfolge;

-- name: DeleteOrtswechsel :exec
DELETE FROM ortswechsel WHERE reise_id = sqlc.arg(reise_id);

-- name: InsertReisetag :one
INSERT INTO reisetag (
  id, reise_id, datum, land_manuell, satzort_manuell, land_begruendung,
  fruehstueck_gestellt, mittag_gestellt, abend_gestellt,
  zuzahlung_fruehstueck, zuzahlung_mittag, zuzahlung_abend,
  mahlzeit_quelle, unterkunft, verpflegung_ausgeschlossen, ausschluss_grund
) VALUES (
  sqlc.arg(id), sqlc.arg(reise_id), sqlc.arg(datum), sqlc.narg(land_manuell), sqlc.narg(satzort_manuell),
  sqlc.narg(land_begruendung), sqlc.arg(fruehstueck_gestellt), sqlc.arg(mittag_gestellt), sqlc.arg(abend_gestellt),
  sqlc.arg(zuzahlung_fruehstueck), sqlc.arg(zuzahlung_mittag), sqlc.arg(zuzahlung_abend),
  sqlc.arg(mahlzeit_quelle), sqlc.arg(unterkunft), sqlc.arg(verpflegung_ausgeschlossen), sqlc.narg(ausschluss_grund)
)
RETURNING *;

-- name: ListReisetage :many
SELECT * FROM reisetag
WHERE reise_id = sqlc.arg(reise_id)
ORDER BY datum;

-- name: GetReisetag :one
SELECT * FROM reisetag
WHERE reise_id = sqlc.arg(reise_id) AND datum = sqlc.arg(datum);

-- name: UpdateReisetag :one
UPDATE reisetag SET
  land_manuell = sqlc.narg(land_manuell),
  satzort_manuell = sqlc.narg(satzort_manuell),
  land_begruendung = sqlc.narg(land_begruendung),
  fruehstueck_gestellt = sqlc.arg(fruehstueck_gestellt),
  mittag_gestellt = sqlc.arg(mittag_gestellt),
  abend_gestellt = sqlc.arg(abend_gestellt),
  zuzahlung_fruehstueck = sqlc.arg(zuzahlung_fruehstueck),
  zuzahlung_mittag = sqlc.arg(zuzahlung_mittag),
  zuzahlung_abend = sqlc.arg(zuzahlung_abend),
  mahlzeit_quelle = sqlc.arg(mahlzeit_quelle),
  unterkunft = sqlc.arg(unterkunft),
  verpflegung_ausgeschlossen = sqlc.arg(verpflegung_ausgeschlossen),
  ausschluss_grund = sqlc.narg(ausschluss_grund)
WHERE reise_id = sqlc.arg(reise_id) AND datum = sqlc.arg(datum)
RETURNING *;

-- name: UpdateReisetagMeals :one
UPDATE reisetag SET
  fruehstueck_gestellt = sqlc.arg(fruehstueck_gestellt),
  mittag_gestellt = sqlc.arg(mittag_gestellt),
  abend_gestellt = sqlc.arg(abend_gestellt),
  mahlzeit_quelle = sqlc.arg(mahlzeit_quelle)
WHERE reise_id = sqlc.arg(reise_id) AND datum = sqlc.arg(datum)
RETURNING *;

-- name: DeleteReisetag :exec
DELETE FROM reisetag
WHERE reise_id = sqlc.arg(reise_id) AND datum = sqlc.arg(datum);

-- name: InsertFahrt :one
INSERT INTO fahrt (
  id, reise_id, nutzer_id, datum, start_ort, ziel, zweck, fahrzeugart, km, hin_und_zurueck,
  vorlage_id, erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(reise_id), sqlc.arg(nutzer_id), sqlc.arg(datum), sqlc.arg(start_ort),
  sqlc.arg(ziel), sqlc.narg(zweck), sqlc.arg(fahrzeugart), sqlc.arg(km), sqlc.arg(hin_und_zurueck),
  sqlc.narg(vorlage_id), sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: ListFahrten :many
SELECT * FROM fahrt
WHERE reise_id = sqlc.arg(reise_id) AND nutzer_id = sqlc.arg(nutzer_id)
ORDER BY datum, id;

-- name: ListFahrtenByNutzer :many
SELECT * FROM fahrt
WHERE nutzer_id = sqlc.arg(nutzer_id)
ORDER BY id;

-- name: GetFahrt :one
SELECT * FROM fahrt
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: UpdateFahrt :one
UPDATE fahrt SET
  datum = sqlc.arg(datum),
  start_ort = sqlc.arg(start_ort),
  ziel = sqlc.arg(ziel),
  zweck = sqlc.narg(zweck),
  fahrzeugart = sqlc.arg(fahrzeugart),
  km = sqlc.arg(km),
  hin_und_zurueck = sqlc.arg(hin_und_zurueck),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DeleteFahrt :execrows
DELETE FROM fahrt
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: InsertVorlage :one
INSERT INTO vorlage (
  id, nutzer_id, name, art, daten, erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(name), sqlc.arg(art), sqlc.arg(daten),
  sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: GetVorlage :one
SELECT * FROM vorlage
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: ListVorlagen :many
SELECT * FROM vorlage
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND (sqlc.arg(art) = '' OR art = sqlc.arg(art))
  AND (sqlc.narg(cursor) IS NULL OR id > sqlc.narg(cursor))
ORDER BY id
LIMIT sqlc.arg(limit_n);

-- name: UpdateVorlage :one
UPDATE vorlage SET
  name = sqlc.arg(name),
  daten = sqlc.arg(daten),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DeleteVorlage :execrows
DELETE FROM vorlage
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: ListProjekte :many
SELECT projekt, MAX(beginn) AS zuletzt
FROM reise
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND projekt IS NOT NULL
  AND projekt != ''
  AND (sqlc.arg(q) = '' OR projekt LIKE sqlc.arg(q_like))
GROUP BY projekt
ORDER BY MAX(beginn) DESC
LIMIT sqlc.arg(limit_n);
