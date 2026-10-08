# Stack-Recherche: Frontend, Linting, DB-Portabilität, Multi-Arch-CI, Versionen

> **Status:** Recherche, nur Fakten und Messwerte, keine Produktentscheidungen · **Stand:** 08.10.2026 (Abfragen ca. 23:30 MESZ)
> **Quellen:** npm-Registry (`npm view`), GitHub Releases/API, `proxy.golang.org`, go.dev, Container-Registries, Projekt-Dokumentation. Hands-on-Spikes liegen unter `docs/research/stack-spikes/` (ohne `node_modules`).
> **Messumgebung:** Box mit 8 Kernen, Node 24.21.0, Go 1.27.2, npm. Zeiten sind Richtwerte, keine Benchmarks.

---

## 1. Frontend: SolidJS 2.0 RC + solid-ui vs. Svelte 5 + shadcn-svelte

### 1.1 Status SolidJS 2.0

| Fakt | Wert |
|---|---|
| Neueste Version | `solid-js@2.0.0-rc.14` (dist-tag `next`), veröffentlicht **08.10.2026** |
| npm `latest` | `solid-js@1.9.17` (07.10.2026), also weiterhin 1.x |
| Zeitleiste | experimental.0 13.02.2025 → beta.0 03.03.2026 (35 Betas) → rc.0 12.08.2026 → rc.14 08.10.2026 (14 RCs in 8 Wochen) |
| Stable-Termin | Kein angekündigter Termin. Laut RC-Blogpost ist die „API eingefroren“. |
| Tatsächliche Stabilität | rc.14 enthält trotzdem als **„Breaking“** markierte Änderungen (z. B. `AttributeSlot` → `BindingSlot`, kleingeschriebene `on*`-Attribute sind keine Event-Handler mehr). |
| Node-Anforderung | `solid-js@next` verlangt Node ≥ 22.12 |

**Wesentliche Breaking Changes 1.x → 2.0** (Quelle: `documentation/solid-2.0/MIGRATION.md`, RC-Blogpost):
- Das Web-Runtime ist jetzt ein eigenes Paket `@solidjs/web`, `solid-js/web` und `solid-js/store` gibt es nicht mehr. `jsxImportSource` ist `@solidjs/web`, und `solid-js` exportiert keinen `JSX`-Namespace mehr.
- `createResource` entfällt (stattdessen async `createMemo`/`createStore(fn)` + `<Loading>`). `Suspense` heißt jetzt `Loading`, `ErrorBoundary` heißt `Errored`, `SuspenseList` heißt `Reveal`.
- `batch` ist ersetzt durch `flush()`, `onMount` durch `onSettled`. `createEffect(compute, apply)` hat jetzt zwei Phasen. `on`, `createComputed`, `startTransition`, `useTransition`, `produce` und `createMutable` entfallen.
- **`splitProps`/`mergeProps` entfallen**, stattdessen gibt es `omit`/`merge` (per `Object.keys(await import('solid-js'))` geprüft).
- Neu sind der Compiler auf Oxc-Basis (Rust) in `@solidjs/vite-plugin` sowie Actions und Optimistic-APIs im Core.

### 1.2 Ökosystem-Kompatibilität mit Solid 2.0 RC (Stand heute)

