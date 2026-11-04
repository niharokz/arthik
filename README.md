# Arthik

[![pipeline](https://gitlab.com/niharokz/arthik/badges/main/pipeline.svg)](https://gitlab.com/niharokz/arthik/-/pipelines)
[![release](https://gitlab.com/niharokz/arthik/-/badges/release.svg)](https://gitlab.com/niharokz/arthik/-/releases)
[![Android APK](https://img.shields.io/badge/Android-APK-3a807a)](https://gitlab.com/niharokz/arthik/-/releases/permalink/latest/downloads/arthik.apk)

**Self-hosted, double-entry personal finance over plain YAML files.** The everyday feel of a
phone money tracker (accounts, categories, budgets, reports) with every transaction balanced,
plus an **inbox** that turns bank SMS and due subscriptions into drafts you accept with one tap.

- **Web app (PWA)** with offline adds, and an **Android app** that runs fully offline over the folder Syncthing keeps in sync ([android/README.md](android/README.md), [download the APK](https://gitlab.com/niharokz/arthik/-/releases/permalink/latest/downloads/arthik.apk)).
- **Plain files.** Data is YAML in `.md` files, readable in Obsidian or any editor; edits made there are picked up within seconds.
- **One small Go binary**, one container, no database, no third-party services, no tracking.
- **Your home screen**: choose, order and create the blocks you see.
- **Private by default**: one user, IP bans after failed sign-ins, signed sessions, an optional read-only demo login.

## Features

| Area | What you get |
|---|---|
| Accounts | Bank, cash, wallet, investment, deposit, property, receivable, credit card, loan, payable … Balance (today) and projected balance (incl. future-dated). Credit cards: limit, available, statement/due dates, billed/unbilled. Hide, archive, leave out of net worth (their total is shown as *not counted*). Bank balance check: record what the bank shows and see ✅ or the difference. |
| Transactions | Expense, income, transfer, splits (category lines, and account lines such as a loan's principal), refunds. Category picker with search, recent and group grid. Calculator amount field (`120+45*2`). Remembered transactions: type a few letters of a title and tap a past entry to take its title and category (the account and everything else stay as you set them). Labels, notes, time. Every saved transaction has happened; past dates fix history; future dates count from their day. Duplicate, bulk label/delete. |
| Categories | Two levels (parent › child) for expense, income, and “both ways” (e.g. *Market* gains and losses). Totals for a week, month, quarter, half-year, year or any custom range; the default period is a setting. Delete with “move transactions to …”. |
| Budgets | Per category, weekly → yearly, scaled to the period you view, reset every period, with on-pace progress. |
| Inbox | Drafts from bank SMS (the phone app). Accept, edit, “same as” an entry you already made, or reject. Accounts are found from the card/account digits in the SMS; the category from what you taught it (🧠), a past transaction with that name (↺), the local AI (✨, optional) or your fallback. A debit plus a matching card payment become one transfer. The bank's reported balance is checked against yours. Accepting teaches the merchant. |
| Notifications | 🔔 Reminders from Kronos (bills and subscriptions due). Never a transaction by themselves: each shows *Paid ✓* when a payment with the same amount is recorded near its due date, or that its SMS waits in the inbox. Dismiss, reopen, or add the payment. |
| Plugins | Personal add-ons switched on by their own data file. **Loans** (`loans.md`): lender summary, repayment schedule (paste the lender's table each year; estimate to payoff), EMIs left, interest still to pay, this financial year's principal and interest, *Record EMI* and *Split as EMI*. |
| Reports | Cash flow, spending and income by category with drill-down, category trend, net worth over time, balance sheet at any date, account flow, labels, calendar. Every chart has a table. |
| Search & data | Full-text search plus filters (dates, type, accounts, categories, labels, amount, posted/future); totals of the filtered list; CSV export of exactly what you filtered; CSV import with a dry-run preview. |
| Home | Your blocks, in your order: net worth with its trend, the period's income/expense/savings, inbox, spending, budgets, upcoming, credit cards, accounts, recent, cash flow, plus blocks you create for any account, category or label. ✎ on the home screen. |
| Settings | Default period, week start, month start day (salary cycles), year start month (e.g. April for the Indian FY), look-ahead days, light/dark/auto, hide amounts, menu order (which four screens sit in the phone's bottom bar), inbox fallback categories, learned rules. |

## Quick start (Docker)

```sh
git clone https://gitlab.com/niharokz/arthik.git && cd arthik
cp .env.example .env                       # fill in FINANCE_PATH, PUID/PGID, ARTHIK_USER, ARTHIK_SECRET …
mkdir -p state                             # STATE_PATH, writable by PUID:PGID
docker compose build
docker compose run --rm -it arthik hash-password   # → ARTHIK_PASSWORD_HASH='$2a$12$…' in .env (single quotes)
docker compose up -d
```

The container joins the Docker network `ARTHIK_NETWORK` (`homelab`), where your reverse proxy
reaches it as `arthik:8080`: `/` is the start page (demo login and APK link), `/app` the app.
For a quick local test add `ports: ["8080:8080"]`, or run `sh scripts/dev.sh` (needs Go).

## Configuration (`.env`)

| Variable | Used by | Meaning |
|---|---|---|
| `FINANCE_PATH` | compose | Host folder with the data files (mounted at `/data/finance`). |
| `STATE_PATH` | compose | Host folder for server-only files (`bans.json`), mounted at `/state`. Not synced. |
| `PUID`, `PGID` | compose | User the container runs as (must own `FINANCE_PATH` and `STATE_PATH`). |
| `ARTHIK_NETWORK` | compose | Docker network of your reverse proxy (`homelab`). |
| `TZ` | app | Time zone that decides “today”. |
| `ARTHIK_USER`, `ARTHIK_PASSWORD_HASH` | app | Your sign-in; the hash comes from `arthik hash-password`. `ARTHIK_PASSWORD` (plain text) also works. |
| `ARTHIK_SECRET` | app | Signs session cookies; set it so you stay signed in across restarts. |
| `ARTHIK_SESSION_HOURS` | app | How long a sign-in lasts without the device key (12). |
| `ARTHIK_DEVICE_KEY` | app | Typed once on a device (sign-in screen or Settings → This device): that device stays signed in until it signs out. Change it to sign all such devices out. ≥ 24 characters, e.g. `openssl rand -hex 32`. |
| `ARTHIK_LOGIN_MAX_FAILS`, `ARTHIK_BAN_DAYS` | app | Failed sign-ins (or wrong device keys) within a day before an address is banned from the whole site (5), and for how long (30 days). |
| `ARTHIK_BAN_EXEMPT` | app | Comma-separated CIDRs never banned (empty). |
| `ARTHIK_TRUSTED_PROXIES` | app | Proxies whose `CF-Connecting-IP` / `X-Forwarded-For` name the client (private ranges). |
| `ARTHIK_DEMO_USER`, `ARTHIK_DEMO_PASSWORD` | app | Optional read-only demo login, shown on the start page (data in `<FINANCE_PATH>/demouser`, regenerated monthly). Both empty = no demo. |
| `ARTHIK_WATCH_SECONDS` | app | How often to look for edits made by other apps (default 3). |
| `ARTHIK_PRIMARY` | app | `true` (default): this instance writes computed files, imports the inbox and merges Syncthing conflict copies. Set `false` on any second instance sharing the folder. |
| `ARTHIK_INBOX_MAX_AGE_DAYS`, `ARTHIK_INBOX_KEEP_DAYS`, `ARTHIK_INBOX_PAIR_DAYS` | app | Inbox: ignore older source items (30), forget reviewed drafts (120), pair a debit with a matching credit within N days (2). |
| `ARTHIK_INBOX_RESCAN_MINUTES` | app | Re-read the inbox files at least this often, even when no file change was noticed (10). |
| `ARTHIK_AI_URL`, `ARTHIK_AI_MODEL`, `ARTHIK_AI_TIMEOUT_SECONDS` | app | Optional Ollama that suggests categories for new merchants, e.g. `http://ollama:11434` and `qwen2.5:7b-instruct-q4_K_M`. Empty = off. |

`docker-compose.yml` passes only these variables (and `TZ`) into the container, so `.env` can be shared with other services. The phone app runs the same engine with `ARTHIK_MODE=local` (set by the app).

## Sign-in and lock-out

- A sign-in lasts `ARTHIK_SESSION_HOURS` (12). On your own devices, enter the device key
  once (sign-in screen → *Device key*, or Settings → *This device*); that device then stays
  signed in until you sign out. A new key in `.env` signs them all out.
- Five failed sign-ins or wrong device keys from one address within a day ban it from
  every page for `ARTHIK_BAN_DAYS`. Bans survive restarts (`STATE_PATH/bans.json`) and are
  lifted only by you: `docker exec arthik arthik bans`, `docker exec arthik arthik unban <ip|all>`.
- Behind Caddy + Cloudflare Tunnel the client address is read from `CF-Connecting-IP` /
  `X-Forwarded-For`, and only when the request comes from a trusted proxy, so a client
  can't choose its own address.

## Several devices on one folder

The server and the phone share the folder through Syncthing. Each transaction records when
it last changed (`updated`) and a deletion leaves a tombstone (`deleted:`), so when both
sides changed the same month file while apart, the main instance merges Syncthing's
`.sync-conflict-…` copy by id — newest change wins — and removes the copy. Definition
files (accounts, categories, labels) merge by id with the main copy winning; inbox
drafts and learned rules merge by id, newest change winning.
Only the main instance writes balances and imports the inbox.

## Data files

```
<FINANCE_PATH>/
  settings.md                     preferences
  accounts.md                     accounts + computed balance / projected_balance
  categories.md                   category tree + budgets + computed total for the default period
  labels.md                       labels + computed count
  money.md                        computed overview (net worth, period totals, ledger check) — output only
  loans.md                        optional, yours: the Loans plugin (read only by Arthik)
  transactions/transaction_YYYYMM.md   one file per month
  inbox/drafts.md                 drafts waiting for review (and reviewed ones, for a while)
  inbox/learned.md                merchant → title/category rules learned when you accept
  inbox/kronos.md                 written only by Kronos: bills that fell due (shown as reminders)
  inbox/sms_<phone>.md            written only by the phone app: new bank SMS, raw text
  inbox/sms_rules.md              optional: your own SMS patterns (see below)
  demouser/                       the demo dataset (same layout)
```

A transaction is a list of postings that must sum to zero (`+` debit, `-` credit):

```yaml
transactions:
  - id: t-20261001-55c7e6
    date: '2026-10-01'
    type: expense            # expense | income | transfer | journal
    title: Groceries
    labels: [home]
    postings:
      - account: wallet
        amount: '-540.00'
      - category: food-groceries
        amount: '540.00'
```

**Transactions are the only source of truth.** Every balance and total in the other
files is recomputed after each change and rewritten only when it actually changed.
You can edit any file by hand; Arthik notices within seconds, repairs what it safely
can (missing ids, a transaction saved in the wrong month file, labels that don't exist
yet) and rebuilds everything. A file that does not parse is never overwritten — changes
are paused and the app shows the file and line until it is fixed. Entries that don't
balance or point at unknown accounts are listed as problems and left out of totals.
Unknown keys you add to any record are kept.

## Inbox

Producers each own one file in `inbox/`, so no two programs ever write the same file.
The main instance adds a draft for every new item (ids are stable, so nothing is
drafted twice; items older than `ARTHIK_INBOX_MAX_AGE_DAYS` are ignored):

| Step | When | Run it now |
|---|---|---|
| Phone copies bank SMS → `inbox/sms_<device>.md` | daily, time set in the app (Settings → Phone) | *Copy now* (app) |
| SMS → drafts | *as soon as they arrive*, or *every day at HH:MM* (`inbox_sms_time`, set on the Inbox screen or in Settings) | *Run now* |
| Kronos subscriptions → drafts | as soon as Kronos writes `inbox/kronos.md` | *Run now* |

"As soon as" means within seconds of the file changing, re-checked at least every
`ARTHIK_INBOX_RESCAN_MINUTES` (10) in case a change was missed. A daily SMS import
missed while the server was down runs once when it is back. Review drafts on the **Inbox** screen:

- **Accept** creates the transaction (id `d-<draft>`, so accepting on the phone and
  on the server still makes one) and remembers the merchant in `inbox/learned.md`.
- **Edit** opens the normal form (with the original SMS above it); saving accepts.
  Tick *Remember …4111* and the account keeps those SMS digits.
- **Same — don't add** links the draft to a transaction you already entered.
- **Reject** sets it aside (*Done* tab → *Restore*). **Not a transfer** splits a pair.

**SMS patterns** live in `internal/inbox/sms_rules.yaml`: card spends, card payments and
account debits/credits in the wording HDFC, SBI, YES and IDFC use (many banks phrase
them the same way), plus a catch-all whose drafts are marked “check”. Add or
override patterns for your banks in `<FINANCE_PATH>/inbox/sms_rules.md` — same layout,
tried first, `disable: [name]` switches a built-in off. Test one with:

```sh
docker exec arthik arthik parse-sms "Spent Rs.2000 On HDFC Bank Card 4111 At GYFTR On 2026-10-09:20:34:36"
```

## Command line

```
arthik serve            run the web app (default)
arthik hash-password    print a bcrypt hash for ARTHIK_PASSWORD
arthik check [--demo]   validate the data files and list problems
arthik rebuild          re-read everything and rewrite all computed values
arthik demo-seed        regenerate the demo data now
arthik health           exit 0 when the server answers (Docker health check)
arthik summary          this month's spending vs budget and the inbox, as JSON (read-only)
arthik parse-sms TEXT   show how the inbox would read an SMS
arthik bans             list banned addresses (and until when)
arthik unban IP|all     lift a ban; the running server notices at once
arthik version
```

In Docker: `docker exec arthik arthik check`.

## Releases and the Android APK

A release is a tag. Write the `## X.Y.Z` section of [CHANGELOG.md](CHANGELOG.md) first, then:

```sh
sh scripts/release.sh "v1.0.2 customizable home, inbox bell"
```

It sets `VERSION`, adds a changelog section if there is none, runs the tests and
`scripts/check-public.sh`, commits, tags and pushes. GitLab CI ([.gitlab-ci.yml](.gitlab-ci.yml))
then tests on a shared runner, builds the **signed APK on your own runner** (no CI minutes
used, the key never leaves your server), uploads it to the package registry and creates the
release with that version's notes. The newest APK is always at
`/-/releases/permalink/latest/downloads/arthik.apk`, linked from the start page.

Set up the runner once, on the machine with Docker and the signing key:

1. Install [GitLab Runner](https://docs.gitlab.com/runner/install/linux-repository/) to run as **your user** (it needs Docker and `$KEYS_HOME/.arthik-keys`):
   `sudo gitlab-runner uninstall && sudo gitlab-runner install --user "$USER" --working-directory "$HOME/.gitlab-runner" && sudo gitlab-runner start`
2. GitLab → the project → **Settings → CI/CD → Runners → New project runner**: tag `arthik-apk`, untick *Run untagged jobs*. Then `sudo gitlab-runner register --url https://gitlab.com --token <token> --executor shell`.
3. Put the keystore password next to the key:
   `printf 'ARTHIK_KEYSTORE_PASSWORD=%s\n' '<password>' > "$KEYS_HOME/.arthik-keys/arthik.env" && chmod 600 "$KEYS_HOME/.arthik-keys/arthik.env"`

A local build works the same way: `./android/build.sh` (all phones) or `./android/build.sh quick`
(64-bit phones, no tests). Install `scripts/check-public.sh` as a pre-push hook to keep `.env`,
keys, patch zips and words listed in `.private-words` out of the public repo:
`ln -s ../../scripts/check-public.sh .git/hooks/pre-push`.

## Development

```sh
go vet ./... && go test ./...
sh scripts/dev.sh            # http://localhost:8080 — sign in as me / dev, or the demo
```

Go 1.23+, dependencies: `gopkg.in/yaml.v3`, `golang.org/x/crypto/bcrypt`. The front end
is plain ES modules and CSS embedded in the binary — no build step, no CDN.

```
cmd/arthik/          the binary: serve, CLI commands
internal/book/       data files: load, validate, compute, write, merge Syncthing conflicts
internal/feature/…   one package per screen's API (accounts, transactions, review = inbox, …)
internal/inbox/      SMS + Kronos drafts, learned rules, confidence
internal/auth/       users, sessions, device key       internal/guard/  IP bans
web/static/          the app (ES modules, CSS) and the start page
android/             the Android app (Kotlin WebView shell + the same engine)
```

Issues and merge requests use the templates in `.gitlab/`. Security reports: [SECURITY.md](SECURITY.md).

## Licence

MIT
