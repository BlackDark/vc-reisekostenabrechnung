# Betrieb

Betriebshandbuch für Backup, Wiederherstellung, Updates und Aufbewahrung.

## Täglicher Betrieb

`reisekosten doctor` prüft `PRAGMA quick_check` und ob `DATA_DIR` beschreibbar ist. Der Prozess führt `PRAGMA optimize` beim Start und danach täglich aus. `wal_autocheckpoint` bleibt auf dem SQLite-Default, damit Litestream die WAL-Datei lesen kann.

## Backup ohne Litestream

`reisekosten backup --out /backup/reisekosten.tar` schreibt einen konsistenten SQLite-Snapshot (`VACUUM INTO`) und, bei lokalem Speicher, die Dateien unter `STORAGE_LOCAL_PATH` in ein Tar-Archiv (`reisekosten.db`, `files/…`, `files.txt`).

Wiederherstellen:

1. App stoppen.
2. `reisekosten.db` aus dem Tar nach `DB_PATH` legen. Eine vorhandene `-wal`/`-shm` daneben löschen.
3. `files/` zurück nach `STORAGE_LOCAL_PATH` kopieren.
4. App starten. `reisekosten doctor` muss `doctor ok` ausgeben.

Bei S3 liegen die Dateien im Bucket. Das Tar enthält dann nur die Datenbank und `files.txt` mit den Schlüsseln. Den Bucket zusätzlich versionieren.

## Litestream

`deploy/docker-compose.backup.yml` startet Litestream 0.5.17 (`litestream/litestream:0.5.17-scratch`) als UID **65532**, wie die App. Root-Dateisystem nur lesend, `cap_drop: ALL`, `tmpfs` auf `/tmp`, `HOME=/tmp`. Ohne `HOME` hat das scratch-Image kein Home-Verzeichnis; `/tmp` ist der beschreibbare Arbeitsort. Die Datenbankdatei und die Replik muss UID 65532 schreiben können, weil die App dieselbe UID benutzt.

```sh
docker compose -f docker-compose.yml -f docker-compose.backup.yml up -d
```

Pflichtvariablen (sonst bricht Compose mit `fehlt` ab): `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY`, `LITESTREAM_REPLICA_URL` (`s3://bucket/pfad`, Bucket in der EU/EWR).

Der CI-Job `backup` stellt eine Datei-Replik mit UID 65532 wieder her (`scripts/litestream-smoke.sh`). Das ist die Probe für O16. Ein monatlicher Restore gegen den echten S3-Bucket bleibt eine Aufgabe der betreibenden Stelle und ist nicht Teil von v1-CI.

### Restore aus der S3-Replik

1. App und Litestream stoppen.
2. Vorhandene `reisekosten.db`, `-wal` und `-shm` beiseitelegen.
3. Im Litestream-Container, weiterhin als UID 65532:

```sh
litestream restore -config /etc/litestream.yml -o /data/reisekosten.db /data/reisekosten.db
```

4. App starten und `reisekosten doctor` ausführen.
5. Stichprobe: eine bekannte Abrechnung öffnen. Dateien kommen aus dem Volume oder aus S3, nicht aus der Litestream-Replik.

## Updates

Image-Tag in `APP_VERSION` setzen, `docker compose up -d`. Migrationen laufen beim Start und sind wiederholbar. Vor dem Update einen `reisekosten backup` oder einen Litestream-Restore-Punkt festhalten. Ein Rollback ist ein älteres Image auf demselben Volume, solange keine Migration dazwischen das Schema verengt hat. Im Zweifel die Datenbank aus dem Backup zurückspielen.

## S3 Object Lock

Für Archivdateien ist ein Bucket mit Versionierung oder Object Lock im Compliance-Modus sinnvoll. Die App verlangt das nicht. `PutIfAbsent` schreibt ein Objekt nur, wenn der Schlüssel frei ist. Object Lock verhindert zusätzlich, dass ein Admin das Objekt vor Fristablauf im Bucket löscht. Aufbewahrungsdatum des Buckets und `aufbewahren_bis` in der App sollten zusammenpassen. Region: EU/EWR (`S3_DATA_LOCATION`).

## Aufbewahrung

Belege und fertige Exporte bleiben mindestens bis zum 31.12. des Jahres *J* + 8, wobei *J* das spätere Jahr aus Bestätigung und Einreichung ist. Einreichen verlängert eine kürzere Belegfrist auf dieses Datum.

Es gibt keine automatische Löschung dieser Dateien. Ein monatlicher Job (`RETENTION_REPORT_ENABLED`, Default an) schreibt `aufbewahrung.bericht` ins Protokoll. Admins sehen den Bericht unter **Aufbewahrung**. Löschen geht nur nach dem Fristende, mit bestätigtem Hinweis auf die Ablaufhemmung (§ 147 Abs. 3 Satz 5 AO) und einem Grund. Die Dateien verschwinden; Belegnummer, SHA-256, Zeitpunkt und Grund bleiben im Protokoll (`aufbewahrung.geloescht`).

Erfassungs-JPEGs werden nach `BELEG_ERFASSUNG_KARENZ` (Default 720 h) automatisch gelöscht. Das ist kein Archivbeleg.
