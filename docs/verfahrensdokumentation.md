# Verfahrensdokumentation (Vorlage)

Vorlage für das ersetzende Scannen nach GoBD. Die konkrete Stelle füllt die Namen aus.

## Wer erfasst

Der angemeldete Nutzer fotografiert oder lädt den Beleg in der installierten PWA. Das Rohbild der Kamera bleibt auf dem Gerät.

## Wann

Vor oder nach der Ausgabe. Die Bestätigung (Sichtkontrolle) vergibt die Belegnummer `JJJJ-NNNN` je Nutzer und Jahr (Europe/Berlin).

## Pipeline

Version `2026.2` (`internal/belegpipe`).

1. Browser: Ecken erkennen, Nutzer korrigiert, entzerren, Profil Bon (945 px) oder A4 (2480 px), JPEG-Qualität 90.
2. Server: Hintergrund normalisieren (Closing etwa 1/25 der Breite, Weichzeichnen, Division, Streckung 2–98 %), danach Median 3×3 auf den Farbversatz je Pixel; Farbversätze über 16 (roter Minusbetrag, blauer Stempel) bleiben unverändert.
3. Archivbeleg: Farbe, AVIF, Qualität und Geschwindigkeit aus `BELEG_AVIF_QUALITY` (70) und `BELEG_AVIF_SPEED` (8). Alternative `BELEG_FORMAT=webp`.
4. Ableitungen je Seite: Vorschau WebP, längste Kante 320 px; Anzeigebild `bild` WebP, 1600 px breit (Bon behält seine 945 px), Qualität 85; Export-JPEG Qualität `BELEG_JPEG_QUALITY` (70).
5. Belege aus Pipeline `2026.1` haben kein Anzeigebild. `GET /api/v1/belege/{id}/bild` liefert für sie den Archivbeleg; nach einer Neuaufbereitung entsteht das Anzeigebild.
6. PDF und XML werden unverändert als Empfangsformat gespeichert; von einer PDF-Seite werden Vorschau, Anzeigebild und Export-JPEG wie bei einem Foto erzeugt.

Ecken, Profil, Codec und Qualität stehen in `pipeline_parameter`. SHA-256 am Archivbeleg. Nach der Bestätigung ist die Datei unveränderbar.

## Fehler

Schlägt die Aufbereitung fehl, bleibt der Beleg `fehlgeschlagen` und kann neu aufbereitet oder gelöscht werden. Die Erfassungs-JPEG wird nach der Bestätigung für `BELEG_ERFASSUNG_KARENZ` (720 h) behalten und dann gelöscht. Das Löschen wird protokolliert.

## Aufbewahrung

`aufbewahren_bis` ist der 31. Dezember des achten Jahres nach dem späteren Jahr aus Bestätigung und Einreichung. Speicherort: lokales Volume oder S3 in der EU/EWR (`S3_DATA_LOCATION`).

Eine Löschung vor diesem Datum lehnt der Server ab. Danach löscht ein Admin die Dateien nur nach dem Hinweis auf die Ablaufhemmung (§ 147 Abs. 3 Satz 5 AO). Belegnummer, SHA-256, Zeitpunkt und Grund bleiben im Protokoll. Die Erfassungs-JPEG wird nach `BELEG_ERFASSUNG_KARENZ` automatisch entfernt; Archivbeleg und Original nicht.
