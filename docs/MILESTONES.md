# Meilensteine

> Vertikale, jeweils auslieferbare Schritte zur Umsetzung von [`SPEC.md`](SPEC.md). Jeder Meilenstein endet mit einem Release über release-please (Versionen `0.x`, `1.0.0` nach M9), einem grünen CI-Lauf und einem lauffähigen Multi-Arch-Image auf ghcr.io.
> Repo: `git@github.com:BlackDark/vc-reisekostenabrechnung.git` (öffentlich). Stand: 09.10.2026.

**Definition of Done (für jeden Task):** Code + Tests (Unit/Integration, wo sinnvoll E2E), OpenAPI und Generat aktuell, UI-Texte in `de` **und** `en`, Glossarbegriffe korrekt, Audit-Ereignisse für Zustandsänderungen, CI grün („CI ok“), Conventional-Commit-PR-Titel, Doku (SPEC/ADR) angepasst, falls eine Entscheidung abweicht.

| # | Meilenstein | Release | Kernergebnis |
|---|---|---|---|
| M1 | Walking Skeleton | 0.1.0 | Login (Passwort, OIDC, Header), leere App als PWA, Multi-Arch-Image, CI ≈ 3 min, release-please-Fluss |
| M2 | Arbeitgeber, Tätigkeitsstätten, Satztabellen | 0.2.0 | Arbeitgeber, Tätigkeitsstätten, Satztabellen 2024–2026 mit Admin-Overrides und CSV-Import |
| M3 | Reisen und Pauschalen | 0.3.0 | Reisen mit Ortswechseln, Reisetagen, Fahrten; Rechenkern mit Golden-Tests |
| M4 | Belege | 0.4.0 | Kamera/Datei → Archivbeleg (Farb-AVIF), Belegeingang, Duplikate, S3 |
| M5 | Ausgaben, MwSt, Fremdwährung, Bewirtung | 0.5.0 | vollständige Kostenerfassung mit Steueranteilen, EZB-Kursen, Warnungen |
| M6 | KI-Auslesen | 0.6.0 | optionale Vorschläge aus Belegen über OpenAI-kompatiblen Endpunkt |
| M7 | Abrechnung und Export | 0.7.0 | Abrechnungen mit Status, Entsperrung, Vorschüssen; PDF/A-3b + ZIP/CSV/JSON |
| M8 | Export-Feinschliff, Aufbewahrung, Backup | 0.8.0 | vollständiges PDF-Layout de/en, veraPDF-Validator, Fristen, Litestream |
| M9 | CI-, Security- und README-Feinschliff | 1.0.0 | CodeQL, Trivy, cosign, SBOM, Screenshot-Durchlauf, README mit Screenshots |

---

## M1 – Walking Skeleton (0.1.0)

**Ziel:** Ein dünner Durchstich durch alle Schichten: Nutzer meldet sich per Passwort oder Pocket ID an und sieht eine leere, installierbare App in Deutsch oder Englisch. Image läuft multi-arch, read-only und nonroot; die günstigen CI-/Release-Fundamente stehen von Anfang an.

**Spec:** 2, 3.6, 9 (Auth, Betrieb), 10 (ohne 10.9-Details), 11, 13 (Grundgerüst), 14 (Grundgerüst), 15, 16, 17, 18.1, 18.2, 18.5, 18.7, 19.

**Nicht enthalten:** Fachobjekte, Trivy/cosign/SBOM, Screenshot-Durchlauf, README-Feinschliff (→ M9). CodeQL ist enthalten, weil das Repo öffentlich ist (SPEC O5).

