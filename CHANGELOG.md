# Changelog

## 1.2.2 — 2026-10-11

- Android: switching to another app no longer locks Arthik at once. The lock delay now
  defaults to 5 minutes (choices up to 1 hour) and survives Android stopping the app in
  the background.
- Android: every SMS run reports what it saw ("since 11 Oct 00:00: 12 SMS, 3 from
  banks"); when it sees nothing it says whether older bank SMS are visible or whether
  the phone hides SMS from the app.

## 1.2.1 — 2026-10-11

- **Inbox** (replaces Scheduled): drafts from bank SMS (phone app) and due
  subscriptions (Kronos), in `<finance>/inbox/`. Accept, edit, “same as existing”,
  reject/restore, split a pair. Accounts from the SMS card/account digits (new
  *SMS digits* field on accounts, or tick *Remember* when accepting); categories from
  learned rules → past transactions → Kronos's category → optional local AI → a
  fallback set in Settings. Debit + card payment become one transfer. Duplicate and
  bank-balance checks. Accepting teaches the merchant (`inbox/learned.md`, editable in
  Settings → Learned rules). Dashboard shows the inbox; the menu shows a badge.
- **SMS patterns as data** (`internal/inbox/sms_rules.yaml`, overridable in
  `inbox/sms_rules.md`); `arthik parse-sms` to test one.
- **Optional Ollama** (`ARTHIK_AI_URL`, `ARTHIK_AI_MODEL`): suggests a category from your
  own list for merchants nothing else knows. Off by default; main instance only.
- **Reminders removed** (`reminders.md`, the Scheduled screen and the repeat option);
  recurring bills now come from Kronos. A saved menu order with “scheduled” becomes “inbox”.
- **Android**: nightly copy of new bank SMS into `inbox/sms_<device>.md` at a chosen
  time (only messages after the last run, never older than the day it was switched on,
  at most N days after a missed night); “Allow restricted settings” guide. New lock
  option: fingerprint/face with the phone's screen lock as backup (no Arthik PIN).
- **Logo**: a hand holding a ₹ coin, white on Arthik teal (web, PWA, Android adaptive +
  themed icon).
- Kronos 1.x companion jobs: `arthik_subscriptions` (02:12) and `arthik_inbox_notify`
  (07:00 Telegram notice) — shipped separately in the Kronos repo.

## 1.2.0 — 2026-10-11

- **Remembered transactions**: typing a title in the add form lists matching past
  entries (`GET /api/suggest`); tapping one fills type, amount, accounts, category,
  splits, labels and notes — only the date and time stay. Works on web and phone.
- **Menu order**: Settings → Menu order moves screens up/down; the first four are the
  phone's bottom bar, the rest go under More. Saved as `nav_order` in `settings.md`,
  so every device shows the same order.
- **New logo**: a glowing "A" whose crossbar is a balance beam, on deep teal; new PWA
  icons and Android adaptive icon (with a themed/monochrome layer).
- The app is now called **Arthik** everywhere it is shown (code names stay lowercase).

## 1.1.0 — 2026-10-11

- **Android app** (`android/`): the same UI running offline on the phone, with the
  arthik engine embedded; demo or your Syncthing folder; PIN/fingerprint lock; CSV
  export to Downloads; one-command Docker build of a signed APK.
- **Multi-device sync**: per-transaction `updated` stamps, deletion tombstones, automatic
  merge of Syncthing conflict copies (newest change wins), main/replica roles
  (`ARTHIK_PRIMARY`). Two-device scenario tests in `internal/e2e`.
- **Local mode** for the engine (`ARTHIK_MODE=local`).
- **Motion**: screen transitions, sliding sheets, growing bars and charts, count-up
  totals, skeleton loading; respects "reduce motion".
- Data health separates information (merges) from warnings.
- Compose joins the external `homelab` network instead of publishing a port.

## 1.0.0 — 2026-10-06

- First release of the rewrite: double-entry ledger, Bluecoins-style features, YAML
  `.md` files, read-only demo user, PWA with offline queue.
