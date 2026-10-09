<p align="center">
  <img src="docs/assets/banner.svg" alt="Reisekostenabrechnung" width="640">
</p>

<p align="center">
  <a href="https://github.com/BlackDark/vc-reisekostenabrechnung/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/BlackDark/vc-reisekostenabrechnung/actions/workflows/ci.yml/badge.svg?branch=main"></a>
  <img alt="Platforms" src="https://img.shields.io/badge/platform-linux%2Famd64%20%7C%20linux%2Farm64-blue">
  <img alt="Image size" src="https://img.shields.io/badge/image-see%20release%20notes-lightgrey">
</p>

# Reisekostenabrechnung

Self-hosted web app for a German **Reisekostenabrechnung** (travel expense claim). Record a business trip, capture a **Beleg** (receipt), apply the meal and mileage allowances, and file an **Abrechnung** the employer can review: a PDF/A-3b with every receipt attached.

German and English. Each person sees only their own trips. [Features](docs/features.md) lists what the app covers.

## Screenshots

Desktop frames are 1440×900. Mobile frames are 390×844. How to refresh them is in [Development](docs/development.md).

| | Desktop | Mobile |
|---|---|---|
| Trips | <img alt="Trips, desktop, dark" src="docs/assets/screenshots/desktop/reisen.png" width="360"> | <img alt="Trips, mobile, dark" src="docs/assets/screenshots/mobile/reisen.png" width="180"> |
| A trip, with allowances | <img alt="Trip to Paris, desktop, dark" src="docs/assets/screenshots/desktop/reise-detail.png" width="360"> | <img alt="Trip to Paris, mobile, dark" src="docs/assets/screenshots/mobile/reise-detail.png" width="180"> |
| Capture a Beleg | <img alt="Receipt capture, desktop, dark" src="docs/assets/screenshots/desktop/beleg-neu.png" width="360"> | <img alt="Receipt capture, mobile, dark" src="docs/assets/screenshots/mobile/beleg-neu.png" width="180"> |
| Abrechnung | <img alt="Expense claim, desktop, dark" src="docs/assets/screenshots/desktop/abrechnung-detail.png" width="360"> | <img alt="Expense claim, mobile, dark" src="docs/assets/screenshots/mobile/abrechnung-detail.png" width="180"> |
| Trips, light theme | <img alt="Trips, desktop, light" src="docs/assets/screenshots/desktop/reisen-light.png" width="360"> | <img alt="Trips, mobile, light" src="docs/assets/screenshots/mobile/reisen-light.png" width="180"> |
| Sign-in, light theme | <img alt="Sign-in, desktop, light" src="docs/assets/screenshots/desktop/login-light.png" width="360"> | <img alt="Sign-in, mobile, light" src="docs/assets/screenshots/mobile/login-light.png" width="180"> |

## Quickstart

Docker with Compose v2. The repository is public, so the download does not need a token.

```sh
mkdir reisekosten && cd reisekosten
for f in docker-compose.yml .env.example; do
  curl -fsSL -o "$f" "https://raw.githubusercontent.com/BlackDark/vc-reisekostenabrechnung/main/deploy/$f"
done
cp .env.example .env            # set at least APP_BASE_URL
docker compose up -d
docker compose logs app | grep -i setup
```

Open `APP_BASE_URL`. The log line is the setup token for the first admin, until a user exists. [Installation](docs/installation.md) covers Litestream and the rest.

## Further reading

- [Features](docs/features.md)
- [Installation](docs/installation.md)
- [Configuration](docs/configuration.md), the environment variables
- [Authentication](docs/authentication.md), password, OIDC with Pocket ID, trusted header
- [Operations](docs/operations.md), backup, restore, retention
- [Development](docs/development.md)
- [Tax rules, in short](docs/tax-rules.md)
- [Specification](docs/SPEC.md), [milestones](docs/MILESTONES.md), [glossary](GLOSSARY.md), [architecture decisions](docs/adr/)

## Disclaimer

**Not tax advice.** The app applies the published allowances. It does not replace the employer's review or a tax adviser. You remain responsible for the correctness of an Abrechnung. [Tax rules, in short](docs/tax-rules.md).