**Abnahmekriterien**
- `curl` von `deploy/docker-compose.yml` + `.env.example`, `.env` ausfüllen, `docker compose up -d` startet die App auf amd64 **und** arm64 (Image aus ghcr.io); fehlende Pflichtvariablen brechen mit `… fehlt` ab.
- Ersteinrichtung per Setup-Token oder `INITIAL_ADMIN_*`; Login per Passwort; Login per Pocket ID mit PKCE; `OIDC_ALLOWED_GROUP` legt Konten an, `OIDC_ADMIN_GROUP` vergibt Admin; Header-Auth wirkt nur aus `TRUSTED_PROXIES`.
- PR-CI (warm) ≈ 3 min; `e2e` hängt nur von `image` ab; nur PR-Läufe werden abgebrochen; jeder Job hat `timeout-minutes`; Pflicht-Check „CI ok“ ist auch bei Doku-PRs grün.
- Container-Smoke-Test (read-only, UID 65532, tmpfs `/tmp`, `cap-drop ALL`) prüft Health, Readiness, Login-Seite, Neustart auf demselben Volume.
- release-please öffnet mit `GITHUB_TOKEN` einen Release-PR, auf dem die CI läuft; nach Merge veröffentlicht `release.yml` erst nach kompletter CI das Multi-Arch-Image und schreibt Tags + Digest in die Release-Notes.

**Tasks**
1. **Repo-Grundgerüst Backend** – Go-Modul, `cmd/reisekosten` (`serve`, `migrate`, `healthcheck`, `config check`), `internal/config` (ENV + `*_FILE`, Validierung), `slog`, `/healthz`, `/readyz`, `/version`, SPA-Embed, `Makefile`, `.golangci.yml`. *AC:* `make check` grün; ungültige Konfiguration bricht mit verständlicher Meldung ab.
2. **SPA-Shell** – Vite 8 + Svelte 5 + Tailwind 4 + shadcn-svelte, Biome mit Svelte-Template-Support, `svelte-check` (TS 6), Paraglide de/en mit Sprachumschalter, responsives App-Layout, Login-Seite, `vite-plugin-pwa` (Manifest, Icons, Update-Prompt); Router-Spike `sv-router` vs. `svelte-spa-router` (O8) mit Entscheidung im PR. *AC:* App installierbar (Lighthouse „installable“), kein horizontales Scrollen bei 360 px.
3. **DB- und API-Fundament** – goose-Migration `0001` (nutzer, nutzer_identitaet, session, audit_ereignis, job), sqlc (Engine sqlite) + Postgres-Probe, `api/openapi.yaml` mit Auth-/Profil-Endpunkten, oapi-codegen + openapi-typescript/-fetch, Problem-Details, Audit-Hash-Kette mit `audit verify`. *AC:* Generat-Diff-Check im CI; Migrationstest leer → aktuell.
4. **Authentifizierung** – Argon2id, scs-Sessions (`__Host-`-Cookie), Ersteinrichtung (10.6), OIDC mit PKCE/nonce/Gruppen/Verknüpfung, Header-Auth + `TRUSTED_PROXIES` + Client-IP, `http.CrossOriginProtection`, Sicherheits-Header/CSP, Login-Rate-Limit, minimale Admin-Nutzerliste. *AC:* Auth-Matrix-Tests; E2E Passwort- und OIDC-Login gegen `mock-oauth2-server`.
5. **Container und Compose** – Dockerfile nach 17.1 (BUILDPLATFORM, Cross-Compile, Typst-COPY, distroless nonroot, Cache-Mounts), `.dockerignore`, `deploy/docker-compose.yml` mit `${VAR:?fehlt}`, `deploy/.env.example` ohne Beispiel-Secrets, `deploy/docker-compose.backup.yml` + `deploy/litestream.yml`; `docker compose config`-Check im CI. *AC:* `docker buildx build --platform linux/amd64,linux/arm64` läuft ohne QEMU; die Image-Größe wird im Build-Log ausgegeben.
6. **CI-Pipeline** – `ci.yml` nach 18.2: `changes`, `go`, `codegen`, `web`, `image` (Multi-Arch-Build ohne Push + amd64-Artefakt, `type=gha`-Cache, `buildkit-cache-dance`), `smoke` (18.5), `e2e` (Login-Flows, desktop + mobile), `pr-titel`, `ci-ok`; Concurrency nur für PRs abbrechen; Timeouts. *AC:* warmer PR-Lauf ≤ 3:30 min, gemessen an drei PRs.
7. **Release-Fluss** – `release-please-config.json`, Manifest `0.1.0`, `release-please.yml` (Release-PR, `gh workflow run ci.yml --ref <branch>`, `gh workflow run release.yml --ref <tag>`), `release.yml` (`workflow_call` der CI, Multi-Arch-Push nach ghcr, Notes mit Tags und Digest); O4 verifizieren. **Eduard:** „Allow GitHub Actions to create and approve pull requests“ aktivieren, Ruleset für `main`, Renovate-App installieren (O15). *AC:* Release `v0.1.0` mit Image-Digest in den Notes.
8. **Renovate scharf schalten** – `renovate.json5` (bereits im Repo) wirkt; Onboarding-PR prüfen, erste Digest-Pins mergen. *AC:* Dependency-Dashboard vorhanden, Automerge eines devDependency-Patches nach grüner CI beobachtet.

