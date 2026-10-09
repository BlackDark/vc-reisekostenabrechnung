-- name: InsertBeleg :one
INSERT INTO beleg (
  id, nutzer_id, typ, status, seiten, sha256_original, pipeline_version, pipeline_parameter,
  duplikat_von, erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(typ), sqlc.arg(status), sqlc.arg(seiten),
  sqlc.arg(sha256_original), sqlc.arg(pipeline_version), sqlc.arg(pipeline_parameter),
  sqlc.narg(duplikat_von), sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: GetBeleg :one
SELECT * FROM beleg
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: GetBelegByID :one
SELECT * FROM beleg
WHERE id = sqlc.arg(id);

-- name: ListBelege :many
SELECT * FROM beleg
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND (sqlc.arg(status) = '' OR status = sqlc.arg(status))
  AND (
    sqlc.arg(nur_eingang) = 0
    OR status IN ('hochgeladen', 'in_aufbereitung', 'zur_bestaetigung', 'fehlgeschlagen')
  )
  AND (sqlc.arg(cursor) = '' OR id < sqlc.arg(cursor))
ORDER BY id DESC
LIMIT sqlc.arg(limit_n);

-- name: UpdateBelegReady :one
UPDATE beleg SET
  status = 'zur_bestaetigung',
  seiten = sqlc.arg(seiten),
  pipeline_version = sqlc.arg(pipeline_version),
  pipeline_parameter = sqlc.arg(pipeline_parameter),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('hochgeladen', 'in_aufbereitung', 'zur_bestaetigung', 'fehlgeschlagen')
RETURNING *;

-- name: SetBelegFailed :exec
UPDATE beleg SET
  status = 'fehlgeschlagen',
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('hochgeladen', 'in_aufbereitung', 'fehlgeschlagen');

-- name: ConfirmBeleg :one
UPDATE beleg SET
  status = 'bestaetigt',
  belegnummer = sqlc.arg(belegnummer),
  bestaetigt_am = sqlc.arg(bestaetigt_am),
  bestaetigt_von = sqlc.arg(bestaetigt_von),
  aufbewahren_bis = sqlc.arg(aufbewahren_bis),
  erfassung_loeschen_am = sqlc.arg(erfassung_loeschen_am),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id)
  AND status = 'zur_bestaetigung' AND version = sqlc.arg(version)
RETURNING *;

-- name: StornoBeleg :one
UPDATE beleg SET
  status = 'storniert',
  storno_grund = sqlc.arg(storno_grund),
  storniert_am = sqlc.arg(storniert_am),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id)
  AND status = 'bestaetigt' AND version = sqlc.arg(version)
RETURNING *;

-- name: DeleteBelegOffen :execrows
DELETE FROM beleg
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id)
  AND version = sqlc.arg(version)
  AND status IN ('hochgeladen', 'in_aufbereitung', 'zur_bestaetigung', 'fehlgeschlagen');

-- name: TouchBelegReprocess :one
UPDATE beleg SET
  status = 'in_aufbereitung',
  pipeline_parameter = sqlc.arg(pipeline_parameter),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id)
  AND version = sqlc.arg(version)
  AND status IN ('hochgeladen', 'in_aufbereitung', 'zur_bestaetigung', 'fehlgeschlagen')
RETURNING *;

-- name: ClearErfassung :exec
UPDATE beleg SET
  erfassung_loeschen_am = NULL,
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id);

-- name: BumpBelegnummer :one
INSERT INTO beleg_nummer (nutzer_id, jahr, naechste)
VALUES (sqlc.arg(nutzer_id), sqlc.arg(jahr), 1)
ON CONFLICT (nutzer_id, jahr) DO UPDATE SET naechste = beleg_nummer.naechste + 1
RETURNING naechste;

-- name: FindBelegBySHA :one
SELECT * FROM beleg
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND sha256_original = sqlc.arg(sha256)
  AND status != 'storniert'
ORDER BY id
LIMIT 1;

