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
| Apply `EXPORT_TIMEOUT` to the Typst export | [#17](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/17) |
| Apply `TZ_DEFAULT` when a new trip has no zone | [#18](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/18) |
| Use `BELEG_JPEG_QUALITY` for PDF-source pages in the export | [#19](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/19) |
| Map stammdaten and profile store errors with `writeStoreErr` | [#20](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/20) |
| Fail startup when a boolean env var is not a boolean | [#21](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/21) |
| Measure AVIF encode time in the container, then accept ADR 0005 or switch to WebP | O7, [#29](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/29) |
| Track JStG 2026 and ship the 2027 Satztabelle when the BMF letter is out (expected Nov/Dec 2026) | O12, [#28](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/28) |
| Second design pass leftover: Ausgabe and Beleg forms still open as separate pages, not in a side panel like the Fahrt form | – |
| Research: add BMF-RK 2020 Rz. 64–86 wording to the tax research | O1 |

Watch only, no action planned: O10 Dreimonatsfrist heuristic, O11 German VAT in foreign currency, O13 Frühstück herausrechnen, O14 erste Tätigkeitsstätte not modelled.

## Possible features

Ideas, not committed. Larger items get a design note in [ideas/](ideas/).

| Idea | Notes |
|---|---|
| **Email receipt intake** into the Belegeingang | [Design note](ideas/email-receipt-intake.md), [#25](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/25) |
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
