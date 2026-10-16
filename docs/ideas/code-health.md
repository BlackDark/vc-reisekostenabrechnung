# Idea: code health findings

What a full pass over the repository turned up, ordered by benefit. Every claim below was
verified against the source (and the test suite) rather than inferred, so each line carries
the file and line that proves it. Nothing here is committed work; this is a candidate list.

Method: read every non-generated Go package, `web/src`, all four workflows, `Dockerfile`,
`deploy/`, `scripts/`, and the docs; then check each suspected problem with `rg` for real
callers and with `go test ./...` for a green baseline. Claims that did not survive that check
are listed under [Checked and rejected](#checked-and-rejected), with the reason, so the same
ground is not re-covered later.

How to read the tiers:

- **Tier 1** changes behaviour or wastes a large share of a request. Do these first.
- **Tier 2** removes real duplication or dead weight. Worth a slice each.
- **Tier 3** documentation and tooling drift. Cheap, but only after the code is settled.
- **Rejected** looked like a finding and is not. Do not re-open without new evidence.

Baseline when this was written: `go build ./...`, `go vet ./...` and `go test ./...` all pass.

Landed since, newest first. The item bodies below are left as they were found, so the
measurement that justified each one is still readable:

- **3.10** the i18n check compares flattened keys and the placeholders of each key
  ([#56](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/56)).
- **3.8** the unused end-to-end Compose file and four forwarding scripts removed
  ([#56](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/56)).
- **3.6** every copy of a tool version is checked, not one file per tool
  ([#56](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/56)).
- **3.3** the backup job runs on the files it reads instead of on the image filter
  ([#56](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/56)).
- **3.2** the toolchain filter lists what the parity check reads; the overlap half is
  decided against, see [Checked and rejected](#checked-and-rejected)
  ([#56](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/56)).
- **3.11** the `make dev` claim, the `make check` claim, the IMAP password file form and
  the job named in the parity script's header
  ([#56](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/56)).
- **2.1** one helper answers a stale write version, including the ordering the handlers
  used to decide between themselves
  ([#55](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/55)).
- **2.4** the stored audit row is mapped to its response in one place
  ([#55](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/55)).
- **2.3** the helpers named for what they do
  ([#55](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/55)).
- **1.10** the rate-limiter map sheds the keys whose bucket has refilled, so it stops
  growing for the life of the process ([#53](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/53)).
- **2.2** one path-traversal guard, in `internal/files`, called by the directory store and
  held there by the storage contract test ([#53](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/53)).
- **2.5** six unused UI component trees, one unused dependency and four dead re-exports
  removed ([#53](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/53)).
- **2.6** seven exported declarations with no caller removed, among them the export
  snapshot's rate list that nothing ever filled in ([#53](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/53)).
- **1.5** the loaders that only read moved to the read pool, so a trip read no longer waits
  behind an upload or an export, and the read pool refuses writes
  ([#52](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/52)).
- **1.12** the VAT month rate is asked once per currency and month, the stored daily rates are
  read per currency in the calculation and only when a line is in one
  ([#51](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/51),
  [#53](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/53)).
- **1.7** the audit chain is verified in pages of 500 rows, so memory no longer scales with
  the length of the history ([#50](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/50)).
- **1.9** the meal recalculation runs inside the expense write, so a failed recalculation
  rolls the expense back instead of leaving the meal flags describing a row that changed
  ([#49](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/49)).
- **1.8** the error-safety half: the external login path propagates the writes it makes and
  refuses a login it could not record
  ([#48](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/48)).
- **1.11** one typst run per batch of receipt pages, 306ms per page down to 175ms on a
  ten-page receipt ([#46](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/46)).
- **1.4** batched bundle load: a page of trips went from four statements plus three per
  expense to seven statements per batch
  ([#45](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/45)).
- **1.3** the whole-history load became about seven times faster as a side effect of the
  batched loader. The recomputation it still does per request is measured and not worth a
  cache: see [Checked and rejected](#checked-and-rejected).
- **1.1** `deploy/.env.example` now states the quality the code and the docs use
  ([#44](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/44)).
- **1.2** one pagination helper that fetches `limit + 1`, trims and returns the cursor
  ([#42](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/42)).
- The indexes the pass itself asked for, in `db/migrations/00015_read_indexes.sql`
  ([#43](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/43)).

## Tier 1: correctness and per-request cost

### 1.1 The deploy example contradicts the code and the docs on receipt quality

`deploy/.env.example:66-67` ships `BELEG_AVIF_QUALITY=40` and `BELEG_AVIF_SPEED=6`. The code
defaults, the documentation and the e2e setup all disagree:

| Source | Quality | Speed |
| --- | --- | --- |
| `internal/config/config.go:166-167` | 70 | 8 |
| `internal/belegpipe/pipe.go:63-67` (the zero-value fallback) | 70 | 8 |
| `docs/configuration.md:71-72` | 70 | 8 |
| `scripts/e2e-up.sh:23` | – | 8 |
| `deploy/.env.example:66-67` | **40** | **6** |

`docs/configuration.md:72` even explains why 8 is the right speed ("8 encodes all-intra about
four times faster than 6"), so the shipped example selects the slower and lower-quality
profile the docs argue against. Anyone who follows the README quickstart gets it.

Fix: make the example agree with the defaults that the code and the docs already state.

Checked and **not** a defect: the empty `HEADER_AUTH_ADMIN_GROUP` and `HEADER_AUTH_ALLOWED_GROUP`
in the same file look like they override the documented `${OIDC_ADMIN_GROUP}` fallback. They do
not. `internal/config/config.go:231` reads them with `lookupDefault`, which substitutes the
OIDC group when the value is empty, so the empty lines already produce the documented
behaviour.

### 1.2 Three pagination rules, and the newest one cannot report a next page

`internal/httpapi/stammdaten.go:776` has the canonical `limitOf` (nil, ≤0 or >200 → 50). Two
handlers re-implement the same rule (`internal/httpapi/handlers.go:340-343`,
`internal/httpapi/abrechnung.go:17-20`), and `abrechnung.go:17` leaves `limit == 0` when the
request carries no `limit`, where `limitOf` would have produced 50.

The "is there another page" check then exists in three forms:

- fetch `limit + 1` and trim (`handlers.go:344` with the trim at `:352`, plus `reisen.go:25`
  and `stammdaten.go:26`), which is correct;
- guess `len(rows) == limit` (`abrechnung.go:36`, `belege.go:56`), which reports "no more
  pages" exactly when the last page is full, so the final page looks empty to a client that
  pages to the end.

Fix: one pagination helper that fetches `limit + 1`, trims, and returns the cursor. This is a
behaviour fix, not only a de-duplication, so it needs the e2e suite and the abort boundary
cases behind it.

### 1.3 Reading one expense reloads the user's entire trip history

`internal/httpapi/ausgaben.go:495-510` (`lineIndex`) calls `ListReisenAll` and then
`a.calculate` over every trip the user has. It is reached from seven endpoints, including the
single-expense read and write paths at `ausgaben.go:26, 52, 64, 89, 129, 151, 167`.

The whole-history load itself is required: SPEC 4.5 ("Mehrere Reisen am selben Kalendertag")
needs every trip of the user that touches a calendar day, across all employers, so a single
expense cannot be evaluated from its own row. The cost is that `ListReisenAll` loads every
trip the user has ever filed and recomputes the whole set on every request, with no cache and
no reuse. Fix by caching the day index per user and invalidating it when a trip or expense of
that user changes; that keeps the semantics and removes the recomputation. If the cache is
still too coarse, note that the rule only needs the trips touching the same calendar day, so
the index can be keyed by day rather than by user.

### 1.4 The bundle fan-out is O(trips × expenses) statements

`internal/store/reisen.go:575-593` (`loadParts`) issues four queries per trip
(`ListOrtswechsel`, `ListReisetage`, `ListFahrten`, `ListAusgabenByReise` via
`loadAusgabeBundles`), then `internal/store/ausgaben.go:808-847` issues three more per
expense (`ListSteueranteile`, `ListBelegeForAusgabe`, `GetEigenbeleg`, the last two inside
`loadAusgabe` at `:824-846`). So one trip costs `4 + 3 × expenses` statements: at the default
page of 50 trips with eight expenses each that is 50 × (4 + 3 × 8) = 1,400 statements for one
list request. The batched pattern already exists in the codebase (`ListBelegMailByIDs`, used
at `internal/httpapi/belege.go:50`), so the per-row loops can become a constant number of
`IN (...)` queries. The same shape repeats at `abrechnung.go:102-108` (one bundle load per
list row), `stammdaten.go:330-334` (one query per Satztabelle year just to count),
`snapshot.go:36-42` (one bundle per trip in a claim) and `exportjob.go:91-98`.

### 1.5 Read-only loads run on the single-connection write pool

`internal/store/store.go:37-48` opens a write pool with `MaxOpenConns(1)` and a read pool with
8, then every `s.tx` user takes the one write connection for a full `BEGIN`/`COMMIT`. There are
78 `s.tx` call sites across 78 methods in `internal/store`, against 57 `s.readQ()` call sites
across 51 methods. Read-only loaders on the write pool include `GetReiseBundle`
(`reisen.go:182`), `ListReisen` (`reisen.go:193`), `ListReisenAll` (`reisen.go:216`),
`ListFahrten` (`reisen.go:418`), `GetAbrechnungBundle` (`abrechnung.go:104`) and
`EffectiveYear` (`stammdaten.go:94`), the last of which is called from `calculate()`, i.e.
from every read path in 1.3.

Consequence: concurrent reads of trips are serialized behind each other and behind in-flight
writes. Both pools get the same DSN (`internal/store/store.go:65-67`), so `foreign_keys`,
`busy_timeout` and WAL apply to the read pool too; moving the read-only loaders onto a read
transaction is therefore a mechanical change, but it spans 78 methods and should be done one
store file at a time behind the existing store tests.

### 1.6 Missing indexes on the hottest filter and sort columns

Verified against `db/migrations/*.sql`, none of these columns has a usable index today:

- `ListWechselkurse`, `WHERE waehrung = ?` (`db/queries/ausgabe.sql:142`). The table has
  only `PRIMARY KEY (datum, waehrung)` (`db/migrations/00005_ausgaben.sql:74-81`), so no
  index prefix matches the filter. Needs `wechselkurs (waehrung, datum)`.
- `ListAuditForNutzer`, `WHERE akteur_nutzer_id = ?` (`db/queries/audit.sql:27`), and both
  audit lists, which sort `ORDER BY zeitpunkt DESC, id DESC` (`db/queries/audit.sql:22,30`).
  Needs `audit_ereignis (akteur_nutzer_id, zeitpunkt, id)` and `(zeitpunkt, id)`; the table
  has only `(objekt_typ, objekt_id)` and `(nr)`.
- `ListRetentionBelege`, `ORDER BY aufbewahren_bis` (`db/queries/aufbewahrung.sql`). Needs
  `beleg (aufbewahren_bis, id)`. Every existing `beleg` index leads with `nutzer_id`, which
  this query does not filter on, so it cannot use any of them.
- `ListRetentionExporte`, `WHERE status='fertig' ORDER BY aufbewahren_bis` (same file).
  Needs `export (status, aufbewahren_bis, id)`; only `(abrechnung_id, version)` exists.
- `DeleteJobsForBeleg`, `WHERE payload = ?` (`db/queries/beleg.sql`), and `LatestJob`,
  `WHERE art = ? AND payload = ?` (`db/queries/belegtext.sql`). Need `job (payload)` and
  `job (art, payload)`; only `(status, naechster_versuch_am)` exists.

Each is a separate migration. They are safe (additive, index-only) but they grow the DB file
and slow writes, so add them with the queries they serve, not in one bulk migration.

### 1.7 `ListAudit` has no limit and the audit table grows once per request

`db/queries/audit.sql` `ListAudit` is `SELECT * FROM audit_ereignis ORDER BY nr ASC` with no
`LIMIT`. It backs `internal/store/store.go:464` (`AuditAll`) and `VerifyAudit`, called from
`cmd/reisekosten/main.go:228`, so the verification cost grows linearly with the whole audit
history. 1.8 is what makes that history grow fast. A page limit plus a range on `nr`, or a
`COUNT(*)` guard, bounds it.

### 1.8 External login does five to six statements, three of them writes, per request

`internal/httpapi/server.go:247` (`headerPrincipal`) calls `resolveExternal`
(`internal/httpapi/identity.go:34-50`) on every request: `GetIdentitaet`, `GetNutzer`,
`TouchIdentitaet` (UPDATE), `syncAdmin` → `SyncGroupAdmin` (write, `identity.go:120`),
`GetNutzer` again (a duplicate of the first read, `identity.go:48`), and `TouchLogin`, which
is `TouchAnmeldung` plus an `appendAudit` INSERT (`internal/store/store.go:315-328`).
Session-based requests do at least one `GetNutzer` per request (`server.go:284`).

So under header or OIDC auth every HTTP request writes an audit row, which is what turns 1.7
into a growth problem. Fix by caching the resolved principal for the request lifetime and
only writing the audit row when the identity actually changed, keeping the `Aktiv` check on
every request.

Two related error-safety defects in the same path, both worth fixing with 1.8:

- `identity.go:48` and `:66` do `n, _ = a.store.GetNutzer(ctx, n.ID)`. A failed re-read
  returns the zero-value `Nutzer`, which is then returned as the authenticated principal.
  The error must be propagated.
- `identity.go:43, 46, 49, 67, 111, 120` and `handlers.go:81, 83, 100, 106` discard the
  error of `TouchLogin`, `TouchIdentitaet`, `SyncGroupAdmin`, `RecordFailure` and
  `RecordAudit`. These are the GoBD-relevant audit writes; a dropped error means a lost
  login event with no trace.

### 1.9 A failed recalculation is written after a successful expense change

`internal/httpapi/ausgaben.go:51, 88, 108` run `_ = a.store.ApplyMealSuggestions(...)` after
the write has already succeeded: `PostAusgabe` (`:34`), `PatchAusgabe` (`:67`) and
`DeleteAusgabe` (`:92`). The client gets 201/200/204 while the derived `MahlzeitQuelle` state
silently diverges from the expense. Make the recalculation part of the same transaction, or
surface the failure.

### 1.10 The rate-limiter map never evicts

`internal/httpapi/limit.go:12-27` keeps one `*rate.Limiter` per key forever. `allowAPI`
(`internal/httpapi/server.go:150-168`) keys by client IP for every `/api/` request, falling
back to the authenticated user ID when there is one (`:159-161`), so an instance reachable
from the internet accumulates one limiter per distinct source IP, plus one per username at
`handlers.go:58`. Add a periodic sweep of idle entries.

### 1.11 One typst process per PDF page, with a full PDF write each time

`internal/httpapi/jobs.go:255` loops over pages and calls `internal/belegpipe/pdf.go:14`
(`RenderPDFPage`), which creates a temp dir (`:18`), writes the whole PDF, writes a `.typ`
file, spawns `typst compile` with a 60 s timeout (`:37`) and reads the result back. A
ten-page receipt costs ten temp dirs, ten full-PDF writes and ten process spawns. Batching
the pages into one typst run, or at minimum reusing one temp dir, removes the spawn and the
repeated copy.

### 1.12 Unbounded scans and un-memoised lookups inside the calculation

- `internal/httpapi/ausgaben.go:450` (`a.store.AllRates`) scans every row of `wechselkurs`
  with no `WHERE` and no `LIMIT` (`db/queries/ausgabe.sql:146`), and it runs on every
  `calculate()`, i.e. on every endpoint from 1.3.
- `internal/httpapi/ausgaben.go:457` then calls `fx.EmbeddedECB()`, which re-parses the
  embedded `ecb.csv` on every call (`internal/fx/fx.go:65`).
- `internal/httpapi/ausgaben.go:449-487` (`attachMoney`) calls `MonthRate`
  (`internal/store/ausgaben.go:456`) per expense line with no memoization; on a cache miss
  that calls `fx.BMF` (`internal/fx/fx.go:83`), which re-parses the embedded `bmf.csv` on
  every call (`internal/fx/fx.go:74-75`).
- `internal/store/ausgaben.go:793-800` (`checkBelege`) issues one `GetBeleg` per id instead
  of an `IN` list, although `ListBelegMailByIDs` shows the batched form
  (`internal/httpapi/belege.go:50`).

## Tier 2: duplication and dead weight

### 2.1 Error to HTTP mapping is hand-rolled in more than 200 places

`internal/httpapi/reisen.go:823` (`writeStoreErr`) already maps the four store sentinels
(`ErrNotFound`, `ErrConflict`, `ErrInvalid`, `CodeError`) and is used at 94 call sites. The
surrounding boilerplate is not shared. Measured over the 25 non-test files in
`internal/httpapi`, `writeProblem` has 191 call sites in total: the exact literal
`writeProblem(w, http.StatusInternalServerError, "intern", "Internal error", "")` appears 63
times, the not-found form 36 times, the unprocessable form 38 times and the
precondition-failed form 10 times.

The concrete divergence, not just the repetition: the `If-Match` check plus 412 sequence is
spelled out inline at `handlers.go:263-283`, `handlers.go:437-455` and `handlers.go:130-150`
instead of going through the shared mapper. A new endpoint has to remember to add it by hand.

Fix: extend `writeStoreErr` to cover the version-mismatch and validation cases, then convert
the three inline copies. Leave the 63 internal-error sites alone; they are one line each and
churning them costs more review than it saves.

### 2.2 Two copies of the path-traversal guard

`safe(root, key)` exists twice with identical bodies: `internal/files/files.go:49` and
`internal/storage/local.go:167`. This is the security-relevant duplicate, because a fix to
one does not reach the other. Two more exact duplicates: `deref`
(`internal/store/stammdaten.go:670` and `internal/httpapi/reisen.go:921`) and `emptyNil`
(`internal/store/reisen.go:814` and `internal/httpapi/stammdaten.go:799`).

Fix: one guard in `internal/files` (it already owns blob storage semantics) and have
`internal/storage/local.go` call it.

### 2.3 Same name, different semantics

- `yearOf` returns `time.Now().UTC().Year()` for unparseable input at
  `internal/store/abrechnung.go:876`, but `0` at `internal/httpapi/reisen.go:907`.
- `itoa` delegates to a hand-rolled `fmtInt` at `internal/store/aufbewahrung.go:251`, while
  `internal/httpapi/snapshot.go:557` uses `strconv.FormatInt` directly.
- `compact` means a JSON compactor at `internal/audit/chain.go:68` and a `[]string` trimmer
  at `internal/auth/provision.go:55`.

Rename one side in each pair, or delete the helper that only exists to wrap a stdlib call.

### 2.4 One mapping written twice

`internal/httpapi/protokoll.go:28-33` inlines the `sqlitedb.AuditEreigni` to
`api.ProtokollEreignis` mapping that already exists as `toProtokoll`
(`internal/httpapi/abrechnung.go:396`). Two copies of the same field list.

### 2.5 Six unused UI component trees and two unused dependencies

Verified by `rg` with the component's own directory excluded; each has zero importers in
`web/src` and `e2e`:

| Directory | Files | Lines |
| --- | --- | --- |
| `web/src/lib/components/ui/alert/` | 5 | 127 |
| `web/src/lib/components/ui/avatar/` | 7 | 157 |
| `web/src/lib/components/ui/checkbox/` | 2 | 46 |
| `web/src/lib/components/ui/dropdown-menu/` | 18 | 404 |
| `web/src/lib/components/ui/scroll-area/` | 3 | 83 |
| `web/src/lib/components/ui/select/` | 13 | 297 |

That is 1,114 lines, about 21% of the 5,267 lines in the `web/src/lib/components/ui` tree
(209 files), while the live select primitive is `native-select` (11 importers). The cost is
not just dead bytes: `svelte-check` and Biome still process these trees, and two select APIs
sit side by side, so a contributor picks the wrong one.

Also unused, verified with no import anywhere in `web/src`, `e2e` or `scripts`:

- `@testing-library/svelte` (`web/package.json:34`); all six Vitest files import only
  `vitest`.

`workbox-window` (`web/package.json:27`) looked like a third one and is not: the generated
`virtual:pwa-register/svelte` module imports it, and Vite resolves that import from the
`web` package, so removing it fails the production build. Nothing in this repository
names it, which is exactly why the removal looked safe; only a build from a clean
`pnpm install` shows the difference.
- Four dead re-exports in `web/src/lib/beleg/corners.ts:3-8` (`detectFromRGBA`,
  `profileWidth`, `suggestProfil`, `warpRGBA`); every real consumer imports `./geometry`
  directly, so the barrel is a second public surface for the same code.

Caveat before deleting: the claim is grep-verified, not compiler-verified. Run `pnpm --filter
web check` and `pnpm exec biome ci .` after the removal; `select` and `dropdown-menu` are
`bits-ui` wrappers whose importer chain (`label`, `field-label`, `command-input`) is live, so
only the wrappers themselves go.

### 2.6 Six exported Go symbols with no caller

Verified declaration-only, tests included:

| Symbol | Declared at |
| --- | --- |
| `store.SessionByToken` | `internal/store/store.go:373` |
| `store.DeleteSessions` | `internal/store/store.go:460` |
| `store.SetGroupAdmin` | `internal/store/store.go:336` |
| `berechnung.SachbezugBetrag` | `internal/berechnung/berechne.go:985` |
| `export.Sample` | `internal/export/sample.go:38` |
| `satz.InlandFields` | `internal/satz/satz.go:515` |

`export.Sample` is a one-line wrapper over `SampleLang`; `cmd/reisekosten/main.go:249,255`
calls `SampleLang` and `Samples`, never `Sample`. `internal/export/snapshot.go:89-90`
(`VMA24hCent`, `VMA8hCent`) is serialized into the export snapshot JSON and read by nothing,
in the backend or `web/src`.

Symbols that look dead and are not, so they are not touched: `fx.EmbeddedBMF`
(`internal/fx/fx.go:84` calls it), `fx.ParseBMF` (`fx.go:273,275`), `id.NewV7`
(`id.go:30`), `audit.Canonical` (`chain.go:82`), `config.ParseRate` (`config.go:534`),
`ki.RetryWait`, `satz.SatzortKey` (`satz.go:189,494`), `satz.LineError`,
`store.RetentionItem`, `belegpipe.EncodeDisplay` (`pipe.go:145,179`) and `belegpipe.Photo`.

## Tier 3: CI/CD, build and docs

### 3.1 One CI job builds an arm64 binary and throws it away

`go-crossbuild` (`.github/workflows/ci.yml:202-213`) runs
`CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags nodynamic -o /tmp/reisekosten-arm64
./cmd/reisekosten`. Nothing reads that path. The `image` job already builds both platforms
through the same command in `Dockerfile:27-31` (`ci.yml:289-300` passes both platforms), and
`scripts/oci-size.py` reports the result. Dropping the job removes a runner, a module download
and a full arm64 compile from every Go pull request.

### 3.2 The `changes` filters overlap in both directions

From `.github/workflows/ci.yml:39-83`: the `go` filter lists 9 paths and the `image` filter
18, and they share 8. Every `internal/**`, `db/**`, `api/**` or `go.mod` change therefore runs
five Go jobs *and* a two-platform image build, compiling the same module twice.

The gap runs the other way too. `image` does not list `.github/workflows/ci.yml`,
`.golangci.yml`, `.node-version`, `biome.json` or `mise.toml`, so a change to any of them
rebuilds nothing and is verified by nothing. `scripts/compose-check.sh` does trigger the full
25-minute image build, because `scripts/**` is an `image` entry (`ci.yml:76`), even though
`.dockerignore:16` keeps `scripts` out of the build context and no `scripts` file can change
the image.

### 3.3 `backup` gates on the wrong signal

`.github/workflows/ci.yml:454-456` gives the `backup` job `needs: changes` and
`if: needs.changes.outputs.image == 'true'`, so it runs on every `deploy/**` or `scripts/**`
change and re-renders the Compose files through `scripts/compose-config.sh`
(`ci.yml:462`), duplicating `scripts/compose-check.sh` that the `image` job already ran at
`ci.yml:286` in the same run.

### 3.4 The image is serialized twice before four consumers load it

`.github/workflows/ci.yml:302-306` materializes the OCI archive into the daemon with `skopeo`,
then `ci.yml:312` runs `docker save | zstd` and uploads the result, and four jobs each run
`zstd -d | docker load` (`ci.yml:329` smoke, `:349` e2e, `:400` pdfa, `:479` scan; the e2e
matrix multiplies that to six loads). One `skopeo copy` straight to a docker-archive (or an
amd64-only `type=docker` build output) removes the second serialization and the load step in
every consumer.

### 3.5 `go-test` installs two tools only to un-skip one guard

`.github/workflows/ci.yml:140-152` apt-installs `poppler-utils` and extracts the Typst binary
from a digest-pinned image. The only thing that needs them is the self-skip at
`internal/export/export_test.go:95-97`. Either make that test's requirement explicit in the
job name, or drop the install and accept the skip.

### 3.6 Tool versions are pinned in places `toolchain-parity` cannot see

`mise.toml:1-14` states that CI is the source of truth and `docs/development.md:5` repeats it,
and `scripts/mise-check.sh` checks seven tools by reading `go.mod`, `.node-version`,
`package.json` and grepping `ci.yml`. Not covered:

- `pnpm` 12.10.1 appears at `package.json:3`, `ci.yml:228`, `ci.yml:246`, `ci.yml:362` (inside
  a `bash -lc` string) and `Dockerfile:6` (`ARG PNPM_VERSION`). `mise-check.sh:36-38` reads
  only `package.json`.
- Typst 0.15.1 appears at `ci.yml:146`, `Dockerfile:35` and `mise.toml:21`.
  `mise-check.sh:49` greps only `ci.yml`.
- Playwright 1.64.0 appears at `ci.yml:361` and `e2e/package.json:9`, and is in `mise.toml`
  nowhere.
- `govulncheck` 1.8.0 appears twice in one job, as the cache key at `ci.yml:192` and the
  install tag at `ci.yml:197`; only the second form is grepped.

Extend `mise-check.sh` to read the workflow and the Dockerfile for these, so the parity job
fails when they drift.

### 3.7 The same shell loops are written five times

Five copies of "poll a port or health endpoint, dump logs, exit 1":
`.github/workflows/ci.yml:160-169`, `scripts/e2e-up.sh:41-52`, and `scripts/smoke.sh:37-45`,
`:62-72` and `:74-80` (three in one file). The retry budgets have drifted apart (30×1s, 40×3s
and 30×1s inside `smoke.sh` alone, 40×1s in `e2e-up.sh` and 40×1s for MinIO), and
`smoke.sh:60` carries a comment that exists only because the loops were copied.

Six near-identical hardened `docker run` blocks re-state the image's security posture:
`scripts/smoke.sh:17-31`, `scripts/e2e-up.sh:11-39`, `scripts/litestream-smoke.sh:36-45` and
`:54-63`, and `.github/workflows/ci.yml:406-412` and `:413-415`. The last two even re-pin
`--user 65532:65532` and `TYPST_PATH`, which `Dockerfile:43-44` already sets.
`deploy/docker-compose.yml:6-10` adds the seventh and eighth copies of the same flags.

`scripts/compose-check.sh` and `scripts/compose-config.sh` render the same Compose files with
different override lists (one passes `APP_BASE_URL` through the environment, the other seds
the `LITESTREAM_*` values into the file), so the two renderings can disagree.

### 3.8 A dead Compose file, three dead root tasks

- `e2e/compose.ci.yml` (30 lines) is referenced only by prose at `docs/SPEC.md:1191`. CI uses
  `scripts/e2e-up.sh` (`ci.yml:351`). The dead file has already drifted: it lacks the `AI_*`
  variables, the rate-limit overrides and the `--security-opt`/`--cap-drop` flags that
  `e2e-up.sh:14-16,33-38` has, and names its volume `rkdata` instead of `rk-e2e-data`.
- Root `package.json:8-10` defines `codegen`, `check` and `build`, none of which is ever
  invoked; every caller uses `pnpm --filter web …` directly (`ci.yml:253-254`,
  `Dockerfile:16`, `Makefile:14-15,22`, `scripts/codegen-check.sh:6`). `e2e/package.json:6`
  has the same dead `test` script.

### 3.9 The bundle budget reports but cannot fail

`scripts/bundle-budget.mjs:24-26` prints three numbers and exits 0; the only failure path is
an initial-chunk count other than one (`:16-19`). CI (`ci.yml:256`, `Makefile:17`) therefore
cannot fail on bundle growth. Pick a threshold from a measured build and enforce it.

### 3.10 The i18n check compares top-level keys only

`scripts/check-i18n.mjs:5` uses `Object.keys(data)`. It is correct today because both
message files are flat (346 keys each once `$schema` is excluded, zero drift, verified by
flattening and diffing both files), but it would silently pass if a nested group were
introduced in one locale only. Flatten before comparing, and add placeholder and
untranslated-value checks while it is open.

### 3.11 Documentation drift

- `docs/SPEC.md:1027` documents `make dev`; the Makefile has five targets (`check`, `test`,
  `web`, `generate`, `fmt`) and no `dev`.
- The same line claims `make check` runs "alle Linter und Tests wie im CI". It runs neither
  `golangci-lint` (`ci.yml:113`) nor `govulncheck` (`ci.yml:200`), and
  `internal/export/export_test.go:95-97` self-skips under it for want of poppler.
- `deploy/.env.example` declares 93 variables against 87 documented rows in
  `docs/configuration.md`. The `*_FILE` variants are separate lines in the example but a
  suffix in the docs (`INITIAL_ADMIN_PASSWORD_FILE`, `OIDC_CLIENT_SECRET_FILE`,
  `S3_SECRET_ACCESS_KEY_FILE`, `AI_API_KEY_FILE`), and `MAIL_IMAP_PASSWORD_FILE` exists only
  in prose at `docs/configuration.md:116`, with no example line at all.
- `scripts/mise-check.sh:7` says "The go-lint job runs it"; the caller is the separate
  `toolchain-parity` job (`ci.yml:126`).

## Checked and rejected

Recorded so nobody re-opens them.

- **A whole-table read of the stored daily rates (1.12, second half).** The calculation reads
  the rate of a line's own currency and never another currency's, so the unbounded read of
  every row is gone: with 10,000 stored rows (20 currencies over 500 business days) it cost
  41ms per call against 2.1ms for the 500 rows of one currency, and it ran on every endpoint
  that calculates. The bounded read stays, because a foreign line does need its currency's
  rates.
- **A per-user day-index cache in front of the calculation (1.3).** Measured on 20 trips
  with 100 expense lines: `ListReisenAll` alone is 6.6ms and a full `calculate` is
  10.4-16.6ms, so the recomputation a cache would remove is a few milliseconds at that
  size, while the cache would have to be invalidated on every trip and expense write of that
  user. Revisit when an account reaches hundreds of trips, and key the index by calendar day
  then, since that is all the rule needs.
- **Batching `checkBelege`'s per-id `GetBeleg` (1.12).** The ids come from the expense in
  hand, so the loop is one primary-key lookup per linked receipt, one to three of them, in a
  write transaction that is open anyway. An `IN` list would save microseconds.
- **Reducing the audit row to one per changed identity (1.8, the performance half).** The
  login path writes an audit row per authenticated request. Dropping it is a data-safety
  decision and not a performance one, so it stays until the maintainer calls it; the check
  that the Nutzer is still active stays per request either way.
- **Sharing `deref` and `emptyNil` (2.2).** `internal/store` and `internal/httpapi` each
  declare them with identical bodies, and a single home would mean one package owning a
  helper the other has no business reaching into (a pointer dereference is not a store
  concern). The bodies are two to five lines with no branching to drift.
- **`belegdatei_sha_idx` looks duplicated** across `db/migrations/00004_belege.sql:45` and
  `00008_beleg_bild.sql:26`. It is not: 00008 rebuilds the table
  (`DROP TABLE belegdatei` at `:24`), and dropping a table drops its indexes, so recreating
  the index is required.
- **The trip `LIKE '%…%'` search** (`db/queries/reise.sql`) is a guaranteed scan, but no
  index can serve a leading-wildcard `LIKE`. An FTS table for a single-user self-hosted app
  is more machinery than the problem deserves.
- **Merging Dockerfile `COPY` layers** in the runtime stage. Four static `COPY`s of immutable
  files cost four layer-metadata entries and preserve per-file cache granularity; merging
  them buys nothing measurable.
- **The `panic` calls** at `internal/id/id.go:32`, `internal/satz/satz.go:292` and
  `internal/httpapi/server.go:111` are deliberate fail-fast on unrecoverable init and
  construction errors. Returning errors instead would add plumbing with no recovery path.
- **The `_ =` discards in bulk.** Measured at roughly 200 in non-test tracked Go (the exact
  figure depends on whether inline `, _ :=` forms are counted), the large majority are
  correct (best-effort cleanup, already-answered response headers). Only the audit and
  recalculation classes in 1.8 and 1.9 carry a real consequence.
- **`pr-titel` checking out the repository.** It does not; `.github/workflows/ci.yml:506-515`
  has a single `run` step that greps the PR title from the environment. (It still costs a
  full runner for a regex.)

- **The image filter's overlap with `go` and `web` (3.2, the overlap half).** The filter
  lists `scripts/**` although `.dockerignore` keeps `scripts` out of the build context. It
  has to: `smoke` and `e2e` load the built image and run the shell suites under `scripts/`,
  so a script change must rebuild it. The other direction is no gap either:
  `.golangci.yml`, `.node-version`, `biome.json` and `mise.toml` change no image content,
  and each one already gates the job that reads it.
- **Making the bundle budget a hard gate (3.9).** The item asks for it, but SPEC 13 keeps
  it a budget: the shadcn surface takes precedence over the earlier 150 KB target for the
  initial JS, and the script logs the sizes instead of failing. A gate would turn an
  unrelated UI change red for a number the SPEC deliberately left open, so the decision is
  the maintainer's and not a cleanup's.
- **Dropping the tools the export guard needs (3.5).** `go-test` extracts Typst and installs
  poppler so `internal/export/export_test.go` runs instead of self-skipping. Removing the
  install saves a small image pull per Go change and costs the PDF export coverage, which is
  the wrong trade for the only job that exercises it.
- **The end-to-end environment in SPEC 18.4.** The section names MinIO and a
  `STORAGE_BACKEND=s3` Playwright project; neither the workflow nor `scripts/e2e-up.sh` has
  either. The file reference was corrected while the dead Compose file was removed; the
  claim itself is a spec question, not drift a cleanup can settle.

## Suggested order

1. 1.1 and 1.2 (behaviour fixes, no design work), each with its own test.
2. 1.6 and 1.7 (additive migrations, bounded verification cost).
3. 1.8 through 1.10 (per-request cost and audit safety) as one slice, since they share the
   external-login path.
4. 1.3 through 1.5 and 1.11 through 1.12 (query batching, read pool, typst runs) as
   separate slices, each measurable before and after.
5. Tier 2 in any order; 2.2 and 2.5 are the ones with a user-visible payoff.
6. Tier 3 last, because 3.2 and 3.6 change what CI runs.
