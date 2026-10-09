# Operations

Day-to-day commands. The German handbook with the same procedures is [docs/betrieb.md](betrieb.md).

## Checks

`reisekosten doctor` runs `PRAGMA quick_check` and checks that `DATA_DIR` is writable. The process runs `PRAGMA optimize` at start and then once a day. `wal_autocheckpoint` stays on the SQLite default so Litestream can read the WAL.

## Backup without Litestream

```sh
reisekosten backup --out /backup/reisekosten.tar
```

The archive contains a consistent SQLite snapshot (`VACUUM INTO`) and, for local storage, the files under `STORAGE_LOCAL_PATH` (`reisekosten.db`, `files/…`, `files.txt`).

Restore:

1. Stop the app.
2. Put `reisekosten.db` at `DB_PATH`. Remove a leftover `-wal` or `-shm` next to it.
3. Copy `files/` back to `STORAGE_LOCAL_PATH`.
4. Start the app. `reisekosten doctor` prints `doctor ok`.

With S3 the tar holds the database and `files.txt` only. Version the bucket as well.

## Litestream

`deploy/docker-compose.backup.yml` runs Litestream 0.5.17 as UID 65532, with a read-only root, `cap_drop: ALL`, a tmpfs on `/tmp`, and `HOME=/tmp`. It writes the replica at `LITESTREAM_REPLICA_URL` and a `.reisekosten.db-litestream` directory next to the database. Both are owned by UID 65532, so `/data` must be writable by that user.

The replica URL should be an S3 bucket in the EU/EEA. CI restores a file replica (`scripts/litestream-smoke.sh`). A monthly restore of the real bucket is an operator task.

Restore from S3:

1. Stop the app and Litestream.
2. Move `reisekosten.db`, `-wal`, and `-shm` aside.
3. In the Litestream container, still as UID 65532:

```sh
litestream restore -config /etc/litestream.yml -o /data/reisekosten.db /data/reisekosten.db
```

4. Start the app and run `reisekosten doctor`.
5. Open a known **Abrechnung** (expense claim). Files come from the volume or from S3, not from the Litestream replica.

## Updates

Set `APP_VERSION` to the image tag and run `docker compose up -d`. Migrations run on start and can be repeated. Take a `reisekosten backup` or note a Litestream restore point first. A rollback is an older image on the same volume, as long as no migration in between narrowed the schema.

Images are signed with keyless cosign. The release notes contain the digest and the `cosign verify` command. `:edge` is the latest `main` build, also signed.

## Retention

**Belege** (receipts) and finished exports stay at least until 31 December of year *J* + 8. *J* is the later of the confirmation year and the submission year. Submitting a claim extends a shorter receipt deadline. Nothing in that set is deleted automatically.

A monthly job (`RETENTION_REPORT_ENABLED`, default on) writes `aufbewahrung.bericht` to the audit log. Admins see it under **Aufbewahrung** (retention). Deleting is allowed only after the date, after confirming the **Ablaufhemmung** warning (§ 147 (3) sentence 5 AO, the rule that a deadline does not run while a tax matter is still open), and with a reason. The files go away. The receipt number, SHA-256, time, and reason stay in the log (`aufbewahrung.geloescht`).

Capture JPEGs are removed after `BELEG_ERFASSUNG_KARENZ` (default 720 h). Those are not the archive copy.

For archive objects, a bucket with versioning or Object Lock in compliance mode is a good extra. The app does not require it. `S3_DATA_LOCATION` must be an EU/EEA country unless `S3_ALLOW_NON_EU=true`.
