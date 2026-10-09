-- name: GetSatztabelle :one
SELECT * FROM satztabelle WHERE jahr = sqlc.arg(jahr);

-- name: ListSatztabellen :many
SELECT * FROM satztabelle ORDER BY jahr;

-- name: InsertSatztabelle :one
INSERT INTO satztabelle (
  jahr, status, quelle,
  vma_inland_24h, vma_inland_8h, kuerzung_fruehstueck_pct, kuerzung_hauptmahlzeit_pct,
  uebernachtung_inland_pauschale, km_kraftwagen, km_anderes_motorfahrzeug,
  sachbezug_fruehstueck, sachbezug_hauptmahlzeit, uebliche_mahlzeit_grenze,
  kleinbetragsgrenze, bewirtung_abzug_pct, ust_saetze, aufbewahrung_jahre,
  ersatzlaender, flug_zwischentage_land, schiff_land, erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(jahr), sqlc.arg(status), sqlc.arg(quelle),
  sqlc.arg(vma_inland_24h), sqlc.arg(vma_inland_8h),
  sqlc.arg(kuerzung_fruehstueck_pct), sqlc.arg(kuerzung_hauptmahlzeit_pct),
  sqlc.arg(uebernachtung_inland_pauschale), sqlc.arg(km_kraftwagen), sqlc.arg(km_anderes_motorfahrzeug),
  sqlc.arg(sachbezug_fruehstueck), sqlc.arg(sachbezug_hauptmahlzeit), sqlc.arg(uebliche_mahlzeit_grenze),
  sqlc.arg(kleinbetragsgrenze), sqlc.arg(bewirtung_abzug_pct), sqlc.arg(ust_saetze),
  sqlc.arg(aufbewahrung_jahre), sqlc.arg(ersatzlaender), sqlc.arg(flug_zwischentage_land),
  sqlc.arg(schiff_land), sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: TouchSatztabelle :one
UPDATE satztabelle SET
  status = sqlc.arg(status),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE jahr = sqlc.arg(jahr) AND version = sqlc.arg(version)
RETURNING *;

-- name: InsertAuslandssatz :exec
INSERT INTO auslandssatz (
  id, jahr, land_iso, land_name_de, satzort, ort_name, vma_24h, vma_8h, uebernachtung
) VALUES (
  sqlc.arg(id), sqlc.arg(jahr), sqlc.arg(land_iso), sqlc.arg(land_name_de),
  sqlc.arg(satzort), sqlc.arg(ort_name), sqlc.arg(vma_24h), sqlc.arg(vma_8h), sqlc.arg(uebernachtung)
);

-- name: DeleteAuslandssaetze :exec
DELETE FROM auslandssatz WHERE jahr = sqlc.arg(jahr);

-- name: ListAuslandssaetze :many
SELECT * FROM auslandssatz WHERE jahr = sqlc.arg(jahr) ORDER BY land_name_de, satzort, id;

-- name: ListLaender :many
SELECT DISTINCT land_iso, land_name_de FROM auslandssatz
WHERE jahr = sqlc.arg(jahr)
ORDER BY land_name_de, land_iso;

-- name: InsertSatzOverride :one
INSERT INTO satz_override (
  id, jahr, land_iso, satzort, feld, alter_wert, neuer_wert, grund, admin_id, zeitpunkt
) VALUES (
  sqlc.arg(id), sqlc.arg(jahr), sqlc.narg(land_iso), sqlc.narg(satzort), sqlc.arg(feld),
  sqlc.arg(alter_wert), sqlc.arg(neuer_wert), sqlc.arg(grund), sqlc.arg(admin_id), sqlc.arg(zeitpunkt)
)
RETURNING *;

-- name: ListSatzOverrides :many
SELECT * FROM satz_override WHERE jahr = sqlc.arg(jahr) ORDER BY zeitpunkt, id;

-- name: DeleteSatzOverrides :exec
DELETE FROM satz_override WHERE jahr = sqlc.arg(jahr);