| Paket | Version | Solid-2-Status | Geprüft |
|---|---|---|---|
| `@solidjs/vite-plugin` | 3.0.0-next.47 (30.09.) | ✅ peer `solid-js ^2.0.0-rc.13`, `vite ^8 \|\| ^9` | Build ✅ |
| `vite-plugin-solid` | 2.11.14 / next 3.0.0-next.27 | 2.x nur Solid 1, `next` ist ein Wrapper auf `@solidjs/vite-plugin` | – |
| `@solidjs/router` | 2.0.0-next.37 (08.10.) | ✅ peer rc.14, **neue API** (`createRouter`, `defineRoutes`, kein `<Router>`/`<Route>` mehr) | Import/Build ✅ |
| `@solidjs/start` | 2.0.6 (`latest`) | ❌ peer `@solidjs/router <2.0.0-0`, also noch die Solid-1-Linie. Für eine SPA nicht nötig. | – |
| `@kobalte/core` | 0.13.14 `latest` / **2.0.0-alpha.2** (07.09.) | Alpha unterstützt Solid 2, aber **peer exakt `solid-js 2.0.0-rc.3`** (`@kobalte/utils` sogar rc.0). Mit rc.14 nur per `--legacy-peer-deps` installierbar. | Build ✅, Runtime-Smoke ✅, Typecheck ❌ |
| `corvu` (Drawer, OTP …) | 0.7.2 (01/2025) | ❌ peer `solid-js ^1.8`, Teile wurden in die Kobalte-2-Alpha übernommen | – |
| **solid-ui** (stefan-karger) | Registry `solid-ui.com/r/*.json` | ❌ **Keine Solid-2-Version.** Komponenten basieren auf Kobalte 0.13 und `splitProps`. `main` zuletzt am 23.02.2026 gepusht. Der Maintainer schrieb früher, dass er für Solid 2 „alle Komponenten neu machen“ muss. Offener PR #237 „feat(v4): all components“ (Tailwind 4, Stand 23.08.) ist weiterhin auf Solid 1 ausgelegt. | Build ❌ ohne Portierung |
| `shadcn-solid` | Copy-Paste, Kobalte | ebenfalls Solid-1-basiert | – |
| `@tanstack/solid-query` | 5.104.1 `latest` / **6.0.0-rc.5** | ✅ v6 RC peer `solid-js >=2.0.0-rc.13` | Build ✅ |
| `@tanstack/solid-form` | 1.33.5 / 2.0.0-alpha.2 | ❌ importiert `solid-js/web` | Build ❌ |
| `@tanstack/solid-table` | 9.2.7 | ❌ importiert `solid-js/web` | Build ❌ |
| `@formisch/solid`, `@modular-forms/solid` | 1.1.0 / 0.25.1 | ❌ peer `solid-js <2` bzw. `^1.3` | – |
| `@solidjs/testing-library` | 0.8.10 / next 1.0.0-beta.3 | ✅ Beta unterstützt Solid 2 | – |
| `vite-plugin-pwa` | 2.0.0 (03.10.) | ✅ framework-agnostisch, peer `vite ≤ ^8` | Build ✅ (sw.js, 5 Precache-Einträge) |

### 1.3 Hands-on-Spike: Ergebnisse

Gleiche Mini-App in allen drei Varianten: Überschrift, Select mit MwSt-Sätzen (0/7/19 %), Dialog mit Button. Gebaut mit Vite 8.3.4 und Tailwind 4.3.3, anschließend per Playwright-Smoke (`stack-spikes/smoke.mjs`) im Headless-Chromium geklickt: Dialog öffnen, mit Escape schließen, „7 %“ im Select wählen.

| Variante | Setup-Probleme | JS gzip | CSS gzip | `vite build` (warm, 3×) | Typecheck | Runtime-Smoke |
|---|---|---|---|---|---|---|
| **Solid 2.0 rc.14** + Kobalte 2.0.0-alpha.2 + solid-ui (button, dialog, select) | 1) Peer-Konflikt, nur mit `--legacy-peer-deps`. 2) Build-Fehler `"splitProps" is not exported by solid-js`, alle 17 Stellen per Skript auf `omit()` portiert. 3) **33 TS-Fehler** nach Portierung (`JSX` nicht aus `solid-js`, Kobalte-2-Typen `PolymorphicProps`/`ValidComponent` geändert, `FormControlDescriptionProps` nicht mehr generisch) | **59,3 KB** | 4,4 KB | 0,29 / 0,22 / 0,24 s | ❌ 33 Fehler | ✅ ohne Konsolenfehler |
| Referenz: **Solid 1.9.17** + Kobalte 0.13.14 + solid-ui (unverändert) | keine | **48,0 KB** | 4,2 KB | 0,61 / 0,72 / 0,56 s | ✅ 0 Fehler (auch mit TS 7.0.2) | ✅ |
| **Svelte 5.57.2** + bits-ui 2.19.5 + shadcn-svelte 1.7.0 (button, dialog, select) | `shadcn-svelte init` verlangt nicht-interaktiv ein `--preset`-Kürzel und blieb ohne TTY hängen, also `components.json` von Hand angelegt. `add … --yes` lief danach sauber. Die Registry-`utils.ts` importiert das neue Paket `cn` (shadcn-ui/cn 0.4.0), das manuell installiert werden musste. | **78,6 KB** | 6,8 KB | 1,74 / 1,79 / 1,77 s | ✅ `svelte-check` 0 Fehler (mit TS 6.0.3) | ✅ |

**Größenaufteilung** (Quelltextanteile laut Sourcemap, vor Minify):
- Solid 2: `@solidjs/signals` 283K, `@kobalte/core` 116K, `tailwind-merge` 104K, `@solidjs/web` 79K, `@floating-ui` ~62K
- Solid 1: `@kobalte/core` 142K, `tailwind-merge` 104K, `solid-js` 81K, `@floating-ui` ~62K
- Svelte: `svelte` (Runtime) 419K, `bits-ui` 187K, `tailwind-variants` 137K, `cn` 44K, `@floating-ui` ~62K, `tabbable` 27K

