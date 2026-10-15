# Roadmap

What is done, what is still open, and ideas that are not committed. Status as of 10 October 2026.
The open points table in [SPEC §22](SPEC.md#22-offene-punkte) and the later list in [SPEC §21](SPEC.md#21-später-liste-nicht-v1) stay the source of the history; this page is the overview.

## Done

| # | Milestone | Release | Result |
|---|---|---|---|
| M1 | [Walking skeleton](MILESTONES.md#m1--walking-skeleton-010) | 0.1.0 | Sign-in (password, OIDC, header), PWA shell, multi-arch image, release-please |
| M2 | [Arbeitgeber, Tätigkeitsstätten, Satztabellen](MILESTONES.md#m2--arbeitgeber-tätigkeitsstätten-satztabellen-020) | 0.2.0 | Employers, places of work, rate tables 2024–2026 with admin overrides and CSV import |
| M3 | [Reisen und Pauschalen](MILESTONES.md#m3--reisen-und-pauschalen-030) | 0.3.0 | Trips with Ortswechsel, Reisetage, Fahrten; calculation core with golden tests |
| M4 | [Belege](MILESTONES.md#m4--belege-040) | 0.4.0 | Camera/file to Archivbeleg (colour AVIF), Belegeingang, duplicates, S3 |
| M5 | [Ausgaben, MwSt, Fremdwährung, Bewirtung](MILESTONES.md#m5--ausgaben-mwst-fremdwährung-bewirtung-050) | 0.5.0 | Expenses with VAT shares, ECB rates, warnings |
| M6 | [KI-Auslesen](MILESTONES.md#m6--ki-auslesen-060) | 0.6.0 | Optional AI suggestions from receipts |
| M7 | [Abrechnung und Export](MILESTONES.md#m7--abrechnung-und-export-070) | 0.7.0 | Claims with status, unlock, advances; PDF/A-3b, ZIP, CSV, JSON |
| M8 | [Export polish, Aufbewahrung, backup](MILESTONES.md#m8--export-feinschliff-aufbewahrung-backup-080) | 0.8.0 | PDF layout de/en, veraPDF in CI, retention, Litestream as UID 65532 |
| M9 | [CI, security, README polish](MILESTONES.md#m9--ci--security--und-readme-feinschliff-100) | 1.0.0 | CodeQL, Trivy, cosign, SBOM, screenshots |

Also done after M9: shadcn-svelte UI with dark default; second design pass with summary cards, trip tabs (Tage, Ausgaben, Fahrten, Belege, Verlauf), collapsible day rows with meal toggles, Fahrt form in a side panel, dashboard home, viewport-sized README screenshots; release workflow fixes ([#22](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/22), [#24](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/24)); actionlint in CI.

Resolved open points (history in SPEC §22): O2 (decided, see below), O3 veraPDF job, O4 release-please flow, O5 CodeQL and O6 public repo (moot, the repo is public), O8 router, O9 glossary, O16 Litestream as UID 65532.

Backend config wiring done in `main` (through commit `fecc74c`): `EXPORT_TIMEOUT`, `BELEG_JPEG_QUALITY`, `EXPORT_PDF_STANDARD`, `FX_ECB_URL` and `FX_TIMEOUT` are read from the environment and applied; startup fails on a malformed boolean environment variable; `TZ_DEFAULT` defaults the zone of a new trip; stammdaten and profile store errors map through `writeStoreErr` ([#17](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/17)–[#21](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/21), [#31](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/31)).

Also done: the own audit log `GET /api/v1/protokoll`, the admin system status `GET /api/v1/admin/system` ([#32](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/32), [#33](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/33)), and the receipt capture in a side panel like the Fahrt form ([#35](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/35)), plus a PWA install button with an iOS share-sheet hint ([#34](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/34)). Personal API tokens (stored only as a SHA-256 hash) with a bearer-authenticated receipt upload landed as the first #25 slice ([#25](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/25), design-note option d); and `POST /api/v1/eingang/mail` files a raw email's attachments into the Belegeingang (design-note a/c, reuses the upload path, dedupes by checksum, archives the `.eml`). The profile UI to mint a `mail:ingest` token (with its forwarding address) and to manage the sender allowlist landed too. A Beleg now also carries its origin: `quelle` (`upload` or `mail`), the archive of the `.eml`, sender, subject and date, readable through `GET /api/v1/belege/{id}/mail` and shown in the Beleg detail and in the Belegeingang ([#38](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/38)). Intake now refuses a mail whose Authentication-Results verdict names an unproven sender (spf/dkim/dmarc fail, softfail or permerror) and keeps the verdict it saw on the Beleg. A mail without that header is still filed, because the verdict is the receiving server's and not ours. Remaining for #25: the IMAP transport still has no durable unit test; verify it against a containerized IMAP server (Dovecot/GreenMail).

## Open tasks

### Maintainer (Eduard)

| Task | Reference |
|---|---|
| Re-run `release.yml` on `main` with `tag=v1.0.0`; the v1.0.0 release is still a **draft** because the `notes` job failed in the first run | [#24](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/24) |
| Make the GHCR package `ghcr.io/blackdark/vc-reisekostenabrechnung` public (then switch the image size badge to a live one) | O6 |
| Install the Renovate GitHub app for the repo | O15, [#2](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/2) |
| Send 15–20 real receipt photos to validate ADR 0005 (still `proposed`) | O7, [#29](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/29) |
| Confirm O2: one Verpflegungspauschale per calendar day across all employers (current default `EineVerpflegungspauschaleProKalendertag = true`) | O2 |

### Code

| Task | Reference |
|---|---|
| Measure AVIF encode time in the container, then accept ADR 0005 or switch to WebP | O7, [#29](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/29) |
| Track JStG 2026 and ship the 2027 Satztabelle when the BMF letter is out (expected Nov/Dec 2026) | O12, [#28](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/28) |
| O1: authoritative basis for the meal deduction confirmed in the research (BMF-RK 2020 + § 9 Abs. 4a); verbatim Rz. 64–86 wording still pending a fetchable HTML source (PDF not parseable here, researcher unavailable) | O1 |
| Work through the [code health pass](ideas/code-health.md). Done: the pagination "next page" guess ([#42](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/42)) and the missing indexes ([#43](https://github.com/BlackDark/vc-reisekostenabrechnung/pull/43)). Left of the first three: the `deploy/.env.example` quality defaults | [ideas/code-health.md](ideas/code-health.md) |

Watch only, no action planned: O10 Dreimonatsfrist heuristic, O11 German VAT in foreign currency, O13 Frühstück herausrechnen, O14 erste Tätigkeitsstätte not modelled.

**Fixed:** the audit hash chain was linked in write order but verified by id order; with millisecond-resolution UUIDv7 ids, same-millisecond rows could verify out of order. The chain now orders by an explicit `nr` column filled in write order, so `LastAuditHash` and `ListAudit` agree without depending on rowid, which is SQLite-only and broke the Postgres probe; `TestApiTokenLifecycle` verifies it.

### Spec audit (10 October 2026)

Every decision in SPEC §0 (Q1–Q35), the calculation rules with golden tests G01–G20, the API, auth, ENV, CI, Renovate, ADRs 0001–0005 and MILESTONES was checked against the code. The former open code tasks #31–#35 are implemented; what remains is gated on external events (ADR 0005 via [#29](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/29), the 2027 Satztabelle via [#28](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/28)) or research (O1). ADR 0005 stays `proposed` until [#29](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/29) is done.

## Possible features

Ideas, not committed. Larger items get a design note in [ideas/](ideas/).

| Idea | Notes |
|---|---|
| Personal API tokens + upload endpoint (n8n, iOS Shortcuts, scanners) | [#26](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/26) |
| PWA Web Share Target for PDFs and photos | [#27](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/27) |
| E-Rechnung parsing (XRechnung/ZUGFeRD to fields) | SPEC §21 |
| Offline capture with background sync | SPEC §21 |
| Automatic km via a routing service | SPEC §21 |
| DATEV export | SPEC §21 |
| Passkeys/WebAuthn, TOTP | SPEC §21 |
| Postgres engine | SPEC §21, ADR 0003 |
| Approval workflow by a second person | SPEC §21 |
| Reminder for the Dreimonatsfrist and for unassigned Belege (mail or push) | new |
| Recurring trips as templates (same route, same employer) | new |
