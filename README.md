<p align="center">
  <img src="docs/assets/banner.svg" alt="Reisekostenabrechnung" width="640">
</p>

<p align="center">
  <a href="https://github.com/BlackDark/vc-reisekostenabrechnung/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/BlackDark/vc-reisekostenabrechnung/actions/workflows/ci.yml/badge.svg?branch=main"></a>
  <img alt="Plattformen" src="https://img.shields.io/badge/platform-linux%2Famd64%20%7C%20linux%2Farm64-blue">
  <img alt="Image-Größe" src="https://img.shields.io/badge/image-siehe%20Release--Notes-lightgrey">
</p>

# Reisekostenabrechnung

Selbst gehostete Web-App (PWA) für die Reisekostenabrechnung nach deutschem Steuerrecht: Reisen erfassen, Belege per Handykamera aufnehmen, Verpflegungsmehraufwand und Kilometer automatisch berechnen und eine prüfbare Abrechnung als PDF/A mit allen Belegen beim Arbeitgeber einreichen. Deutsch und Englisch, mehrere Nutzer, Login per Passwort, OIDC (z. B. Pocket ID) oder Reverse-Proxy-Header.

> **Status:** in Planung – siehe [Meilensteine](docs/MILESTONES.md). Repo: `git@github.com:BlackDark/vc-reisekostenabrechnung.git` (privat).

## Screenshots

| | Desktop | Mobil |
|---|---|---|
| Reisen | _folgt mit M9_ | _folgt mit M9_ |
| Beleg erfassen | _folgt mit M9_ | _folgt mit M9_ |
| Abrechnung | _folgt mit M9_ | _folgt mit M9_ |

<!-- Ab M9 ersetzen durch z. B. ![Reisen, Desktop](docs/screenshots/reisen-desktop.png) / ![Reisen, Mobil](docs/screenshots/reisen-mobile.png) -->

Die Bilder entstehen im Screenshot-Durchlauf der CI (Artefakt `screenshots`) und werden mit `pnpm screenshots:readme` übernommen – ab Meilenstein M9.

## Schnellstart

Voraussetzung: Docker mit Compose v2. Solange Repo und Paket privat sind, brauchst du ein GitHub-Token mit `repo` bzw. `read:packages`.

```sh
mkdir reisekosten && cd reisekosten
export GH_TOKEN=…   # nur solange das Repo privat ist
for f in docker-compose.yml .env.example; do
  curl -fsSL -H "Authorization: Bearer $GH_TOKEN" -H "Accept: application/vnd.github.raw" \
    -o "$f" "https://api.github.com/repos/BlackDark/vc-reisekostenabrechnung/contents/deploy/$f"
done
cp .env.example .env            # Werte eintragen, mindestens APP_BASE_URL
echo "$GH_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
docker compose up -d
docker compose logs app | grep -i setup   # einmaliger Setup-Token für den ersten Admin
```

Danach `APP_BASE_URL` im Browser öffnen (TLS terminiert dein Reverse Proxy). Fehlende Pflichtwerte meldet Compose mit `… fehlt`. Backup per Litestream: zusätzlich `docker-compose.backup.yml` und `litestream.yml` laden und `docker compose -f docker-compose.yml -f docker-compose.backup.yml up -d`.

Alle Einstellungen (OIDC, Header-Auth, S3, KI, …) stehen in [SPEC.md, Abschnitt 11](docs/SPEC.md#11-konfiguration-umgebungsvariablen).

## Dokumentation

- [Spezifikation](docs/SPEC.md) – Domänenmodell, Rechenregeln, API, Sicherheit, Betrieb, CI/CD
- [Meilensteine](docs/MILESTONES.md)
- [Glossar](GLOSSARY.md) und [Architekturentscheidungen](docs/adr/)
- Recherche: [Steuerrecht](docs/research/steuer-reisekosten.md), [Stack](docs/research/stack.md), [Belegkompression](docs/research/beleg-kompression.md)

## Entwicklung

Go 1.27, Node 24 + pnpm, Svelte 5, SQLite; Lint mit golangci-lint und Biome. `make dev` startet Backend und Vite-Devserver, `make check` führt alle Prüfungen aus (ab M1). Commits und PR-Titel folgen [Conventional Commits](https://www.conventionalcommits.org/de/); Releases erstellt release-please.

## Hinweis

**Keine Steuerberatung.** Die App rechnet nach bestem Wissen mit den veröffentlichten Pauschalen und Regeln (Stand siehe Satztabellen), ersetzt aber weder die Prüfung durch den Arbeitgeber noch eine steuerliche Beratung. Für die Richtigkeit einer Abrechnung bleibt der Nutzer verantwortlich.
