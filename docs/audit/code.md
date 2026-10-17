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
