# Arthik

Self-hosted, **double-entry** personal finance — the everyday feel of a phone money
tracker (accounts, categories, budgets, reports) with every transaction balanced:
debited somewhere, credited somewhere else — plus an **inbox** that turns bank SMS
and due subscriptions into drafts you accept with one tap.

- Mobile-first **PWA**: install it on your phone, add transactions **offline**, they sync when you are back.
- Data is plain **YAML in `.md` files** — readable in Obsidian or any editor, and edits made there are picked up automatically.
- One static Go binary, one container, no database, no third-party services, no tracking.
- A built-in, read-only **demo user** with generated data.
- An **Android app** with the same screens, running fully offline on the phone against the Syncthing-synced folder, which also copies new bank SMS into the inbox every night — see [android/README.md](android/README.md).

## Features

| Area | What you get |
|---|---|
| Accounts | Bank, cash, wallet, investment, deposit, property, receivable, credit card, loan, payable … Balance (today), cleared balance, projected balance (incl. future-dated). Credit cards: limit, available, statement/due dates, billed/unbilled. Hide, archive, leave out of net worth. Reconcile against a statement. |
| Transactions | Expense, income, transfer, split across categories, refunds. Calculator amount field (`120+45*2`). Remembered transactions: type a few letters of a title and tap a past entry to fill in everything. Labels, notes, time, status (uncleared/cleared/reconciled). Past dates fix history; future dates count from their day. Duplicate, bulk status/label/delete. |
| Categories | Two levels (parent › child) for expense, income, and “both ways” (e.g. *Market* gains and losses). Totals for a week, month, quarter, half-year, year or any custom range; the default period is a setting. Delete with “move transactions to …”. |
| Budgets | Per category, weekly → yearly, scaled to the period you view, reset every period, with on-pace progress. |
| Inbox | Drafts from bank SMS (the phone app) and due subscriptions (Kronos). Accept, edit, “same as” an entry you already made, or reject. Accounts are found from the card/account digits in the SMS; the category from what you taught it (🧠), a past transaction with that name (↺), the category Kronos named (📋), the local AI (✨, optional) or your fallback. A debit plus a matching card payment become one transfer. The bank's reported balance is checked against yours. Accepting teaches the merchant. |
| Reports | Cash flow, spending and income by category with drill-down, category trend, net worth over time, balance sheet at any date, account flow, labels, calendar. Every chart has a table. |
| Search & data | Full-text search plus filters (dates, type, accounts, categories, labels, status, amount, posted/future); totals of the filtered list; CSV export of exactly what you filtered; CSV import with a dry-run preview. |
| Settings | Default period, week start, month start day (salary cycles), year start month (e.g. April for the Indian FY), look-ahead days, light/dark/auto, hide amounts, menu order (which four screens sit in the phone's bottom bar), inbox fallback categories, learned rules. |

## Quick start (Docker)

```sh
git clone https://gitlab.com/niharokz/arthik.git && cd arthik
cp .env.example .env        # set FINANCE_PATH, PUID/PGID, ARTHIK_USER, ARTHIK_PASSWORD, ARTHIK_SECRET
docker compose up -d --build
```

The container joins the external Docker network `homelab`, where the reverse proxy
reaches it as `arthik:8080` (`/` is the public about page, `/app` is the app). Use another
network name in `docker-compose.yml` if yours differs, or add a `ports:` entry to reach
it directly. The demo user and password are shown on the sign-in screen.

To store a bcrypt hash instead of a plain password:

```sh
docker compose run --rm -it arthik hash-password
# paste the result into .env in single quotes: ARTHIK_PASSWORD='$2a$12$…'
```

## Configuration (`.env`)

| Variable | Used by | Meaning |
|---|---|---|
| `FINANCE_PATH` | compose | Host folder with the data files (mounted at `/data/finance`). |
| `PUID`, `PGID` | compose | User the container runs as (must own `FINANCE_PATH`). |
| `TZ` | app | Time zone that decides “today”. |
| `ARTHIK_USER`, `ARTHIK_PASSWORD` | app | Your sign-in; the password may be a bcrypt hash. |
| `ARTHIK_SECRET` | app | Signs session cookies; set it so you stay signed in across restarts. |
| `ARTHIK_DEMO`, `ARTHIK_DEMO_USER`, `ARTHIK_DEMO_PASSWORD` | app | Read-only demo user (data in `<FINANCE_PATH>/demouser`, regenerated monthly). |
| `ARTHIK_WATCH_SECONDS` | app | How often to look for edits made by other apps (default 3). |
| `ARTHIK_PRIMARY` | app | `true` (default): this instance writes computed files, imports the inbox and merges Syncthing conflict copies. Set `false` on any second instance sharing the folder. |
| `ARTHIK_INBOX_MAX_AGE_DAYS`, `ARTHIK_INBOX_KEEP_DAYS`, `ARTHIK_INBOX_PAIR_DAYS` | app | Inbox: ignore older source items (30), forget reviewed drafts (120), pair a debit with a matching credit within N days (2). |
| `ARTHIK_INBOX_RESCAN_MINUTES` | app | Re-read the inbox files at least this often, even when no file change was noticed (10). |
| `ARTHIK_AI_URL`, `ARTHIK_AI_MODEL`, `ARTHIK_AI_TIMEOUT_SECONDS` | app | Optional Ollama that suggests categories for new merchants, e.g. `http://ollama:11434` and `qwen2.5:7b-instruct-q4_K_M`. Empty = off. |

The phone app runs the same engine with `ARTHIK_MODE=local` (set by the app; not used on the server).

## Several devices on one folder

nimory and the phone share the folder through Syncthing. Each transaction records when
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
  accounts.md                     accounts + computed balance / cleared_balance / projected_balance
  categories.md                   category tree + budgets + computed total for the default period
  labels.md                       labels + computed count
  money.md                        computed overview (net worth, period totals, ledger check) — output only
  transactions/transaction_YYYYMM.md   one file per month
  inbox/drafts.md                 drafts waiting for review (and reviewed ones, for a while)
  inbox/learned.md                merchant → title/category rules learned when you accept
  inbox/kronos.md                 written only by Kronos: subscriptions that fell due
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
    status: cleared          # uncleared | cleared | reconciled
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
  on nimory still makes one) and remembers the merchant in `inbox/learned.md`.
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
arthik version
```

In Docker: `docker exec arthik arthik check`.

## Development

```sh
go test ./...
ARTHIK_DATA=./data ARTHIK_PASSWORD=dev go run ./cmd/arthik serve   # http://localhost:8080
```

Go 1.23+, dependencies: `gopkg.in/yaml.v3`, `golang.org/x/crypto/bcrypt`. The front end
is plain ES modules and CSS embedded in the binary — no build step, no CDN.

## Licence

MIT