**Nackte Runtime** (Zähler-Button, ohne UI-Bibliothek, JS gzip): Solid 1.9 **4,3 KB** · Svelte 5 **10,1 KB** · Solid 2.0 rc.14 **11,6 KB**. Solid 2 hat also den Größenvorteil von Solid 1 gegenüber Svelte 5 im Leerlauf nicht mehr. Mit UI-Bibliothek liegt Solid trotzdem ~20 KB gzip unter Svelte + bits-ui.

**Build-Zeit:** Solid 2 baut dank Oxc-Compiler am schnellsten (~0,25 s), Svelte am langsamsten (~1,8 s, 713 Module durch bits-ui und Icons). Bei Projekten dieser Größe ist beides vernachlässigbar.

### 1.4 Ökosystem-Reife (Fakten)

- **Svelte 5:** stabil seit 10/2024. SvelteKit 3.0.0 erschien am 01.10.2026 (3.0.1 am 06.10.). bits-ui 2.x und shadcn-svelte 1.x sind stabil und werden aktiv gepflegt (Releases 09–10/2026). TanStack Query, Form und Table haben Svelte-5-Adapter (`@tanstack/svelte-query` 6.3.1, `svelte-form` 1.33.5, `svelte-table` 9.2.8), dazu `sveltekit-superforms` 3.0.0. Hinweis: `@vite-pwa/sveltekit` 1.1.0 unterstützt nur Kit 1/2. Für eine reine Vite-SPA ohne Kit funktioniert `vite-plugin-pwa` 2.0.0 direkt (getestet).
- **Solid 2:** Core, Router und Query (RC) sind bereit. Der UI-Unterbau (Kobalte 2) ist **Alpha** mit exakt gepinntem, veraltetem RC als Peer. **Es gibt keine Solid-2-fähige shadcn-Portierung.** Formulare (TanStack Form, Formisch, Modular Forms) und Tabellen (TanStack Table) sind noch nicht Solid-2-fähig.
- **Solid 1.9:** stabil, solid-ui und Kobalte 0.13 funktionieren ohne Anpassung. Ein späterer Umstieg auf 2.0 bedeutet aber eine Migration (Imports, Effects, Resources, Props-Helfer, UI-Komponenten).

### 1.5 Empfehlung (Begründung, keine Entscheidung)

**Die Bedingung „Solid 2.0 RC + solid-ui, wenn kompatibel“ ist heute nicht erfüllt.** solid-ui hat keine Solid-2-Version, und Kobalte 2 ist Alpha mit Peer-Konflikt. Die Komponenten bauen erst nach händischer Portierung und haben dann noch 33 Typfehler. Formulare und Tabellen fehlen. Ein eigener Fork von solid-ui auf Kobalte-Alpha würde den Wartungsaufwand vor allem in die UI-Schicht verlagern, also genau dorthin, wo man ihn mit shadcn vermeiden will.

Daraus folgt nach der vom Nutzer vorgegebenen Regel („falls nicht, dann Svelte“): **Svelte 5 + shadcn-svelte (bits-ui) + Tailwind 4 als reine Vite-SPA.** Das Setup ist stabil, typsauber und vollständig (Forms, Tables, Query, PWA). Dafür ist das Bundle ~20–30 KB gzip größer als bei Solid. Für eine PWA, die nach dem ersten Laden aus dem Service-Worker-Cache startet, spielt das praktisch keine Rolle.

Ehrliche Gegenposition: Solid ist im Laufzeitmodell schlanker (feingranulare Updates ohne Svelte-Runtime-Anteil) und hatte im Spike das kleinere Bundle. **Solid 1.9 + solid-ui** wäre eine sofort funktionierende Alternative, bringt aber eine absehbare 2.0-Migration mit. Solid 2 erneut prüfen, sobald `solid-js@latest` = 2.x ist und solid-ui/Kobalte stable sind.

---

## 2. Linting, Formatierung, Typecheck (ohne ESLint)

Getestete Versionen: Biome 2.5.15, Oxlint 1.87.0, oxfmt 0.72.0.

### 2.1 Biome 2.5.15

| Thema | Ergebnis (hands-on) |
|---|---|
| `.tsx` (Solid-JSX) | ✅ Linting und Formatierung voll. Solid-Regeln: `correctness/noSolidDestructuredProps`, `performance/useSolidForComponent` (Rule-Domain `solid`). a11y-Regeln (z. B. `useAltText`) greifen. |
| `.svelte` **Standard** | ⚠️ Nur der `<script>`-Teil wird gelintet. **False Positives:** im Template benutzte Variablen und Imports werden als `noUnusedVariables`/`noUnusedImports` gemeldet. Templates werden nicht geprüft. |
| `.svelte` mit `"html": { "experimentalFullSupportEnabled": true }` | ✅ Template-bewusst: keine False Positives mehr, a11y im Template (`useAltText`, `useButtonType`) greift, und Svelte-Dateien werden formatiert (Script und Markup). Das Flag heißt aber weiterhin **„experimental“**. Svelte-Regeln liegen in **`nursery`**: `noSvelteAtDebugTags`, `noSvelteAtHtmlTags`, `noSvelteExportLet`, `noSvelteLegacyConst`, `noSvelteUnnecessaryStateWrap`, `useSvelteRequireEachKey`, `useSvelteKitRuneImports`. |
| Formatter | Biome formatiert TS/TSX/JSON/CSS und (mit Flag) Svelte. Im Test wurde der `<script>`-Inhalt ohne Einrückung ausgegeben (Prettier-Svelte rückt standardmäßig ein); das ist konfigurierbar. |

