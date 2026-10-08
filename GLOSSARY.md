# Reisekostenabrechnung

Erfassung beruflicher Reisen mit Belegen und Pauschalen und deren Abrechnung gegenüber einem Arbeitgeber nach deutschem Steuerrecht.

## Beteiligte

**Nutzer**:
Eine Person, die eigene Reisen erfasst und abrechnet. Sieht ausschließlich ihre eigenen Daten.
_Avoid_: Account, Mitarbeiter, User

**Admin**:
Ein Nutzer, der zusätzlich andere Nutzer verwaltet und Satztabellen pflegt.
_Avoid_: Superuser, Owner

**Arbeitgeber**:
Das Unternehmen, das einem Nutzer Reisekosten erstattet und auf das Rechnungen für den Vorsteuerabzug lauten. Ein Nutzer hat in der Regel einen, kann aber mehrere haben.
_Avoid_: Firma, Mandant, Auftraggeber, Kunde

**Vorlage**:
Eine gespeicherte Vorbelegung für wiederkehrende Reisen oder Fahrten, z. B. Ziel, Strecke und Kilometer.
_Avoid_: Favorit, Template

**Projekt**:
Optionale Bezeichnung, der eine Reise zugeordnet wird, z. B. ein Kunde oder eine Kostenstelle.
_Avoid_: Kostenstelle, Kunde, Tag

## Reisen

**Reise**:
Eine beruflich veranlasste Abwesenheit von Wohnung und erster Tätigkeitsstätte, vom Aufbruch bis zur Rückkehr, ein- oder mehrtägig.
_Avoid_: Trip, Dienstgang, Fahrt, Eintrag

**Reisetag**:
Ein Kalendertag innerhalb einer Reise, für den Abwesenheitsdauer, maßgebliches Land und gestellte Mahlzeiten festgehalten sind.
_Avoid_: Tag, Etappe

**Tätigkeitsstätte**:
Ein Ort, an dem während einer Reise beruflich gearbeitet wird.
_Avoid_: Einsatzort, Kundenstandort

**Maßgebliches Land**:
Das Land, dessen Pauschalen für einen Reisetag gelten.
_Avoid_: Reiseland, Zielland

**Fremdwährung**:
Jede Währung außer Euro, in der eine Ausgabe bezahlt wurde; wird mit einem dokumentierten Kurs in Euro umgerechnet.

## Kosten

**Ausgabe**:
Eine einzelne, vom Nutzer selbst bezahlte Aufwendung auf einer Reise mit Betrag, Kostenart und Steueranteilen. Jede Ausgabe gehört zu genau einer Reise.
_Avoid_: Position, Kosten, Eintrag, Spese

**Kostenart**:
Die steuerliche Einordnung einer Ausgabe: Fahrtkosten, Verpflegung, Übernachtung, Reisenebenkosten oder Bewirtung.
_Avoid_: Kategorie, Typ

**Steueranteil**:
Der Teil einer Ausgabe, der einem Umsatzsteuersatz unterliegt, mit Netto-, Steuer- und Bruttobetrag. Eine Ausgabe kann mehrere haben.
_Avoid_: MwSt-Position, Steuerzeile

**Fahrt**:
Eine mit einem privaten Fahrzeug zurückgelegte Strecke, die über die Kilometerpauschale abgerechnet wird.
_Avoid_: Strecke, Kilometereintrag

**Bewirtung**:
Eine Ausgabe für die Verpflegung von Geschäftspartnern, mit Anlass und Teilnehmern.
_Avoid_: Geschäftsessen, Spesen

**Gestellte Mahlzeit**:
Ein Frühstück, Mittag- oder Abendessen an einem Reisetag, das vom Arbeitgeber oder auf dessen Veranlassung gestellt wurde, z. B. im erstatteten Hotelpreis enthalten, vom Arbeitgeber organisiert oder als Ausgabe erstattet.
_Avoid_: Freiessen, Verpflegung

**Pauschale**:
Ein gesetzlich festgelegter Betrag, der ohne Einzelnachweis angesetzt wird: Verpflegungspauschale, Übernachtungspauschale oder Kilometerpauschale.
_Avoid_: Spesensatz, Tagegeld, Faktor

**Mahlzeitenkürzung**:
Die Minderung der Verpflegungspauschale eines Reisetags für Mahlzeiten, die vom Arbeitgeber oder auf dessen Veranlassung gestellt wurden.
_Avoid_: Abzug, Frühstücksabzug

**Satztabelle**:
Alle für ein Kalenderjahr gültigen Pauschalen und Sätze, einschließlich der Auslandspauschalen je Land. Maßgeblich ist das Datum des Reisetags oder der Ausgabe.
_Avoid_: Faktoren, Konfiguration, Stammdaten

## Nachweise

**Beleg**:
Die Nachweisdatei zu einer Ausgabe, z. B. Foto, PDF oder E-Rechnung. Fotos werden als Archivbeleg aufbewahrt, PDFs und E-Rechnungen unverändert im Empfangsformat.
_Avoid_: Quittung, Anhang, Nachweis, Scan

**Archivbeleg**:
Die aufbereitete, nach Bestätigung durch den Nutzer unveränderbar gespeicherte Fassung eines Belegs; sie gilt als das Original.
_Avoid_: Master, Scan, Rohbild

**Eigenbeleg**:
Ein vom Nutzer erstellter Ersatz, wenn für eine Ausgabe kein Beleg vorliegt.
_Avoid_: Ersatzbeleg, Notiz

## Abrechnung

**Abrechnung**:
Eine festgeschriebene Zusammenstellung von Reisen eines Nutzers gegenüber einem Arbeitgeber für einen Abrechnungszeitraum.
_Avoid_: Report, Spesenreport, Export

**Abrechnungszeitraum**:
Der Zeitraum (Tag, Woche, Monat, Quartal oder frei), dessen Reisen eine Abrechnung umfasst. Eine Reise gehört zum Zeitraum ihres Enddatums.
_Avoid_: Periode, Filter

**Vorschuss**:
Ein Betrag, den der Arbeitgeber dem Nutzer vorab gezahlt hat und der in einer Abrechnung vom Erstattungsbetrag abgezogen wird.
_Avoid_: Anzahlung, Abschlag

**Abrechnungsstatus**:
Der Stand einer Abrechnung: Entwurf, Eingereicht oder Bezahlt.
_Avoid_: Phase, Workflow-Status

**Entsperrung**:
Das protokollierte Zurücksetzen einer eingereichten Abrechnung in den Entwurf, um sie zu korrigieren.
_Avoid_: Reopen, Storno

**Export**:
Die aus einer eingereichten Abrechnung erzeugten Dateien, unveränderbar gespeichert.
_Avoid_: Download, Ausdruck