---

## M2 – Arbeitgeber, Tätigkeitsstätten, Satztabellen (0.2.0)

**Ziel:** Ein Nutzer pflegt seine Arbeitgeber und Tätigkeitsstätten; ein Admin sieht die ausgelieferten Satztabellen 2024–2026, überschreibt Werte mit Begründung und importiert ein neues Jahr per CSV.

**Spec:** 2, 3.2 (Nutzer, Arbeitgeber, Tätigkeitsstätte, Satztabelle, Auslandssatz, Satzüberschreibung), 3.3 (I2, I3), 4.1, 9 (Arbeitgeber, Tätigkeitsstätten, Satztabellen, Admin), 10.9.

**Abnahmekriterien**
- Genau ein Standard-Arbeitgeber (I3); Logo-Upload erscheint im Profil-Vorschau-Briefkopf.
- Satztabellen 2024, 2025, 2026 vollständig (223/224/214 Auslandseinträge, [R] 2.5), jedes Land hat einen ISO-Code; Ersatzzuordnungen und Luxemburg-Fallback abfragbar.
- Admin-Override ändert den wirksamen Wert, ist im Protokoll mit Grund sichtbar; CSV-Import eines Testjahres 2027 als Entwurf → aktivieren.
- Mandantentrennung per Test für alle neuen Repository-Methoden.

**Tasks**
1. **Arbeitgeber** – Migration, sqlc, API, UI (Liste, Formular, Standard setzen, archivieren, Logo als `datei`). *AC:* I3 per DB-Constraint/Transaktion erzwungen.
2. **Tätigkeitsstätten** – CRUD mit Land/Satzort-Auswahl aus der Satztabelle. *AC:* Satzort-Auswahl zeigt „im Übrigen“ und Städte (z. B. Paris, Genf).
3. **Satztabellen-Daten** – Einbetten der CSVs aus `docs/research/data/`, `satztabellen/laender.csv` (BMF-Name ↔ ISO 3166-1), Inlandswerte 2024–2026, Ersatzländer; Lesetest „alle Länder gemappt“. *AC:* Lookup-Funktion `Satz(jahr, land, satzort)` mit Tests für Fallbacks.
4. **Satztabellen-UI und -API** – Ansicht je Jahr mit Suche; Admin-Overrides mit Grund; CSV-Import (Format = Recherche-CSV + `land_iso`), Validierungsbericht, Aktivieren. *AC:* Import einer fehlerhaften CSV zeigt Zeilenfehler, ändert nichts.
5. **Nutzerverwaltung und Profil** – Admin: Nutzer anlegen/deaktivieren, Admin-Flag, Passwort zurücksetzen, Identität verknüpfen/lösen; Nutzer: Profil, Sprache, Personalnummer, aktive Sessions beenden. *AC:* letzter Admin kann sich nicht entziehen; Deaktivieren beendet Sessions.
6. **E2E** – Flows für Arbeitgeber, Tätigkeitsstätten und Satztabellen (de/en). *AC:* in `e2e` integriert.