-- name: FindBelegByDateiSHA :one
SELECT beleg.* FROM beleg
JOIN belegdatei ON belegdatei.beleg_id = beleg.id
WHERE beleg.nutzer_id = sqlc.arg(nutzer_id)
  AND belegdatei.sha256 = sqlc.arg(sha256)
  AND belegdatei.variante IN ('archiv', 'original')
  AND beleg.status != 'storniert'
ORDER BY beleg.id
LIMIT 1;

-- name: ListOffeneDuplikate :many
SELECT * FROM beleg
WHERE nutzer_id = sqlc.arg(nutzer_id)
  AND duplikat_von IS NOT NULL
  AND status != 'storniert'
ORDER BY id DESC
LIMIT 100;

-- name: InsertBelegdatei :one
INSERT INTO belegdatei (
  id, beleg_id, variante, seite, speicher_schluessel, mime, bytes, sha256, unveraenderbar
) VALUES (
  sqlc.arg(id), sqlc.arg(beleg_id), sqlc.arg(variante), sqlc.arg(seite),
  sqlc.arg(speicher_schluessel), sqlc.arg(mime), sqlc.arg(bytes), sqlc.arg(sha256),
  sqlc.arg(unveraenderbar)
)
RETURNING *;

-- name: ListBelegdateien :many
SELECT * FROM belegdatei
WHERE beleg_id = sqlc.arg(beleg_id)
ORDER BY variante, seite;

-- name: GetBelegdatei :one
SELECT * FROM belegdatei
WHERE beleg_id = sqlc.arg(beleg_id) AND variante = sqlc.arg(variante) AND seite = sqlc.arg(seite);

-- name: DeleteBelegdateiVariante :exec
DELETE FROM belegdatei
WHERE beleg_id = sqlc.arg(beleg_id) AND variante = sqlc.arg(variante);

-- name: MarkBelegdateienFest :exec
UPDATE belegdatei SET unveraenderbar = TRUE
WHERE beleg_id = sqlc.arg(beleg_id)
  AND variante IN ('archiv', 'original', 'export_jpeg');

-- name: InsertJob :one
INSERT INTO job (
  id, art, status, payload, versuche, naechster_versuch_am, erstellt_am, geaendert_am
) VALUES (
  sqlc.arg(id), sqlc.arg(art), 'pending', sqlc.narg(payload), 0,
  sqlc.arg(naechster_versuch_am), sqlc.arg(erstellt_am), sqlc.arg(geaendert_am)
)
RETURNING *;

-- name: ClaimJob :one
UPDATE job SET
  status = 'running',
  versuche = versuche + 1,
  geaendert_am = sqlc.arg(jetzt)
WHERE id = (
  SELECT id FROM job
  WHERE (status = 'pending' AND (naechster_versuch_am IS NULL OR naechster_versuch_am <= sqlc.arg(jetzt)))
     OR (status = 'running' AND job.geaendert_am <= sqlc.arg(stale))
  ORDER BY id
  LIMIT 1
)
RETURNING *;

-- name: FinishJob :exec
UPDATE job SET
  status = 'done',
  fehler = NULL,
  geaendert_am = sqlc.arg(jetzt)
WHERE id = sqlc.arg(id);

-- name: RescheduleJob :exec
UPDATE job SET
  status = 'pending',
  fehler = sqlc.arg(fehler),
  naechster_versuch_am = sqlc.arg(naechster_versuch_am),
  geaendert_am = sqlc.arg(jetzt)
WHERE id = sqlc.arg(id);

-- name: FailJobRow :exec
UPDATE job SET
  status = 'failed',
  fehler = sqlc.arg(fehler),
  geaendert_am = sqlc.arg(jetzt)
WHERE id = sqlc.arg(id);

-- name: ResumeJobs :exec
UPDATE job SET
  status = 'pending',
  geaendert_am = sqlc.arg(jetzt)
WHERE status = 'running';

-- name: DeleteJobsForBeleg :exec
DELETE FROM job
WHERE payload = sqlc.arg(beleg_id);
