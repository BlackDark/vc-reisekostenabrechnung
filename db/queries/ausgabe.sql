-- name: InsertAusgabe :one
INSERT INTO ausgabe (
  id, nutzer_id, reise_id, kostenart, datum, leistender, beschreibung, waehrung, betrag,
  kurs, kurs_quelle, kurs_datum, kurs_grund, betrag_eur, rechnungsart, rechnung_auf_arbeitgeber,
  empfaenger, verkehrsmittel, uebernachtung_naechte, fruehstueck_enthalten, mahlzeit, bewirtung,
  tse_beleg, erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(reise_id), sqlc.arg(kostenart), sqlc.arg(datum),
  sqlc.arg(leistender), sqlc.narg(beschreibung), sqlc.arg(waehrung), sqlc.arg(betrag),
  sqlc.narg(kurs), sqlc.narg(kurs_quelle), sqlc.narg(kurs_datum), sqlc.narg(kurs_grund),
  sqlc.arg(betrag_eur), sqlc.arg(rechnungsart), sqlc.arg(rechnung_auf_arbeitgeber),
  sqlc.narg(empfaenger), sqlc.narg(verkehrsmittel), sqlc.arg(uebernachtung_naechte),
  sqlc.arg(fruehstueck_enthalten), sqlc.narg(mahlzeit), sqlc.narg(bewirtung),
  sqlc.arg(tse_beleg), sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: UpdateAusgabe :one
UPDATE ausgabe SET
  kostenart = sqlc.arg(kostenart),
  datum = sqlc.arg(datum),
  leistender = sqlc.arg(leistender),
  beschreibung = sqlc.narg(beschreibung),
  waehrung = sqlc.arg(waehrung),
  betrag = sqlc.arg(betrag),
  kurs = sqlc.narg(kurs),
  kurs_quelle = sqlc.narg(kurs_quelle),
  kurs_datum = sqlc.narg(kurs_datum),
  kurs_grund = sqlc.narg(kurs_grund),
  betrag_eur = sqlc.arg(betrag_eur),
  rechnungsart = sqlc.arg(rechnungsart),
  rechnung_auf_arbeitgeber = sqlc.arg(rechnung_auf_arbeitgeber),
  empfaenger = sqlc.narg(empfaenger),
  verkehrsmittel = sqlc.narg(verkehrsmittel),
  uebernachtung_naechte = sqlc.arg(uebernachtung_naechte),
  fruehstueck_enthalten = sqlc.arg(fruehstueck_enthalten),
  mahlzeit = sqlc.narg(mahlzeit),
  bewirtung = sqlc.narg(bewirtung),
  tse_beleg = sqlc.arg(tse_beleg),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: GetAusgabe :one
SELECT * FROM ausgabe
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: ListAusgabenByReise :many
SELECT * FROM ausgabe
WHERE reise_id = sqlc.arg(reise_id) AND nutzer_id = sqlc.arg(nutzer_id)
ORDER BY datum, id;

-- name: ListAusgabenByNutzer :many
SELECT * FROM ausgabe
WHERE nutzer_id = sqlc.arg(nutzer_id)
ORDER BY datum, id;

-- name: DeleteAusgabe :execrows
DELETE FROM ausgabe
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: SetBewirtung :one
UPDATE ausgabe SET
  bewirtung = sqlc.narg(bewirtung),
  tse_beleg = sqlc.arg(tse_beleg),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: SetRechnungsart :one
UPDATE ausgabe SET
  rechnungsart = sqlc.arg(rechnungsart),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version)
RETURNING *;

-- name: DeleteSteueranteile :exec
DELETE FROM steueranteil
WHERE ausgabe_id = sqlc.arg(ausgabe_id);

-- name: InsertSteueranteil :one
INSERT INTO steueranteil (
  id, ausgabe_id, position, satz, steuerland, netto, steuer, brutto, netto_eur, steuer_eur, brutto_eur
) VALUES (
  sqlc.arg(id), sqlc.arg(ausgabe_id), sqlc.arg(position), sqlc.arg(satz), sqlc.arg(steuerland),
  sqlc.arg(netto), sqlc.arg(steuer), sqlc.arg(brutto), sqlc.arg(netto_eur), sqlc.arg(steuer_eur), sqlc.arg(brutto_eur)
)
RETURNING *;

-- name: ListSteueranteile :many
SELECT * FROM steueranteil
WHERE ausgabe_id = sqlc.arg(ausgabe_id)
ORDER BY position;

-- name: DeleteAusgabeBelege :exec
DELETE FROM ausgabe_beleg
WHERE ausgabe_id = sqlc.arg(ausgabe_id);