---

## M3 – Reisen und Pauschalen (0.3.0)

**Ziel:** Reisen (ein- und mehrtägig, Inland und Ausland) mit Ortswechseln erfassen, Reisetage mit gestellten Mahlzeiten und Unterkunft pflegen, Fahrten mit Kilometerpauschale; der Rechenkern zeigt live die Tagesberechnung.

**Spec:** 3.2 (Reise, Ortswechsel, Reisetag, Fahrt, Vorlage, Projekt), 3.3 (I1, I4), 4.1–4.9, 4.14, 4.16 (G01, G02, G04–G14, G18, G20), 8 (B05, B06, W03, W12, W14), 9 (Reisen, Fahrten, Vorlagen, Projekte).

**Abnahmekriterien**
- Golden-Tests G01, G02, G04–G14, G18, G20 grün; Property-Tests für „max. eine Pauschale je Tag“ und Reihenfolgeunabhängigkeit.
- Reise in < 1 min am Handy anlegbar; Tagesansicht zeigt je Reisetag Tagesart, Land, Pauschale, Kürzungen, Regel-IDs.
- Manuelle Land-Überschreibung nur mit Begründung, protokolliert.

**Tasks**
1. **Rechenkern I – Tage und Pauschalen** – Tagesart, Abwesenheit (Zeitzonen), VMA Inland/Ausland, Über-Nacht, Mahlzeitenkürzung inkl. Zuzahlung/Kappung, Satztabelle nach Datum. *AC:* G01, G02, G04, G11, G12, G18.
2. **Rechenkern II – Maßgebliches Land** – Regeln 4.6 (Grundregel, Rückreisetag, Flug > 2 Tage, Schiff, Satzort, Ersatzländer). *AC:* G07, G08, G09, G10.
3. **Rechenkern III – Mehrere Reisen am Tag** – Kandidaten, Zuordnung zur spätest endenden offenen Reise, Anrechnung bereits in gesperrten Reisen gewährter Pauschalen. *AC:* G05, G06; Property-Tests.
4. **Rechenkern IV – Übernachtung und Kilometer** – Unterkunftsarten, Übernachtungspauschale, Fahrten. *AC:* G13, G14.
5. **Reisen-API und Datenmodell** – Migrationen, Reise mit Ortswechseln, Reisetag-Abgleich bei Datumsänderung, `GET /reisen/{id}/berechnung`, Validierungen (B06). *AC:* I1, I4 per Test.
6. **Reisen-UI** – Liste mit Filtern, mobiles Formular (Beginn/Ende mit Zeitzone, Ortswechsel-Editor mit Ländersuche), Tagesansicht mit Mahlzeiten-Häkchen und Unterkunft, Projekt-Autovervollständigung. *AC:* E2E Inland + Ausland, de/en, desktop/mobile.
7. **Fahrten und Vorlagen** – Fahrten-UI mit Hin-/Rückfahrt; Vorlagen für Reisen und Fahrten anlegen/anwenden. *AC:* Vorlage erzeugt unabhängige Kopie.
8. **Dreimonatsfrist und Warnungs-Grundgerüst** – Warnungs-Engine (berechnet), W03 mit Aktion „Pauschale ausschließen“, W12, W14, Startseite „Zu erledigen“. *AC:* G20.

---

## M4 – Belege (0.4.0)

**Ziel:** Belege per Kamera oder Datei erfassen, Ecken korrigieren, als bereinigtes Farb-AVIF archivieren und bestätigen; PDFs/E-Rechnungen unverändert speichern; Speicher lokal oder S3 (EU).

**Spec:** 3.2 (Beleg, Belegdatei, Belegtext), 3.3 (I8, I9, I13), 3.5, 5, 10.8, 12.1, 13 (Kamera, lazy Modul).

