# Idea: email receipt intake

Status: idea, not committed. Issue [#25](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/25). Related: [#26](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/26) (API tokens), [#27](https://github.com/BlackDark/vc-reisekostenabrechnung/issues/27) (Web Share Target).

## Goal

Many receipts arrive by email: hotel invoices, train tickets, fuel apps, online shops. Today the user downloads the attachment and uploads it. Goal: forward or redirect the mail, or let the app fetch it, and the attachments land in the **Belegeingang** (unassigned receipts, SPEC §5.2). From there the user assigns each Beleg to a Reise with "Ausgabe anlegen", as today.

## Options

### (a) Scheduled IMAP polling

The app logs in to a mailbox the operator controls and polls a folder.

- Config by ENV: `MAIL_IMAP_HOST`, `PORT`, `USERNAME`, `PASSWORD` or OAuth2, `FOLDER` (default `INBOX`), `INTERVAL` (e.g. `5m`), `PROCESSED_ACTION` (`move:Processed` | `flag` | `delete`).
- Auth: app password for most providers; XOAUTH2 for Gmail and Microsoft 365, which are phasing out basic auth (needs a refresh token and a token endpoint).
- Use IMAP `IDLE` where the server supports it, polling as fallback. Track progress with `UIDVALIDITY` + `UID`, so a restart does not re-import.
- Maps mail to a user by the recipient token (see c) or by a sender allowlist per user.
- Pros: works with any provider, no public endpoint, fits a self-hosted box behind NAT. Cons: credentials in the app, polling delay, one more background job.

### (b) Inbound webhook services

Postmark Inbound, Mailgun Routes, Cloudflare Email Workers or Amazon SES (via SNS/S3) receive the mail and `POST` it to an endpoint such as `POST /api/v1/eingang/mail/{provider}`.

- Each provider has its own payload and signature (Mailgun HMAC, Postmark basic auth, SNS signed messages). One small adapter per provider; prefer providers that pass the raw MIME so the original `.eml` can be kept.
- Pros: instant, no mailbox credentials. Cons: needs a public HTTPS endpoint and a domain with MX records, third-party processor (GDPR: data processing agreement), per-provider code.

### (c) Per-user secret forwarding address

One shared mailbox with plus addressing: `belege+<token>@example.com`. The token is random (≥ 128 bit), shown in the profile, and can be rotated. Works with (a) and (b): the token picks the user.

- Pros: the user just forwards or sets a mail rule ("redirect invoices from DB to …"). Cons: the token is in the address, so anyone who knows it can drop files into that user's Belegeingang; mitigate with a sender allowlist and rotation.

### (d) Web Share Target and API token upload

Not email, but solves the same problem on the phone and for automation.

- The PWA manifest registers a `share_target` (`POST`, `multipart/form-data`, accepts `application/pdf`, `image/*`, `application/xml`). Share from the mail app, banking app or files straight into the Belegeingang. Android Chrome supports it; iOS Safari does not, so iOS uses a Shortcut with the API token.
- Personal API tokens (scope `belege:upload`, revocable, hashed at rest) and `POST /api/v1/belege` with a bearer token. n8n, iOS Shortcuts, a scanner, or a Paperless-style consume folder script can push files.
- Pros: small, no mail infrastructure, base for (a) and (b). Cons: not "just forward the mail".

## Common handling

**Attachments.** Accept PDF, JPEG/PNG/HEIC/WebP, and XRechnung/ZUGFeRD XML (ZUGFeRD is PDF with embedded XML; keep it as PDF). Ignore inline images below a size threshold (logos, signatures). If a mail has no attachment, optionally render the HTML body to PDF as the Beleg (common for online receipts); later step.

**Security.**
- Sender allowlist per user (addresses or domains), check SPF/DKIM/DMARC results from `Authentication-Results` where available.
- Token per user, rotatable; webhook signature check; rate limit per token.
- Size and type limits (reuse the upload limits), sniff the content type, never trust the file name.
- Never load remote content from HTML mails; no link following.
- Archives (`.zip`) off by default; if enabled, limit entry count, total uncompressed size and depth (zip bombs).
- Parse MIME with a hardened library, run in the same rootless container, no new capabilities.

**GoBD.** Keep the original `.eml` and each attachment unchanged, store SHA-256 for both, link the Beleg to its source mail, write an audit event ("received by mail from … at …"). Retention follows the Beleg (**Aufbewahrung** until 31 December of J + 8). The processed mail in the mailbox is not the archive; the app copy is.

**Dedupe.** Reuse the SHA-256 duplicate check from M4 (`GET /belege/duplikate`). Same attachment twice (forward + CC) is imported once; also dedupe on `Message-ID`.

**Landing.** New Belege appear in the Belegeingang with source "email", sender, subject and date. The existing flow assigns them to a Reise. Optional: if AI reading (M6) is enabled for the user, run it and show the suggestions; nothing is booked automatically.

## Recommendation and effort

1. **(d) API tokens + upload endpoint** first: about 2–3 days, reusable by everything else. Web Share Target on top: about 1 day.
2. **(a) IMAP polling with (c) plus-address tokens**: about 4–6 days incl. `.eml` storage, allowlist, dedupe, ENV docs and e2e test against a GreenMail/Dovecot container. Fits self-hosting best.
3. **(b) webhook adapters** only on demand: about 1–2 days per provider.

XOAUTH2 for Gmail/Microsoft adds about 2 days and can follow later; app passwords cover the first release.