-- name: InsertAusgabeBeleg :exec
INSERT INTO ausgabe_beleg (ausgabe_id, beleg_id)
VALUES (sqlc.arg(ausgabe_id), sqlc.arg(beleg_id));

-- name: ListBelegeForAusgabe :many
SELECT beleg.id, beleg.status
FROM ausgabe_beleg
JOIN beleg ON beleg.id = ausgabe_beleg.beleg_id
WHERE ausgabe_beleg.ausgabe_id = sqlc.arg(ausgabe_id) AND beleg.nutzer_id = sqlc.arg(nutzer_id)
ORDER BY beleg.id;

-- name: UpsertEigenbeleg :one
INSERT INTO eigenbeleg (
  ausgabe_id, grund, zahlungsempfaenger, art, erstellt_am, bestaetigt_am, bestaetigt_von
) VALUES (
  sqlc.arg(ausgabe_id), sqlc.arg(grund), sqlc.arg(zahlungsempfaenger), sqlc.arg(art),
  sqlc.arg(erstellt_am), sqlc.narg(bestaetigt_am), sqlc.narg(bestaetigt_von)
)
ON CONFLICT (ausgabe_id) DO UPDATE SET
  grund = excluded.grund,
  zahlungsempfaenger = excluded.zahlungsempfaenger,
  art = excluded.art,
  bestaetigt_am = excluded.bestaetigt_am,
  bestaetigt_von = excluded.bestaetigt_von
RETURNING *;

-- name: GetEigenbeleg :one
SELECT * FROM eigenbeleg
WHERE ausgabe_id = sqlc.arg(ausgabe_id);

-- name: UpsertWechselkurs :exec
INSERT INTO wechselkurs (datum, waehrung, kurs, quelle, abgerufen_am)
VALUES (sqlc.arg(datum), sqlc.arg(waehrung), sqlc.arg(kurs), sqlc.arg(quelle), sqlc.arg(abgerufen_am))
ON CONFLICT (datum, waehrung) DO UPDATE SET
  kurs = excluded.kurs,
  quelle = excluded.quelle,
  abgerufen_am = excluded.abgerufen_am;

-- name: ListWechselkurse :many
SELECT * FROM wechselkurs
WHERE waehrung = sqlc.arg(waehrung)
ORDER BY datum;

-- name: ListWechselkurseAll :many
SELECT * FROM wechselkurs
ORDER BY datum, waehrung;

-- name: UpsertUstMonatskurs :exec
INSERT INTO ust_monatskurs (jahr, monat, waehrung, kurs, abgerufen_am)
VALUES (sqlc.arg(jahr), sqlc.arg(monat), sqlc.arg(waehrung), sqlc.arg(kurs), sqlc.arg(abgerufen_am))
ON CONFLICT (jahr, monat, waehrung) DO UPDATE SET
  kurs = excluded.kurs,
  abgerufen_am = excluded.abgerufen_am;

-- name: GetUstMonatskurs :one
SELECT * FROM ust_monatskurs
WHERE jahr = sqlc.arg(jahr) AND monat = sqlc.arg(monat) AND waehrung = sqlc.arg(waehrung);

-- name: InsertVorschuss :one
INSERT INTO vorschuss (
  id, nutzer_id, arbeitgeber_id, datum, betrag, notiz, erstellt_am, geaendert_am, version
) VALUES (
  sqlc.arg(id), sqlc.arg(nutzer_id), sqlc.arg(arbeitgeber_id), sqlc.arg(datum), sqlc.arg(betrag),
  sqlc.narg(notiz), sqlc.arg(erstellt_am), sqlc.arg(geaendert_am), 1
)
RETURNING *;

-- name: GetVorschuss :one
SELECT * FROM vorschuss
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id);

-- name: ListVorschuesse :many
SELECT * FROM vorschuss
WHERE nutzer_id = sqlc.arg(nutzer_id)
ORDER BY datum, id;

-- name: UpdateVorschuss :one
UPDATE vorschuss SET
  datum = sqlc.arg(datum),
  betrag = sqlc.arg(betrag),
  notiz = sqlc.narg(notiz),
  geaendert_am = sqlc.arg(geaendert_am),
  version = version + 1
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND version = sqlc.arg(version) AND abrechnung_id IS NULL
RETURNING *;

-- name: DeleteVorschussOffen :execrows
DELETE FROM vorschuss
WHERE id = sqlc.arg(id) AND nutzer_id = sqlc.arg(nutzer_id) AND abrechnung_id IS NULL;
