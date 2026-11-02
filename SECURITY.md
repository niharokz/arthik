# Security

## Reporting a problem

Please report security issues privately, not in a public issue: open a
**confidential issue** at <https://gitlab.com/niharokz/arthik/-/issues/new>
(tick "This issue is confidential"). Include what you found, how to reproduce it,
and the version (`arthik version`, also at the bottom of the start page).

You should get a reply within a week. Fixes ship as a new version, noted in
`CHANGELOG.md`.

## Supported versions

Only the latest release. Update with `git pull && docker compose up -d --build`.

## What Arthik protects, and how

- **Sign-in:** one user; bcrypt password hash (`ARTHIK_PASSWORD_HASH`), compared in constant
  time, every failure delayed. Five failed sign-ins or wrong device keys from one address in a
  day ban it from every page for `ARTHIK_BAN_DAYS` (30). Bans survive restarts
  (`STATE_PATH/bans.json`); only `arthik unban` lifts them. Client addresses come from proxy
  headers only when the request arrives through a trusted proxy.
- **Sessions:** HMAC-SHA256 signed cookie, `HttpOnly`, `SameSite=Lax`, `Secure` behind HTTPS.
  12 hours, or until sign-out on a device given the device key; changing the key signs those
  devices out.
- **Requests:** every change needs the `X-Arthik` header (blocks cross-site forms); read-only
  users (the demo) can't change anything.
- **Browser:** strict Content-Security-Policy (`script-src 'self'`), no framing, `nosniff`, no
  third-party scripts, fonts or analytics.
- **Container:** runs as your user, read-only root filesystem, all capabilities dropped,
  `no-new-privileges`; it only writes to the data and state folders. Only `ARTHIK_*` variables
  and `TZ` reach it, so a shared `.env` does not leak other services' secrets.
- **Phone app:** no passwords; an app lock (PIN or fingerprint) guards it. The PIN is stored
  hashed and never written to the settings file.

## Your part

- Use a long passphrase, stored as `ARTHIK_PASSWORD_HASH`; set `ARTHIK_SECRET`.
- Keep `.env`, signing keys and patch zips out of git (`.gitignore` and
  `scripts/check-public.sh` help).
- Serve Arthik only over HTTPS, behind a proxy you control.
