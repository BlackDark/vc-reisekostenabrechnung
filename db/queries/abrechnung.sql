-- name: InsertAbrechnung :one
INSERT INTO abrechnung (
  id, nutzer_id, arbeitgeber_id, zeitraum_art, von, bis, titel, status, export_sprache,
  quittierte_warnungen, erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(arbeitgeber_id), sqlc.arg(zeitraum_art),
  sqlc.arg(von), sqlc.arg(bis), sqlc.arg(titel), 'entwurf', sqlc.arg(export_sprache),
  '[]', sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: GetAbrechnung :one
SELECT * FROM abrechnung
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: GetAbrechnungByID :one
SELECT * FROM abrechnung
WHERE id = sqlc.arg(id);

-- name: ListAbrechnungen :many
SELECT * FROM abrechnung
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND (sqlc.narg(cursor) IS NULL OR id < sqlc.narg(cursor))
ORDER BY id DESC
LIMIT sqlc.arg(limit_n);

-- name: UpdateAbrechnungKopf :one
UPDATE abrechnung SET
  titel = sqlc.arg(titel),
  export_sprache = sqlc.arg(export_sprache),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
  AND status = 'entwurf' AND einreichung_laeuft = FALSE
RETURNING *;

-- name: MarkEinreichung :one
UPDATE abrechnung SET
  abrechnungsnummer = sqlc.arg(abrechnungsnummer),
  einreichung_laeuft = TRUE,
  einreichung_fehler = NULL,
  quittierte_warnungen = sqlc.arg(quittierte_warnungen),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
  AND status = 'entwurf' AND einreichung_laeuft = FALSE
RETURNING *;

-- name: FinishEinreichung :one
UPDATE abrechnung SET
  status = 'eingereicht',
  eingereicht_am = sqlc.arg(eingereicht_am),
  aktuelle_export_version = sqlc.arg(aktuelle_export_version),
  einreichung_laeuft = FALSE,
  einreichung_fehler = NULL,
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND einreichung_laeuft = TRUE
RETURNING *;

-- name: RollbackEinreichung :one
UPDATE abrechnung SET
  einreichung_laeuft = FALSE,
  einreichung_fehler = sqlc.arg(einreichung_fehler),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND einreichung_laeuft = TRUE
RETURNING *;

-- name: MarkEntsperrt :one
UPDATE abrechnung SET
  status = 'entwurf',
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
  AND status = 'eingereicht'
RETURNING *;

-- name: MarkBezahlt :one
UPDATE abrechnung SET
  status = 'bezahlt',
  bezahlt_am = sqlc.arg(bezahlt_am),
  bezahlt_vermerk = sqlc.narg(bezahlt_vermerk),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
  AND status = 'eingereicht'
RETURNING *;

-- name: MarkBezahltZurueck :one
UPDATE abrechnung SET
  status = 'eingereicht',
  bezahlt_am = NULL,
  bezahlt_vermerk = NULL,
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
  AND status = 'bezahlt'
RETURNING *;

-- name: DeleteAbrechnungEntwurf :execrows
DELETE FROM abrechnung
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id)
  AND status = 'entwurf' AND aktuelle_export_version = 0 AND einreichung_laeuft = FALSE;

-- name: InsertAbrechnungReise :exec
INSERT INTO abrechnung_reise (abrechnung_id, reise_id)
VALUES (sqlc.arg(abrechnung_id), sqlc.arg(reise_id));

-- name: DeleteAbrechnungReisen :exec
DELETE FROM abrechnung_reise WHERE abrechnung_id = sqlc.arg(abrechnung_id);

-- name: ListAbrechnungReiseIDs :many
SELECT reise_id FROM abrechnung_reise
WHERE abrechnung_id = sqlc.arg(abrechnung_id)
ORDER BY reise_id;

-- name: ListReisenByArbeitgeberStatus :many
SELECT * FROM reise
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND arbeitgeber_id = sqlc.arg(arbeitgeber_id)
  AND status = sqlc.arg(status)
ORDER BY ende, id;

-- name: SetReiseStatus :exec
UPDATE reise SET
  status = sqlc.arg(status),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: ListVorschuesseByArbeitgeber :many
SELECT * FROM vorschuss
WHERE nutzer_id = sqlc.arg(nutzer_id) AND arbeitgeber_id = sqlc.arg(arbeitgeber_id)
ORDER BY datum, id;

-- name: ReleaseVorschuesse :exec
UPDATE vorschuss SET
  abrechnung_id = NULL,
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE nutzer_id = sqlc.arg(nutzer_id) AND abrechnung_id = sqlc.arg(abrechnung_id);

-- name: ClaimVorschuss :one
UPDATE vorschuss SET
  abrechnung_id = sqlc.arg(abrechnung_id),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND abrechnung_id IS NULL
RETURNING *;

-- name: ListVorschuesseOfAbrechnung :many
SELECT * FROM vorschuss
WHERE nutzer_id = sqlc.arg(nutzer_id) AND abrechnung_id = sqlc.arg(abrechnung_id)
ORDER BY datum, id;

-- name: NextAbrechnungsnummer :one
INSERT INTO abrechnungsnummer_folge (nutzer_id, arbeitgeber_id, jahr, naechste)
VALUES (sqlc.arg(nutzer_id), sqlc.arg(arbeitgeber_id), sqlc.arg(jahr), 1)
ON CONFLICT (nutzer_id, arbeitgeber_id, jahr) DO UPDATE SET
  naechste = abrechnungsnummer_folge.naechste + 1
RETURNING naechste;

-- name: InsertExport :one
INSERT INTO export (
  id, abrechnung_id, nutzer_id, version, anlass, erstellt_am, snapshot, status
) VALUES (
  sqlc.arg(id), sqlc.arg(abrechnung_id), sqlc.arg(nutzer_id), sqlc.arg(version),
  sqlc.arg(anlass), sqlc.arg(erstellt_am), sqlc.arg(snapshot), 'in_erstellung'
)
RETURNING *;

-- name: GetExport :one
SELECT * FROM export
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: GetExportByID :one
SELECT * FROM export
WHERE id = sqlc.arg(id);

-- name: ListExporte :many
SELECT * FROM export
WHERE abrechnung_id = sqlc.arg(abrechnung_id) AND nutzer_id = sqlc.arg(nutzer_id)
ORDER BY version;

-- name: MaxExportVersion :one
SELECT CAST(COALESCE(MAX(version), 0) AS BIGINT) AS version
FROM export
WHERE abrechnung_id = sqlc.arg(abrechnung_id);

-- name: FinishExportRow :one
UPDATE export SET
  status = 'fertig',
  pdf_schluessel = sqlc.arg(pdf_schluessel),
  pdf_sha256 = sqlc.arg(pdf_sha256),
  zip_schluessel = sqlc.arg(zip_schluessel),
  zip_sha256 = sqlc.arg(zip_sha256),
  snapshot = sqlc.arg(snapshot)
WHERE id = sqlc.arg(id) AND status = 'in_erstellung'
RETURNING *;

-- name: MarkExportErsetzt :exec
UPDATE export SET ersetzt_durch_version = sqlc.arg(ersetzt_durch_version)
WHERE abrechnung_id = sqlc.arg(abrechnung_id)
  AND status = 'fertig'
  AND id != sqlc.arg(id)
  AND ersetzt_durch_version IS NULL;

-- name: FailExportRow :exec
UPDATE export SET status = 'fehlgeschlagen', fehler = sqlc.arg(fehler)
WHERE id = sqlc.arg(id) AND status = 'in_erstellung';
