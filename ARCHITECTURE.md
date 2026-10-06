# arthik — Architecture (internal)

> Internal reference for the Nimory homelab. Share this file (plus only the files being
> changed) instead of the full source. No secret values — only names and paths.
> Last reviewed: 2026-10-06 (v1.0.0, replaces the old CSV-based Arthik and demoarthik).

---

## 1. Context

arthik is a standalone double-entry finance PWA with Bluecoins-style screens. It shares
nothing with Omnimo/Kronos/Natlas except the vault: its files live in
`<notes vault>/data/finance` on nimory (`FINANCE_PATH` in `.env`, mounted at `/data/finance`).

| URL | What |
|---|---|
| `arthik.nihars.com/` | public about page (static, no JS) |
| `arthik.nihars.com/app` | the PWA (sign-in: owner from `.env`, or the read-only demo user) |
| `demoarthik.nihars.com` | retired — the demo is a user now |

No locks on data files (Nimory rule): any app may edit them; arthik re-reads on change.

---

## 2. Deployment

| Item | Value |
|---|---|
| Image | `golang:1.23-alpine` (runs `go vet` + `go test`) → `alpine:3.20`, binary `/usr/local/bin/arthik` |
| Command | `arthik serve`, listens on `:8080` |
| User | `.env` `PUID:PGID`, `init`, `read_only` root fs, `no-new-privileges` |
| Volume | `$FINANCE_PATH → /data/finance` (rw) |
| Port | `${BIND_ADDR}:${PORT} → 8080` (default `127.0.0.1:8085`); Caddy/Cloudflare tunnel in front |
| `.env` | compose: `FINANCE_PATH PUID PGID BIND_ADDR PORT TZ` · app: `ARTHIK_USER ARTHIK_PASSWORD ARTHIK_SECRET ARTHIK_DEMO ARTHIK_DEMO_USER ARTHIK_DEMO_PASSWORD ARTHIK_WATCH_SECONDS` |
| Health | `GET /healthz`; `arthik health` is the Docker healthcheck |
| Logs | stdout (json-file 10 MB × 3) |

Rebuild after code changes (`docker compose up -d --build`); restart after `.env` changes.
Front-end changes need a rebuild too (assets are embedded). Bump `VERSION` in `web/static/sw.js`
only if you want installed PWAs to drop their offline cache.

---

## 3. Code map

```
cmd/arthik/main.go          CLI: serve | hash-password | check [--demo] | rebuild | demo-seed | health | version
internal/config/            .env loader + Config (container-side values only)
internal/money/             Amount = int64 paise; Parse/String/Format(₹ Indian grouping)/MulDiv; YAML+JSON as "123.45"
internal/dates/             Date (calendar day), AddMonths with anchor day
internal/period/            weekly|monthly|quarterly|halfyearly|yearly|custom ranges; week/month/year start opts; Buckets; Resolve
internal/recur/             repeat rules ("monthly", "2 weeks" …) + Next with anchor day
internal/store/             YAML .md read/write: FileError(file,line), atomic write, skip-if-unchanged, emoji unescape
internal/model/             records as stored: Settings, Account(+AccountTypes), Category, Label, Posting, Transaction, Reminder
internal/book/              the ledger engine (one Book per dataset)
  book.go                     load/reload, fixups, Mutate (copy → validate → persist touched files → recompute), DryRun
  entry.go                    Entry (UI shape) ↔ Transaction (postings): BuildTx, EntryOf, ValidateTx, ids, EnsureLabels
  compute.go                  Computed: balances, cleared, projected, net worth, period totals; writes computed fields + money.md
  schedule.go                 PostDue (reminders), Upcoming, RunScheduler (day change + due reminders)
  watch.go                    polling watcher for external edits (acts once a change is stable for one tick)
internal/query/             shared transaction filter (list, search, export, reports)
internal/view/              API shapes: Tx, Account(+Card statement info), BudgetFor (period scaling)
internal/httpx/             Router (auth, CSRF header, read-only guard, panic recovery), JSON helpers
internal/auth/              users, HMAC session cookie, bcrypt/plain password, login rate limit, /api/login|logout|me
internal/feature/<name>/    one package per feature, each with Register(rt):
  state        GET /api/state, GET /api/period
  accounts     POST/PUT/DELETE /api/accounts, /{id}/history, /{id}/reconcile
  categories   POST/PUT/DELETE(?move_to=) /api/categories, GET /api/categories/totals
  labels       POST/PUT/DELETE /api/labels
  transactions GET/POST /api/transactions, GET/PUT/DELETE /{id}, POST /bulk, GET /api/suggest
  reminders    POST/PUT/DELETE /api/reminders, POST /{id}/skip, GET /api/upcoming
  budgets      GET /api/budgets
  reports      GET /api/reports/{cashflow,categories,trend,networth,balance-sheet,labels,accounts,calendar}
  settings     PUT /api/settings
  importexport GET /api/export.csv, POST /api/import (?dry_run=1&create_missing=1)
internal/demo/              demo generator (6 months, relative to today, deterministic per month)
internal/server/            mux: public pages, auth, features, security headers (CSP self-only)
web/static/                 embedded front end (no build step)
  about.html app.html sw.js manifest.webmanifest icon.svg icon-*.png css/app.css
  js/main.js(shell, router, sign-in, sync) util.js api.js(offline queue) ui.js charts.js txform.js txlist.js
  js/views/<feature>.js     one module per screen
```

