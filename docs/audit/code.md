# Audit: code, 2026-10-17 (main 0604fce)

Scope: hand-written Go (`internal/`, `cmd/`, `scripts/`) and web sources (`web/src/`). Generated
and vendored trees (`internal/api/server.gen.go`, `internal/store/sqlitedb/*`, `web/src/lib/api/schema.ts`,
`internal/webui/dist`, `node_modules`) got size and staleness checks only.

Not repeated here: `docs/ideas/code-health.md` holds the finished code-health pass (ids 1.x, 2.x, 3.x).
Every row below is new or a recurrence that the pass did not cover.

Inventory: 652 tracked files; 106 non-test Go files, 44 test Go files, 53,985 Go lines; 262 svelte/ts
files, 17,694 lines. Largest hand-written Go: `internal/berechnung/berechne.go` 982,
`internal/httpapi/reisen.go` 973, `internal/store/ausgaben.go` 928, `internal/store/reisen.go` 905,
`internal/httpapi/stammdaten.go` 853. Largest web source: `ReiseDetail.svelte` 656,
`AbrechnungDetail.svelte` 389, `ausgabe-editor.svelte` 374.

## Code findings

| ID | Sev | Where | Problem and evidence | Fix | Effort | Benefit |
| --- | --- | --- | --- | --- | --- | --- |
| CODE-4 | M | `internal/httpapi/jobs.go:164-166` / `:224-226`, `:192-210` / `:265-281` | `jobFoto` and `jobPDF` repeat the same final-state guard and the same blob write loop (`blobKey` + `SHA256Hex` + `PutIfAbsent` + append to `parts`). | Extract the guard and the write loop. | M | Halves the maintenance surface of both pipeline jobs. |
| CODE-8 | M | `internal/httpapi/ausgaben.go:533-541` | `lineIndex` swallows both errors (`ListReisenAll`, `calculate`) with `return nil`, so the view silently loses its per-line results instead of failing. | Return the error to the handler. | M | A broken view becomes visible instead of empty. |
| CODE-9 | M | `web/src/routes/ReiseDetail.svelte:107-115` | `loadBelege` fetches `/api/v1/belege?limit=100` and then filters by `beleg_ids` in the browser. An account with more than 100 Belege silently shows no receipt for an Ausgabe. | Ask the server for the ids that are needed. | M | Correct receipts for accounts with more than 100 Belege. |
| CODE-10 | M | `internal/berechnung/berechne.go:358-519` (`assign`, 162 lines), `:647-766` (`convertAusgabe`, 120 lines) | Two of the largest functions mix rule dispatch with field mapping inside a 982-line package. | Split rule selection from conversion. | L | Testable seams in the hot calculation path. |
| CODE-14 | L | `web/src/lib/beleg/corners.ts:3-12` vs `web/src/lib/beleg/geometry.ts:133-142` | `frame(w, h)` is duplicated verbatim because `geometry.ts` keeps its copy unexported. | Export `frame` from `geometry.ts` and import it. | S | One definition of the fallback quad. |

Comment findings (COM-1..5): see AUDIT.md.

## Not established

- `internal/httpapi/snapshot.go:270,283` call `rate24`/`kmRate` per Reisetag/Fahrt after `rateSnapshot`
  (`:439-472`) already loaded the year. Cost of `EffectiveYear` was not measured, so this is not a finding.
- `internal/httpapi/snapshot.go:500-519` `advancesOf` lists all Vorschuesse and filters in Go; whether the
  store can filter was not checked.
- `internal/httpapi/mailpoll.go:24-26` calls the poller "the one part not covered by a durable unit test";
  that matches the code (`pollMailOnce` untested), so it stays.
- `internal/httpapi/abrechnung.go:26-31` per-row `GetAbrechnungBundle` is `docs/ideas/code-health.md` 1.4,
  skipped as an existing item.