**Abnahmekriterien**
- Foto → bestätigter Archivbeleg am Handy in < 30 s; Ecken immer korrigierbar; Vorder- und Rückseite in einem Beleg.
- Archivbeleg Ø ≤ 100 KB je Bon-Seite bei den Fixture-Belegen aus [K]; SHA-256 gespeichert; Überschreiben technisch unmöglich (I8).
- Duplikat-Upload erzeugt W04 mit Link auf den vorhandenen Beleg.
- Storage-Contract-Tests grün für `local` und `s3` (MinIO); S3 ohne EU-Standort startet nicht.
- **Praxistest** mit 15–20 Belegen von Eduard dokumentiert; ADR 0005 auf `accepted` (oder begründet angepasst).

**Tasks**
1. **Storage** – `storage.Store` local + S3 (`minio-go`), `PutIfAbsent`, SHA-Prüfung, EU-Prüfung, Contract-Tests mit MinIO-Service im CI. *AC:* 12.1 vollständig.
2. **Job-Queue** – DB-gestützte Jobs mit Retry/Backoff, Worker-Pool, Status am Objekt. *AC:* Neustart setzt laufende Jobs fort.
3. **Upload-API** – Multipart, Magic-Bytes, Größen-/Pixel-/Seitenlimits, Duplikatprüfung, Belegeingang. *AC:* Upload-Validierungstests (10.8).
4. **Server-Pipeline** – Normalisierung (pure Go, versioniert), AVIF/WebP-Encoder, Vorschau, Golden-Images; Messung der Encode-Zeit auf amd64 und arm64 (O7). *AC:* Pipeline-Version in Metadaten; Fallback WebP per ENV.
5. **Client-Erfassung** – Kamera/Datei, lazy geladene Eckenerkennung im Worker, Ecken-Editor mit Lupe, Entzerrung/Skalierung (Profile bon/A4), Upload JPEG q90, Mehrseitigkeit. *AC:* funktioniert auf iOS Safari (installierte PWA) und Android Chrome.
6. **Bestätigung und Lebenszyklus** – Sichtkontrolle des Archivbelegs, Bestätigen (Belegnummer), Neu-Aufbereiten, Löschen (unbestätigt), Stornieren, Karenz-Job für Erfassungs-JPEGs. *AC:* I9, I13.
7. **PDF und E-Rechnung** – Original speichern, Typst-gerenderte Vorschau, Seitenzahl. *AC:* XML wird nie durch die Bildpipeline geschickt.
8. **Praxistest** – Eduards Belege durch die Kette, Bericht in `docs/research/beleg-kompression.md` ergänzen, ADR 0005 Status setzen. *AC:* Ergebnis im PR dokumentiert.

---

## M5 – Ausgaben, MwSt, Fremdwährung, Bewirtung (0.5.0)

**Ziel:** Alle fünf Kostenarten mit Belegen, mehreren Steueranteilen, Fremdwährung (EZB, überschreibbar), Bewirtung mit digitalem Eigenbeleg und Eigenbelegen erfassen; Warnungen sind vollständig.

**Spec:** 3.2 (Ausgabe, Steueranteil, Eigenbeleg, Wechselkurs), 3.3 (I5), 4.7 (Vorschläge), 4.8, 4.10–4.13, 4.16 (G03, G15, G16, G17), 5.2, 8.

**Abnahmekriterien**
- Golden-Tests G03, G15, G16, G17 grün.
- Ausgabe aus Beleg im Eingang anlegen; ein Beleg für mehrere Ausgaben (Hotel + Parken).
- EZB-Kurs am Belegdatum mit Wochenend-Rückfall; Überschreiben per Kurs oder belastetem Betrag; Währung ohne EZB-Kurs → B04.
- Alle Warnungen und Blocker aus SPEC 8 implementiert und an Ausgabe/Reise/Startseite sichtbar.