### 2.2 Oxlint 1.87.0 / oxfmt 0.72.0

| Thema | Ergebnis |
|---|---|
| `.tsx` | ✅ 870+ native Regeln, `jsx-a11y`-Plugin nativ. **Kein natives Solid-Plugin.** `eslint-plugin-solid` (0.18.1) lässt sich als **JS-Plugin (alpha)** über `jsPlugins` laden. Getestet: `solid/reactivity` meldet Destrukturierung von `props` korrekt. Type-aware Linting über `--type-aware` (tsgolint). |
| `.svelte` | ⚠️ **Nur `<script>`-Blöcke.** `no-unused-vars` wird für `.svelte` komplett übersprungen (dokumentiert, Issue #17991). `eslint-plugin-svelte` ist nicht nutzbar, da JS-Plugins **keine Custom-Parser** unterstützen (Issue #20373, offen). Es gibt also keine Template-Regeln. |
| oxfmt `.tsx` | ✅ nativ (Rust) |
| oxfmt `.svelte` | ✅ mit `{"svelte": true}` in `.oxfmtrc.json` und installiertem `svelte` (Prettier-Plugin-basiert, gebündelt; Script-Teil nativ). Getestet: funktioniert, Ausgabe entspricht Prettier-Stil (Einrückung im Script, `<img />`). Ohne die Option werden `.svelte`-Dateien still übersprungen. |

### 2.3 Typecheck

- **TypeScript 7.0.2** (Go-nativ, `latest` seit 08.07.2026): `tsc --noEmit` auf dem Solid-1-Spike in 0,36 s ohne Fehler.
- **`svelte-check` 4.7.6:** peer `typescript ^5 || ^6`, **startet nicht mit TS 7** (getestet: Absturz). Mit **TS 6.0.3** gab es 0 Fehler. Für Svelte also vorerst auf TS 6 bleiben.
- Solid: `tsc` (TS 7) reicht. Svelte: `svelte-check` (TS 6).

### 2.4 Zusammengefasst (Fakten)

- **Für Svelte** bietet Biome mit `html.experimentalFullSupportEnabled` als einziges Tool Lint und Format inklusive Templates in einem Werkzeug. Der Preis: Experimental-Flag und Svelte-Regeln im `nursery`. Oxlint sieht nur `<script>` und hat dort kein `no-unused-vars`.
- **Für Solid** bieten beide gute Abdeckung. Biome bringt zwei native Solid-Regeln mit, Oxlint lädt das vollständige `eslint-plugin-solid` per Alpha-JS-Plugin (Solid-2-Semantik dort unklar).
- Mögliche Kombination: Biome (Lint und Format für Frontend, JSON, CSS) + `svelte-check` (Typen und Svelte-Compiler-Warnungen, die auch a11y abdecken). Oxlint optional zusätzlich für Geschwindigkeit und type-aware Regeln. Alternativ oxfmt als Formatter, wenn Prettier-identische Svelte-Ausgabe gewünscht ist.

---

## 3. DB-Portabilität: SQLite jetzt, Postgres später

### 3.1 Treiber (CGO-frei für distroless und Cross-Compile)

| Treiber | Version | CGO | Cross-Compile arm64 mit `CGO_ENABLED=0` | Binary (Testprogramm, `-s -w`) | Kaltbuild je Arch | 20k Inserts in 1 Tx (WAL) |
|---|---|---|---|---|---|---|
| `modernc.org/sqlite` | v1.60.1 (29.09.) | nein (C nach Go transpiliert) | ✅ | 6 MB | 14–19 s | 546 ms |
| `github.com/ncruces/go-sqlite3` | v0.35.6 (23.09.) | nein (SQLite-Wasm per **wasm2go** zu Go übersetzt, kein wazero mehr; das `embed`-Import ist obsolet) | ✅ | 8 MB | ~13,5 s | 584 ms |
| `github.com/mattn/go-sqlite3` | v1.14.52 | **ja** | nur mit C-Cross-Toolchain (zig/gcc-aarch64) bzw. QEMU, statisches Linken für distroless/static nötig | – | – | – |

Beide CGO-freien Treiber liefen mit goose v3.28.0 (eingebettete Migrationen), WAL-Modus, `busy_timeout` und Foreign Keys über DSN-`_pragma` problemlos. Litestream 0.5.17 nutzt selbst `modernc.org/sqlite` und läuft als offizielles Multi-Arch-Image (`litestream/litestream:0.5.17-scratch`, amd64 und arm64) als Sidecar.

### 3.2 Query-/Code-Gen-Optionen

| Option | Version | Multi-DB | Ansatz | Wartungsaspekt |
|---|---|---|---|---|
| **sqlc** (Dual-Engine) | v1.31.1 (22.04.) | sqlite, postgresql, mysql | Pro Engine ein `sql:`-Block, **gleiche Schema- und Query-Dateien**, zwei generierte Pakete | Siehe Spike 3.3 |
| **bob** | v0.50.0 (11.08.) | sqlite, psql, mysql | Typsicherer Query-Builder plus Codegen aus der DB, auch aus `.sql`-Dateien | Pre-1.0, API-Änderungen zwischen Minor-Versionen |
| **jet** | v2.16.1 (06.10.) | sqlite, postgres, mysql | Typsicherer SQL-Builder, Codegen aus Live-DB, **pro Dialekt eigenes Import-Paket** | Querycode dialektgebunden |
| **sqlx** | v1.4.0 (04/2024) | alle | Handgeschriebenes SQL, `Rebind()` für Platzhalter | Keine Typsicherheit, wenig aktiv |
| **ent** | v0.14.6 (03/2026) | sqlite, postgres, mysql | ORM mit Schema-as-Code und Atlas-Migrationen | Schwergewichtig, widerspricht „sqlc + goose“ |
| **goose** | v3.28.0 (02.09.) | `SetDialect("sqlite3"/"postgres")` | Führt SQL-Dateien aus, **übersetzt aber keine DDL** | Portables DDL oder getrennte Migrationsordner pro Dialekt |

### 3.3 Spike: sqlc mit zwei Engines, eine Query-Datei (`stack-spikes/sqlc-dual-engine`)

- `sqlc.yaml` mit `engine: sqlite` → `internal/store/sqlitedb` und `engine: postgresql` → `internal/store/pgdb`, beide auf **dieselben** `db/migrations` und `db/queries`. Parameter als `sqlc.arg(name)`. sqlc setzt die Platzhalter je Engine (`?1` bzw. `$1`). Beide Pakete kompilieren.
- Mit portablem DDL (`TEXT`, `BIGINT`, `TIMESTAMP`; Geldbeträge in Cent als `BIGINT`) und `database/sql` auch für Postgres (pgx-stdlib statt `sql_package: pgx/v5`) waren **die Modelle identisch** und die Queries bis auf die Platzhalter gleich. Einzige Abweichung: `LIMIT`-Parameter `int32` (pg) vs. `int64` (sqlite), lösbar per `CAST` oder `overrides`.
- **Fallstricke gefunden:**
  - **sqlc-Bug (offen, Issue #3974):** In der SQLite-Engine verschwinden bei `x BETWEEN sqlc.arg(a) AND sqlc.arg(b)` die BETWEEN-Parameter aus dem Params-Struct, **der generierte Code ist zur Laufzeit falsch**. Workaround: `x >= … AND x <= …`.
  - Mit `TIMESTAMPTZ` (pg-only) wurde das SQLite-Feld zu `interface{}`, mit `pgx/v5` als `sql_package` werden es `pgtype.*`-Typen. Für identische Typen also `database/sql` und gemeinsame Typen verwenden.
  - Die Param-Structs sind paketspezifisch (`sqlitedb.CreateTripParams` ≠ `pgdb.CreateTripParams`). Ein gemeinsames Repository-Interface braucht deshalb einen dünnen Adapter pro Engine, oder man nutzt nur eine Engine und behält die Option für später.
  - SQL-Dialektunterschiede (`RETURNING` geht in beiden, `ON CONFLICT` in beiden; JSON-Funktionen, Datumsfunktionen, `STRICT`-Tabellen und Upsert-Details unterscheiden sich) muss man selbst im gemeinsamen Subset halten.

### 3.4 Wartungsärmster Pfad (Einordnung)

1. **Jetzt:** sqlc nur mit Engine `sqlite`, `database/sql`, CGO-freier Treiber, goose mit eingebetteten Migrationen. DDL und Queries bewusst im gemeinsamen Subset halten (portable Typen, Geld in Cent, Datum als ISO-Text oder `TIMESTAMP`, kein `BETWEEN`, keine SQLite-Spezialfunktionen). Der Datenzugriff läuft hinter einem schmalen Store-Interface im Domain-Code.
2. **Bei Bedarf:** zweiten `sql:`-Block `engine: postgresql` in `sqlc.yaml` ergänzen (gleiche Dateien), pgx-stdlib-Treiber und Adapter schreiben. Migrationen nur dann in `migrations/{sqlite,postgres}` aufteilen, wenn DDL divergieren muss. Der CI-Job „sqlc generate für beide Engines“ wäre ein günstiger Wächter, der Portabilitätsbrüche früh zeigt.

Treiberwahl: `modernc.org/sqlite` und `ncruces/go-sqlite3` sind beide geeignet. modernc ist weiter verbreitet (auch in Litestream und vielen Projekten) und erzeugte die kleinere Binary. ncruces hat eine moderne API und unterstützt ebenfalls `database/sql`.

---

## 4. Multi-Arch-Images und CI

### 4.1 Build-Muster ohne QEMU

- **SPA einmal bauen:** `FROM --platform=$BUILDPLATFORM node:24-… AS web` → `pnpm build`. Das Ergebnis ist architekturunabhängig.
- **Go cross-kompilieren:** `FROM --platform=$BUILDPLATFORM golang:1.27-… AS build`, `ARG TARGETOS TARGETARCH`, `CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w"`, SPA per `//go:embed` aus der `web`-Stage. Gemessen: Kaltbuild ~14–19 s je Arch (8 Kerne), warm < 1 s. CGO-frei ist Voraussetzung (siehe 3.1).
- **Final-Stage:** `FROM gcr.io/distroless/static-debian13:nonroot` mit **nur `COPY`**, kein `RUN`. Weil in der Zielplattform nichts ausgeführt wird, braucht der amd64-Runner für das arm64-Image **keine Emulation**, und `docker buildx build --platform linux/amd64,linux/arm64` läuft auf einem einzigen Runner. `tonistiigi/xx` (v1.9.0) ist nur bei CGO nötig.
- **Go-Caches im Dockerfile:** `RUN --mount=type=cache,target=/root/.cache/go-build` und `…/go/pkg/mod`. Diese Cache-Mounts werden **nicht** von `cache-to: type=gha/registry` exportiert. Für Persistenz in GitHub Actions braucht es z. B. `reproducible-containers/buildkit-cache-dance` oder den Go-Build außerhalb von Docker (`actions/setup-go` mit Cache) und `COPY` des fertigen Binaries ins Image.
- **Native arm64-Runner:** Laut GitHub Changelog vom **29.01.2026** gibt es `ubuntu-24.04-arm` und `ubuntu-22.04-arm` **auch für private Repos** auf allen Plänen. Sie zählen gegen die Freiminuten, haben in privaten Repos **2 vCPU** (öffentlich 4) und sind kein Larger-Runner-Feature. Nützlich, falls ein Schritt doch nativ auf arm64 laufen muss (z. B. E2E-Test des arm64-Images). Dann gilt das Docker-Muster „Matrix je Plattform, `push-by-digest`, anschließend Manifest mergen“. Für reines Cross-Compile ist das nicht nötig.

### 4.2 buildx-Cache

| Backend | Eigenschaften |
|---|---|
| `type=gha` | Nutzt die GitHub-Actions-Cache-API (v2), keine Registry-Credentials nötig, Scope pro Branch mit Fallback auf den Default-Branch. Teilt sich das Repo-Cache-Kontingent (standardmäßig 10 GB, Eviction nach LRU/7 Tagen) mit `setup-go`/pnpm-Caches. |
| `type=registry` (z. B. `ghcr.io/…:buildcache`, `mode=max`) | Kein 10-GB-Limit, branchübergreifend und auch lokal nutzbar. Braucht `packages: write` und erzeugt ein zusätzliches Package in ghcr. |

Aktuelle Action-Versionen: `docker/setup-buildx-action` v4.4.1, `docker/build-push-action` v7.4.0, `docker/metadata-action` v6.2.0, `docker/login-action` v4.6.0, `docker/setup-qemu-action` v4.4.0 (bei diesem Muster nicht benötigt), `actions/checkout` v7.0.1, `actions/setup-go` v7.0.0, `actions/setup-node` v7.1.0, `pnpm/action-setup` v6.1.0, `actions/cache` v6.1.0, `golangci/golangci-lint-action` v9.3.0, `dorny/paths-filter` v4.0.3, `actions/attest-build-provenance` v4.2.2, `sigstore/cosign-installer` v4.1.2, buildx v0.38.0.

### 4.3 Base-Image

| Image | Arches | Inhalt | Anmerkung |
|---|---|---|---|
| `gcr.io/distroless/static-debian13:nonroot` | amd64, arm64, arm, s390x, ppc64le, riscv64 | CA-Zertifikate, tzdata, `/etc/passwd` mit `nonroot` (UID 65532), ~2 MiB | Die Distroless-README führt **debian13** als aktuelle Linie. `debian12`-Tags existieren noch, sind dort aber nicht mehr gelistet. Tag `:debug-nonroot` mit Busybox-Shell für Fehlersuche. |
| `cgr.dev/chainguard/static:latest` | amd64, arm64 | Ähnlich (Wolfi-basiert), nonroot standardmäßig | Kostenlos ist nur `:latest`, gepinnte Versions-Tags gibt es nur im Bezahlkatalog. |
| `scratch` | – | leer | Zertifikate, tzdata und User müssen selbst kopiert werden. Für TLS zum OIDC-Provider/S3 sind CA-Zertifikate nötig, Zeitzonen über `time/tzdata`-Embed. |

### 4.4 Typst im distroless-Image

- **Typst v0.15.1** (17.07.2026). Release-Binaries sind **statisch gelinkt (musl)** für `x86_64` und `aarch64` (geprüft: `ldd` meldet „statically linked“) und damit in `static-debian13` lauffähig. Größe: **~53 MB unkomprimiert** (~16–17 MB als `.tar.xz`).
- **Offizielles Multi-Arch-Image** `ghcr.io/typst/typst:0.15.1` (linux/amd64, linux/arm64). Das Binary liegt unter `/bin/typst`. Damit reicht im Dockerfile `COPY --from=ghcr.io/typst/typst:0.15.1 /bin/typst /usr/local/bin/typst`. BuildKit wählt automatisch die passende Arch, es gibt kein `RUN` und keine Emulation. Renovate kann diesen Tag pflegen.
- **Funktionstest:** Ein Bericht mit Tabelle und einem **eingebetteten PDF-Beleg** (`#image("beleg.pdf")`, seit Typst 0.14 möglich, Seite wählbar über `page:`) wurde in **221 ms** zu einem 2-seitigen PDF kompiliert. Der Text des eingebetteten PDFs bleibt erhalten (Vektor, per `pdftotext` lesbar). Typst kann Beleg-PDFs und -Bilder also selbst anhängen. pdfcpu (v0.16.1) bleibt nützlich für Seitenzahl-Ermittlung, Validierung, Attachments oder als Fallback beim Mergen.
- **Fonts:** Typst bringt eingebettete Standardfonts mit (Libertinus, New Computer Modern, DejaVu Sans Mono). Eigene Fonts per `--font-path` ins Image kopieren. distroless enthält keine Systemfonts.
- **Nachtrag 09.10.2026 – PDF/A:** Typst 0.15.1 kennt `--pdf-standard` u. a. `a-2b`, `a-3b`, `a-3u`, `a-4`, `ua-1`. Spike auf der Box: Mit `a-3b`/`a-3u` funktionieren Tabellen, JPEG-Bilder und Dateianhänge (`#pdf.attach(…, relationship: "source")`, per `pdfdetach -list` geprüft). **In allen PDF/A-Modi bricht `#image("beleg.pdf")` ab** („embedding PDFs is currently not supported in this export mode“); `a-2b` verbietet zusätzlich Anhänge. Typst kann PDF-Seiten aber als PNG rendern (`typst compile --format png --ppi 300` mit eingebettetem PDF), was für Vorschauen und gerasterte Exportseiten reicht. veraPDF-Validierung steht aus (kein Java auf der Box).
- **Go-native Alternativen**, falls Typst als externes Binary stört: `github.com/johnfercher/maroto/v2` v2.4.3 (04.10.2026, Grid- und Tabellen-Layout auf gofpdf-Basis), `github.com/signintech/gopdf` v0.38.1 (low-level). Beide ohne Template-Sprache. Das Einbetten fremder PDFs geht dort nur über pdfcpu. Das Image wäre ~17 MB kleiner, dafür ist der Layout-Code imperativ.

---

## 5. Aktuelle stabile Versionen (Stand 08.10.2026)

| Komponente | Version | Datum | Quelle/Anmerkung |
|---|---|---|---|
| **Go** | 1.27.2 | 08.10.2026 | go.dev (1.27.0 am 19.08.2026; 1.26.9 weiterhin unterstützt) |
| Docker `golang` | `1.27.2-trixie`, `1.27.2-alpine3.24` | – | Docker Hub |
| sqlc | v1.31.1 | 22.04.2026 | GitHub/proxy |
| goose (`pressly/goose/v3`) | v3.28.0 | 02.09.2026 | proxy |
| go-oidc (`coreos/go-oidc/v3`) | v3.21.0 | 01.09.2026 | proxy |
| `golang.org/x/oauth2` | v0.37.0 | 25.08.2026 | proxy |
| `golang.org/x/crypto` (enthält `argon2`) | v0.57.0 | 08.09.2026 | proxy |
| `alexedwards/argon2id` | v1.0.0 | 21.10.2023 | stabil, dünner Wrapper um x/crypto/argon2 |
| `alexedwards/scs/v2` | v2.9.0 | 17.04.2025 | Stores: `sqlite3store` (Pseudo-Version 02.10.2025), pgxstore u. a. |
| oapi-codegen (`/v2`) | v2.8.0 | 17.07.2026 | erstmals OpenAPI-3.1-Support, `std-http-server` + `strict-server`; benötigt Go ≥ 1.25 und `oapi-codegen/runtime` ≥ v1.6.0 (aktuell v1.7.0, 16.08.2026) |
| ogen (Alternative) | v1.24.0 | 07.08.2026 | proxy |
| openapi-typescript / openapi-fetch | 7.13.0 / 0.17.0 | 11.02.2026 | npm |
| @hey-api/openapi-ts | 0.99.0 | 22.06.2026 | npm (Pre-1.0) |
| pdfcpu | v0.16.1 | 04.10.2026 | GitHub |
| **Typst** | v0.15.1 | 17.07.2026 | GitHub; Image `ghcr.io/typst/typst:0.15.1` |
| Tailwind CSS (+ `@tailwindcss/vite`) | 4.3.3 | 16.07.2026 | npm |
| **Vite** | 8.3.4 | 08.10.2026 | npm (Rolldown-basiert) |
| Vitest | 5.0.3 | 30.09.2026 | npm; Node `^22.12 \|\| ^24 \|\| >=26` |
| Playwright | 1.64.0 | 07.10.2026 | npm/GitHub |
| Biome | 2.5.15 | 30.09.2026 | npm |
| Oxlint | 1.87.0 | 05.10.2026 | npm |
| oxfmt | 0.72.0 | 05.10.2026 | npm (Pre-1.0) |
| vite-plugin-pwa | 2.0.0 | 03.10.2026 | npm; Breaking nur `@vite-pwa/assets-generator ^2` als Peer erlaubt; Node ≥ 20.19 |
| Workbox (`workbox-build`/`-window`) | 7.4.1 | 04.05.2026 | npm |
| TypeScript | 7.0.2 (`latest`) / 6.0.3 | 08.07.2026 | TS 7 ist Go-nativ; `svelte-check` braucht noch TS ≤ 6 |
| golangci-lint | v2.14.0 | 24.09.2026 | GitHub |
| Litestream | v0.5.17 | 31.08.2026 | GitHub; Image `litestream/litestream:0.5.17-scratch` (amd64, arm64) |
| distroless static | `gcr.io/distroless/static-debian13:nonroot` | rolling | per Digest pinnen (Renovate) |
| **Node.js LTS** | 24.21.0 („Krypton“, Active LTS) | 07.09.2026 | nodejs.org; 22.23.3 („Jod“) Maintenance; 26.11.1 Current (LTS-Wechsel turnusgemäß Ende Oktober) |
| pnpm | 12.10.1 | 06.10.2026 | npm |
| Bun | 1.4.2 | 05.09.2026 | GitHub |
| Renovate | 44.148.1 | 08.10.2026 | npm/GitHub (GitHub-App nutzt immer die aktuelle Version) |
| Svelte | 5.57.2 | 06.10.2026 | npm |
| `@sveltejs/vite-plugin-svelte` | 7.3.1 | 23.09.2026 | peer `vite ^8` |
| bits-ui | 2.19.5 | 03.10.2026 | npm |
| shadcn-svelte (CLI) | 1.7.0 | 16.09.2026 | npm |
| svelte-check | 4.7.6 | 13.08.2026 | npm |
| SvelteKit (falls genutzt) | 3.0.1 | 06.10.2026 | npm; `adapter-static` 4.0.0 |
| solid-js | 1.9.17 stable / 2.0.0-rc.14 | 07.10. / 08.10.2026 | npm |
| SQLite-Treiber | modernc v1.60.1 · ncruces v0.35.6 · mattn v1.14.52 | 29.09. / 23.09. / 05.09.2026 | proxy |

---

## 6. Offene Punkte und Grenzen dieser Recherche

- Docker ist auf der Box nicht installiert. Das Multi-Arch-Dockerfile-Muster (4.1) ist deshalb aus Doku und Einzeltests (Cross-Compile, statisches Typst-Binary, Image-Manifeste) abgeleitet, ein kompletter `buildx`-Lauf wurde nicht gemacht.
- Postgres wurde nicht zur Laufzeit getestet, nur die sqlc-Generierung und Kompilierung des pg-Pakets.
- `eslint-plugin-solid` 0.18.1 über Oxlint-JS-Plugins wurde nur gegen Solid-1-Code getestet. Wie gut es Solid-2-Semantik versteht, ist offen.
- Das Bundle-Ergebnis hängt stark von den genutzten Komponenten ab. Gemessen wurden nur Button, Dialog und Select.
