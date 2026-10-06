# arthik

Self-hosted, **double-entry** personal finance — the everyday feel of a phone money
tracker (accounts, categories, budgets, reminders, reports) with every transaction
balanced: debited somewhere, credited somewhere else.

- Mobile-first **PWA**: install it on your phone, add transactions **offline**, they sync when you are back.
- Data is plain **YAML in `.md` files** — readable in Obsidian or any editor, and edits made there are picked up automatically.
- One static Go binary, one container, no database, no third-party services, no tracking.
- A built-in, read-only **demo user** with generated data.

## Features

| Area | What you get |
|---|---|
| Accounts | Bank, cash, wallet, investment, deposit, property, receivable, credit card, loan, payable … Balance (today), cleared balance, projected balance (incl. future-dated). Credit cards: limit, available, statement/due dates, billed/unbilled. Hide, archive, leave out of net worth. Reconcile against a statement. |
| Transactions | Expense, income, transfer, split across categories, refunds. Calculator amount field (`120+45*2`). Remembered titles fill in category/account/amount. Labels, notes, time, status (uncleared/cleared/reconciled). Past dates fix history; future dates count from their day. Duplicate, bulk status/label/delete. |
| Categories | Two levels (parent › child) for expense, income, and “both ways” (e.g. *Market* gains and losses). Totals for a week, month, quarter, half-year, year or any custom range; the default period is a setting. Delete with “move transactions to …”. |
| Budgets | Per category, weekly → yearly, scaled to the period you view, reset every period, with on-pace progress. |
| Scheduled | Reminders that auto-post: daily, weekly, every 2 weeks, monthly, every 2 months, quarterly, half-yearly, yearly or “N days/weeks/months/years”; end date or number of times; skip, pause. Month-end dates never drift (31 Jan → 28 Feb → 31 Mar). Past start dates back-fill. |
| Reports | Cash flow, spending and income by category with drill-down, category trend, net worth over time, balance sheet at any date, account flow, labels, calendar. Every chart has a table. |
| Search & data | Full-text search plus filters (dates, type, accounts, categories, labels, status, amount, posted/future); totals of the filtered list; CSV export of exactly what you filtered; CSV import with a dry-run preview. |
| Settings | Default period, week start, month start day (salary cycles), year start month (e.g. April for the Indian FY), look-ahead days, light/dark/auto, hide amounts. |

## Quick start (Docker)

```sh
git clone https://gitlab.com/niharokz/arthik.git && cd arthik
cp .env.example .env        # set FINANCE_PATH, PUID/PGID, ARTHIK_USER, ARTHIK_PASSWORD, ARTHIK_SECRET
docker compose up -d --build
```

Open `http://127.0.0.1:8085` (or put your reverse proxy in front: `/` is the public about
page, `/app` is the app). The demo user and password are shown on the sign-in screen.

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
| `BIND_ADDR`, `PORT` | compose | Where the app is published on the host (default `127.0.0.1:8085`). |
| `TZ` | app | Time zone that decides “today”. |
| `ARTHIK_USER`, `ARTHIK_PASSWORD` | app | Your sign-in; the password may be a bcrypt hash. |
| `ARTHIK_SECRET` | app | Signs session cookies; set it so you stay signed in across restarts. |
| `ARTHIK_DEMO`, `ARTHIK_DEMO_USER`, `ARTHIK_DEMO_PASSWORD` | app | Read-only demo user (data in `<FINANCE_PATH>/demouser`, regenerated monthly). |
| `ARTHIK_WATCH_SECONDS` | app | How often to look for edits made by other apps (default 3). |

## Data files

```
<FINANCE_PATH>/
  settings.md                     preferences
  accounts.md                     accounts + computed balance / cleared_balance / projected_balance
  categories.md                   category tree + budgets + computed total for the default period
  labels.md                       labels + computed count
  reminders.md                    scheduled / recurring transactions
  money.md                        computed overview (net worth, period totals, ledger check) — output only
  transactions/transaction_YYYYMM.md   one file per month
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
You can edit any file by hand; arthik notices within seconds, repairs what it safely
can (missing ids, a transaction saved in the wrong month file, labels that don't exist
yet) and rebuilds everything. A file that does not parse is never overwritten — changes
are paused and the app shows the file and line until it is fixed. Entries that don't
balance or point at unknown accounts are listed as problems and left out of totals.
Unknown keys you add to any record are kept.

## Command line

```
arthik serve            run the web app (default)
arthik hash-password    print a bcrypt hash for ARTHIK_PASSWORD
arthik check [--demo]   validate the data files and list problems
arthik rebuild          re-read everything and rewrite all computed values
arthik demo-seed        regenerate the demo data now
arthik health           exit 0 when the server answers (Docker health check)
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
