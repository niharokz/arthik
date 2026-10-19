# Changelog

## 1.3.5 — 2026-10-19

- *When drafts arrive*: step 2 (*SMS become drafts*) now always has its **Run now**
  button beside it on the main device, like *Copy now* in step 1. It was hidden for
  the first minute after a restart (the screen only showed it once an import had run).

## 1.3.4 — 2026-10-19

- **Late phone sync no longer costs a day.** With *Every day at HH:MM*, an SMS file the
  phone wrote before that time but that synced after it (Syncthing on the phone often
  waits until morning) is now read as soon as it lands, instead of at the next day's
  run. A capture made after the daily run still waits for it (or *Run now*).
- *Run now* with nothing new says when the phone's SMS file on this device was
  written, so a file that hasn't synced yet is easy to spot.

## 1.3.3 — 2026-10-14

- **Two inbox timings**, shown together in a new *When drafts arrive* card (Inbox
  screen and Settings):
  1. **Phone copies bank SMS** → `inbox/sms_<device>.md` at the time set in the app
     (Settings → Phone). The file now records that time and the last run
     (`schedule:`, `last_run:`), so every device can show it. *Copy now* in the app.
  2. **SMS become drafts** — new setting `inbox_sms_time` (settings.md): *As soon as
     they arrive* (as before) or *Every day at HH:MM*. If nimory was down at that time,
     it runs once when it is back. The time of the last SMS import is kept in
     `drafts.md` (`sms_imported:`). *Make drafts now* imports at once, whatever the time.
- Picking a remembered transaction while typing a title fills everything **except
  the amount**, which stays what you typed or what the SMS said.
- Kronos subscription drafts are unchanged: imported as soon as Kronos writes them.
- Android build: `./android/build.sh quick` (no tests, 64-bit phones only), `ARTHIK_ABIS`
  to choose phone CPUs, parallel Gradle, and each step's time printed.

## 1.3.2 — 2026-10-14

- **Inbox never waits for a file change it missed:** besides reacting to changed SMS
  and Kronos files, the main device now re-reads them every 10 minutes
  (`ARTHIK_INBOX_RESCAN_MINUTES`), and the change check also compares file sizes.
  Importing is idempotent, so a re-read never makes duplicates.
- Inbox screen: "Checked for new SMS and Kronos drafts 3 min ago · **Check now**"
  (`POST /api/inbox/import`).
- When a data file can't be read, the inbox import says so (log line and the inbox
  banner) instead of pausing silently.

## 1.3.1 — 2026-10-12

- **Android widgets:** three new 2×2 widgets next to the 4×2 overview — **Budget**
  (spent, budget, what's left, bar), **Net worth** (now, this month's change, a
  12-month trend line) and **Inbox** (drafts waiting, money out, the newest one).
  Each opens its screen when tapped.
- **Widget look** (Settings → Phone → Widgets): Auto, Light, Dark, AMOLED, Glass,
  Clear or any colour, with opacity 0–100% and light/dark/auto text, previewed live.
- The summary (`/api/summary`, `arthik summary`) now carries net worth, its change
  this month and the last 12 month-ends.
- **Web:** quieter background grid and softer cards; segmented buttons slide a thumb
  to the chosen option; the top bar gets its edge only once you scroll; the + button
  tucks away while scrolling down; amounts count to their new value when the period
  changes; cards lift on hover; the loading skeleton only shows on slow screens;
  springier switches.

## 1.3.0 — 2026-10-12

- **Overview:** net worth is now the headline card with a trend line you can switch
  between Day (30 days), Week (12 weeks), Month (12 months) and Year (5 years), and
  the change for today / this week / month / year. Income, expense and savings sit
  below as a slim row. Reports gained a daily bucket (`/api/reports/networth?bucket=daily`).
- **Inbox cards:** each draft shows how sure Arthik is — *Ready* (learned rule or past
  transaction, nothing off), *Worth a look* (Kronos or AI category, or a warning),
  *Check first* (fallback category, unknown account, possible duplicate) — and how
  the category was chosen. The original SMS folds open under the card. *Accept N
  ready* accepts every "Ready" draft at once. Cards slide away when accepted or rejected.
- **Colours:** accounts, categories and labels get 12 ready colours, a hex box
  (`#e91e63`, `e91e63` or `e1e`) and the system picker.
- **Learned rules** load 25 at a time, most used first, with search
  (`/api/inbox/rules?q=&offset=&limit=` now returns `{items, total, offset, limit}`).
- New `GET /api/summary` and `arthik summary` (read-only JSON): this month's spending
  against budgets and the inbox at a glance.
- **Android:** home-screen widget (month spend vs budget + inbox, tap → Inbox);
  morning inbox notification at a time you choose (replaces Kronos's Telegram notice
  if you like); edge-to-edge screens with bar icons following light/dark; optional
  Material You colours; launch screen matching Android's splash; smoother start-up,
  a sliding pill in the bottom bar and pull-down-to-close sheets. Widget and
  notification taps open the right screen (`/local/enter?to=inbox`).

## 1.2.3 — 2026-10-12

- Inbox AI: drafts that carry their own category name (e.g. Kronos's `utility`) pass it
  to the model, so a bill is no longer guessed as food when no category matches by name.

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