**Tasks**
1. **Ausgaben-Modell und API** – Kostenarten, Steueranteile (beide Eingabemodi), I5, Vorsteuerfähigkeit, n:m-Belegzuordnung. *AC:* G15.
2. **Ausgaben-UI** – mobiles Formular je Kostenart, Steueranteil-Editor, Anlegen aus Beleg, Hinweise zu nicht erstattbaren Nebenkosten. *AC:* E2E desktop/mobile.
3. **Fremdwährung** – EZB-Client + Cache, Kursregeln, Überschreiben mit Protokoll, EUR-Aufteilung der Steueranteile. *AC:* G16 mit Fixture.
4. **Mahlzeiten-Vorschläge** – Übernachtung mit Frühstück, Verpflegungs-Ausgabe, Bewirtung → gestellte Mahlzeiten mit `mahlzeit_quelle`, abwählbar. *AC:* G03, G17 (Tagesanteil). Vorher O1 recherchieren.
5. **Bewirtung und Eigenbeleg** – Pflichtangaben, Teilnehmerliste, TSE-Checkbox, digitale Bestätigung mit Zeitstempel, Eigenbeleg-Formular. *AC:* G17; B03, W05, W06.
6. **Warnungen komplett** – W01, W02 (inkl. Arbeitgeber-Namensabgleich), W04 (unscharf), W07–W11, W13; Startseite „Zu erledigen“. *AC:* jede Warnung hat Test + Übersetzung.
7. **MwSt-Helfer** – Gastronomie-Kombipreis (30 %) und Hotel-Business-Package (15 %/20 % nach Jahr). *AC:* Helfer füllen nur vor, Werte editierbar.

---

## M6 – KI-Auslesen (0.6.0)

**Ziel:** Optional Felder aus Belegen per OpenAI-kompatiblem Endpunkt vorschlagen lassen; der Nutzer bestätigt, nichts wird automatisch gebucht.

**Spec:** 6, 3.2 (Belegtext), 10.7, 11 (KI).

**Abnahmekriterien**
- Ohne `AI_ENABLED` ist keine KI-UI sichtbar; mit aktivierter KI nur bei Nutzer-Opt-in.
- Funktioniert mit OpenAI-kompatiblem Fake im CI und manuell mit einem echten Endpunkt (z. B. Ollama).
- Bestätigte Felder und Volltext sind als Belegtext-Version gespeichert; keine Bild-/Prompt-Inhalte oder Keys in Logs.

**Tasks**
1. **KI-Client** – Chat-Completions mit Bild, `json_schema` → `json_object` → Text-Fallback, Timeout, Nebenläufigkeit, Rate-Limit. *AC:* Tests gegen Fake mit allen drei Modi.
2. **Validierung und Speicherung** – Schema- und Plausibilitätsprüfung, unbestätigte Belegtext-Version, PDF-Seiten über Typst-Rendering. *AC:* fehlerhafte Antworten führen zu leerem Vorschlag, nicht zu Fehlerseiten.
3. **UI** – „Mit KI auslesen“, Vorbelegung mit sichtbarer Markierung, Konfidenz, Übernehmen/Verwerfen, Datenschutz-Opt-in im Profil. *AC:* E2E gegen Mock-Endpunkt.
4. **W02 aus KI** – `empfaenger_name` vs. Arbeitgeber vor dem Speichern. *AC:* Warnung erscheint im Formular.

---

## M7 – Abrechnung und Export (0.7.0)

**Ziel:** Abrechnungen für Tag/Woche/Monat/Quartal/frei anlegen, prüfen, einreichen (Sperre + unveränderbarer Export), entsperren (protokolliert), als bezahlt markieren; Vorschüsse verrechnen; PDF/A-3b und ZIP herunterladen.

**Spec:** 3.2 (Abrechnung, Vorschuss, Export), 3.3 (I6, I7, I10–I12), 3.4, 4.15, 4.16 (G19), 7.1, 7.2, 7.3 (Grundlayout), 8 (Prüfung), 9 (Abrechnungen, Vorschüsse, Exporte, Protokoll).