Dependencies: `gopkg.in/yaml.v3`, `golang.org/x/crypto` (bcrypt) — both via GitHub
mirrors (`replace` in go.mod), same as Omnimo/Kronos.

---

## 4. Double-entry model

- A **Transaction** has ≥ 2 **postings**; each names exactly one `account` or `category`;
  amounts are signed (`+` debit, `−` credit) and must **sum to zero**.
- Accounts are the balance-sheet side; categories are the income/expense side. Opening
  balances are an implicit equity account (`opening_equity` in money.md), so
  `Σ accounts(projected) + Σ category postings + opening_equity = 0` — checked and written
  as `ledger.balanced`.
- UI shapes → postings (`book.BuildTx`):

| Type | Postings |
|---|---|
| expense A from X to C | `X: −A`, `C: +A` (split: one `+` per category) |
| income A into X from C | `X: +A`, `C: −A` |
| transfer A from X to Y | `X: −A`, `Y: +A` |
| refund (expense, negative A) | signs flip |
| journal | free-form postings (hand-typed); edited in the file, not the form |

- Account balance sign is natural: liabilities owed are **negative** (`opening_balance`
  too). Category “natural” amount: expense → posting; income/both → −posting.
- Category kinds: `expense`, `income`, `both` (gains positive, losses negative — e.g. Market).
- **Future-dated** transactions are stored normally; they count in `projected_balance` and
  in nothing else until their date (`Computed.Effective`). The scheduler recomputes when the
  day changes. Period totals/reports exclude them.
- **Reminders** post occurrences with id `r-<reminder>-<YYYYMMDD>` (idempotent), advance
  `next` with the anchor-day rule, honour `end_date` / `remaining`, mark `done`.
  Creating/editing one back-fills every occurrence up to today.

---

## 5. Files and sync contract

All under the dataset folder; YAML with a `# header` and `updated:` line; written atomically
and only when content changed (so Syncthing isn't spammed).

| File | Edited by | Computed fields (rewritten) |
|---|---|---|
| `settings.md` | user/app | — |
| `accounts.md` | user/app | `balance`, `cleared_balance`, `projected_balance` |
| `categories.md` | user/app | `period`, `total` (default period, parents include children) |
| `labels.md` | user/app | `count` |
| `reminders.md` | user/app | `next`, `anchor_day`, `posted`, `remaining`, `done` maintained |
| `money.md` | arthik only | everything (not watched) |
| `transactions/transaction_YYYYMM.md` | user/app | — |
| `.demo-seed` (demo only) | arthik | month the demo was generated |

Load rules: missing file = empty; parse error = `loadErr` → reads keep last good data, every
write refused with `file line N: …`, nothing written; fixups on load (missing/duplicate ids,
wrong month file, unknown labels → added to labels.md); invalid transactions → `problems`,
excluded from totals. Unknown keys preserved (`Extra` inline maps). Amounts accept quoted
strings or bare numbers. IDs: accounts/categories/labels are slugs of the name
(`food-groceries` for a child); transactions `t-YYYYMMDD-xxxxxx`, client-created `c-…`.

---

## 6. Front end

- Hash router (`#/home`, `#/tx`, `#/account/<id>`, `#/category/<id>`, `#/reports/<tab>` …).
  Mobile: bottom bar + ＋ button; ≥ 900 px: sidebar.
- `html``…`` tagged template escapes every value (data comes from editable files).
- Offline: service worker caches the shell and the last answer of each `GET /api/*`
  (network first). Transaction creates/edits/deletes made offline go to a localStorage
  queue and are replayed in order (`online` event, every 30 s, at start). Creates carry a
  client id → server returns `duplicate: true` on replay. Other edits need a connection.
- Charts are hand-drawn SVG at the box's real pixel width (`charts.js`), with tooltips.
- Theme: nss teal `#3a807a`, square grid background, tokens on `:root`, dark mode auto or forced.

---

## 7. Security

Signed cookie (`HttpOnly`, `SameSite=Lax`, `Secure` behind HTTPS), 30-day sessions; every
non-GET needs header `X-Arthik` (CSRF) and is refused for read-only users; login limited to
10 failures / 15 min per IP (`CF-Connecting-IP` aware); CSP `script-src 'self'`.

---

## 8. Adding a feature

1. `internal/feature/<name>/<name>.go` with `func Register(rt *httpx.Router)`; read with
   `c.Book.View`, change with `c.Book.Mutate(func(d *book.Data, ch *book.Change) error)` —
   validate first, replace records (don't edit nested slices in place), mark `ch.*`.
2. Add `<name>.Register(rt)` in `internal/server/server.go`.
3. Screen: `web/static/js/views/<name>.js` exporting `render(root, ctx)`, route in `main.js`,
   file listed in `sw.js`.
4. Tests: `internal/book/book_test.go` (engine) and `internal/server/server_test.go` (API).

---

## 9. Known limits / later

- Only transaction changes queue offline; accounts/categories/settings need a connection.
- One person per dataset (the demo is a separate folder); INR only; no attachments (by design).
- Watcher is polling (default 3 s).
- Planned later (not built): Telegram notices via Omnimo's outbox, Kronos integration,
  showing `subscription.md` items as upcoming bills.
