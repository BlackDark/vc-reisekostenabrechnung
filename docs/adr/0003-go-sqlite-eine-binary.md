# Go-Backend mit SQLite in einer einzigen Binary

Backend in Go, Daten in SQLite (WAL, cgo-frei über `modernc.org/sqlite`), die gebaute SPA ist in dieselbe Binary eingebettet: ein Image, ein Port, Backup als Datei (optional Litestream nach S3). Rust wurde verworfen, weil die App nicht rechenlastig ist und Go einfacher zu pflegen ist; Postgres wurde für den Start verworfen, weil ein zweiter Container für eine Handvoll Nutzer keinen Nutzen bringt. Damit ein Wechsel zu einer externen Datenbank möglich bleibt, bleiben Schema und Abfragen (`sqlc`) im gemeinsamen SQL-Subset von SQLite und Postgres; ein Postgres-Engine-Block wird erst ergänzt, wenn er gebraucht wird.

## Consequences

- Kein `BETWEEN` mit `sqlc.arg()` unter SQLite (sqlc-Issue #3974), stattdessen `>=`/`<=`.
- Horizontale Skalierung über mehrere Instanzen ist ausgeschlossen, solange SQLite genutzt wird.