**Abnahmekriterien**
- Zustandsautomat 3.4 vollständig, inkl. Entsperrung mit Grund und Bezahlt-Zurücknahme; Sperren nach I7 serverseitig erzwungen.
- Export Version n+1 nach Entsperrung, Version n bleibt abrufbar.
- ZIP enthält PDF, JSON (schema-valide), drei CSVs, Archivbelege/Originale, Belegtexte, Protokoll, `MANIFEST.sha256`.
- PDF ist PDF/A-3b, enthält Deckblatt mit Summen und Auszahlungsbetrag, Tagesberechnung, Ausgaben mit MwSt, Belegteil (Fotos), Originale als eingebettete Dateien.
- G19 grün; E2E „Reise → Beleg → Abrechnung → Export → Entsperren → erneut einreichen → bezahlt“.

**Tasks**
1. **Abrechnungs-Modell und Zustandsautomat** – Migrationen, Übergänge, Sperren (I6, I7), Abrechnungsnummer (I13), Audit. *AC:* Tabellengetriebene Übergangstests.
2. **Zeitraum und Vorschlag** – Zeitraumarten, Vorschlag offener Reisen und Vorschüsse, An-/Abwahl. *AC:* Reise gehört zum Zeitraum ihres Enddatums (I6).
3. **Vorschüsse** – CRUD, Verrechnung, Auszahlungsbetrag/Rückzahlung. *AC:* G19, I12.
4. **Prüfung und Einreichen** – Blocker/Warnungen, Quittierung, Snapshot (I11), Export-Job mit `einreichung_laeuft`. *AC:* fehlgeschlagener Export nimmt die Sperre zurück.
5. **Export-Daten** – Snapshot → JSON (Schema), CSV (Format 7.2), ZIP mit Manifest. *AC:* CSV-Golden-Dateien, Schema-Validierung.
6. **Typst-Grundlayout** – Deckblatt, Reiseübersicht, Tagesberechnung, Ausgaben, Belegteil (JPEG-Ableitungen), PDF/A-3b, Originale als eingebettete Dateien. *AC:* `pdfinfo`/`pdfdetach`-Checks im Go-Test.
7. **UI** – Abrechnungsliste, Assistent (Zeitraum → Auswahl → Prüfung → Einreichen), Detail mit Exportversionen, Entsperren/Bezahlt-Dialoge mit Grund, Protokollansicht. *AC:* E2E-Gesamtfluss.

---

## M8 – Export-Feinschliff, Aufbewahrung, Backup (0.8.0)

**Ziel:** Der Export ist prüfungsreif in Deutsch und Englisch, validiert PDF/A-3b, ist reproduzierbar; Aufbewahrungsfristen, Backup und Verfahrensdokumentation sind betriebsbereit.

**Spec:** 5.3, 7.2–7.4, 10.9, 12.2–12.4, 18.6.

**Abnahmekriterien**
- PDF enthält zusätzlich Bewirtungs-Eigenbelege, Eigenbelege, USt-Übersicht (DE/ausländisch), Hinweise/quittierte Warnungen, Rechtsgrundlagen/Satztabellen-Quellen, Protokollseite; englisches Typst-Layout.
- PDF-Belege sind gerastert sichtbar und als Original eingebettet; E-Rechnungs-XML als Platzhalterseite + eingebettete Datei.
- CI-Job `pdfa` validiert alle `export-sample`-Varianten mit veraPDF gegen PDF/A-3b; Bericht als Artefakt.
- Zweimal derselbe Snapshot → byte-identisches PDF/CSV/JSON.
- Fristen-Bericht und Admin-Löschung mit Protokoll; `reisekosten backup` und Litestream-Restore im Betriebshandbuch erprobt.

**Tasks**
1. **PDF-Layout komplett** – Abschnitte 4–8 aus 7.3, Fußzeilen, Barrierearm (Tags, alt), Fonts. *AC:* Review mit Eduard anhand eines echten Monats.
2. **PDF-Belege und E-Rechnungen** – Rasterung per Typst (300 ppi → JPEG), eingebettete Dateien (`pdf.attach`), Platzhalterseite XML. *AC:* `pdfdetach -list` zeigt alle Originale.
3. **Englischer Export** – Typst-Wörterbuch de/en, `export_sprache`. *AC:* `export-sample --lang en`.
4. **Reproduzierbarkeit und `export-sample`** – feste Zeitstempel (`--creation-timestamp`), Beispieldaten-Generator als CLI. *AC:* Hash-Vergleich im Test.
5. **Validator-Job `pdfa`** – veraPDF 1.30.2, Bericht-Artefakt, JSON-Schema-Check. *AC:* bewusst kaputtes PDF lässt den Job fehlschlagen.
6. **Aufbewahrung** – `aufbewahren_bis`, monatlicher Bericht, Admin-Löschung mit Hinweis Ablaufhemmung, Protokoll. *AC:* 12.4.
7. **Backup und Betrieb** – `reisekosten backup`, `doctor`, Litestream-Konfiguration, `docs/betrieb.md` (Restore, Updates, S3 Object Lock), `docs/verfahrensdokumentation.md`. *AC:* Restore-Probe dokumentiert.

---

## M9 – CI-, Security- und README-Feinschliff (1.0.0)

**Ziel:** Lieferkette und Qualitätssicherung auf Release-Niveau; README als Einstieg mit Screenshots; Version 1.0.0.

**Spec:** 13 (Budgets), 18.3, 18.4 (Screenshot-Durchlauf), 18.7 (Signatur, Notes), 18.8, 18.9 (README).

**Abnahmekriterien**
- CodeQL (O5 entschieden), Trivy (blockierend für fixbare CRITICAL/HIGH), govulncheck laufen; Images sind cosign-signiert (keyless) und tragen SBOM- und Provenance-Attestierungen; SBOM liegt als Release-Asset bei.
- Screenshot-Durchlauf: jede Seite auf desktop und mobile lädt, ohne Konsolenfehler, ohne horizontales Scrollen auf mobile; Screenshots als Artefakt; Prüfung, dass jede Route abgedeckt ist.
- Release-Notes enthalten Tags, Digest, Plattformen, `cosign verify`-Befehl.
- README mit Banner, Badges, Screenshot-Tabelle, Quickstart, Disclaimer „keine Steuerberatung“.

**Tasks**
1. **CodeQL** – `codeql.yml` (go, javascript-typescript), Schedule; abhängig von O5. *AC:* Ergebnisse in Code Scanning oder dokumentierte Entscheidung.
2. **Trivy + govulncheck** – Job `scan` (SARIF), Trivy vor dem Push in `release.yml`. *AC:* eingeschleuste verwundbare Testabhängigkeit wird erkannt (einmalig geprüft).
3. **Signatur und Attestierungen** – `provenance: mode=max`, `sbom: true`, cosign keyless, `attest-build-provenance`, SBOM-Asset, `publish-edge` auf `main`. *AC:* `cosign verify` und `docker buildx imagetools inspect --format '{{json .SBOM}}'` funktionieren.
4. **Screenshot-Durchlauf** – `seiten.spec.ts` mit Routenliste + Router-Abgleich, Konsolenfehler-Guard, Scrollbreiten-Check, Artefakt; `mobile-webkit` auf `main`. *AC:* Durchlauf in ≤ 90 s.
5. **Release-Notes** – Abschnitt „Container-Image“ mit Tags, Digest, Plattformen, Image-Größe, `cosign verify`. *AC:* sichtbar in `v1.0.0`.
6. **README** – Banner (`docs/assets/banner.svg`), Badges (CI, Plattformen, Image-Größe; O6 beachten), Screenshot-Tabelle aus `docs/screenshots/` (`pnpm screenshots:readme`), Quickstart, Konfigurationsauszug, Disclaimer. *AC:* README rendert auf GitHub ohne kaputte Bilder.
7. **Budgets und Abschluss** – Bundle-Budget, LCP-Messung, Durchsicht Später-Liste, Glossar-Kandidaten (O9) übernehmen, `1.0.0` über release-please. *AC:* alle Abnahmekriterien M1–M9 erfüllt.
