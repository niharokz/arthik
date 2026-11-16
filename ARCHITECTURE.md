# Arthik — Architecture (internal)

> Internal reference for the Nimory homelab. Share this file (plus only the files being
> changed) instead of the full source. No secret values — only names and paths.
>
> Last reviewed: **2026-11-16**, for version **1.0.6** (the review started from `main` at
> `fe82f92`, VERSION `1.0.5`; its fixes are 1.0.6, see §19). Reviewed in full: every Go package, the web
> front end, scripts, CI, Docker and the Android build files and manifest. **Not
> re-read in this review: the Kotlin sources** (`android/app/src/main/java/…`); §13 is
> built from `android/README.md`, the manifest and the bridge calls the web UI makes.
> `go vet` and `go test ./...` pass on 1.0.6 (run with Go 1.24 on the review machine; the
> Android build and its unit tests were not run).

---

## 1. Context

Arthik is a standalone double-entry personal finance app with Bluecoins-style screens:
one Go binary that serves a no-build web app (PWA), and the same binary embedded in an
Android app as an on-phone engine. It shares nothing with Omnimo/Kronos/Natlas except
the vault: its files live in `<notes vault>/data/finance` on nimory (`FINANCE_PATH` in
`.env`, mounted at `/data/finance`), and Syncthing carries that folder to the phone.

| URL | What |
|---|---|
| `arthik.nihars.com/` | public start page (no JS): sign in, demo login, APK link, version |
| `arthik.nihars.com/app` | the PWA (sign-in: owner from `.env`, or the read-only demo user) |
| `demoarthik.nihars.com` | retired — the demo is a user now |

Rules that shape everything else:

- **Transactions are the only source of truth.** Every balance and total is computed.
- **No locks on data files** (Nimory rule): any app may edit them; Arthik re-reads on change.
- **One writer per producer file**: Kronos owns `inbox/kronos.md`, the phone owns
  `inbox/sms_<device>.md`, Arthik owns `inbox/drafts.md`, `inbox/learned.md`, `money.md`.
- **One main device** (the server) writes computed values, imports the inbox and merges
  Syncthing conflicts; every other instance is a **replica**.
- INR only, one person per dataset, no attachments, no database, no third-party services.

---

## 2. Deployment

| Item | Value |
|---|---|
| Image | `golang:1.26-alpine` (runs `go vet` + `go test`, then a static `CGO_ENABLED=0` build with `-X main.version=$(cat VERSION)`) → `alpine:3.23`, binary `/usr/local/bin/arthik` |
| Command | `arthik serve`, listens on `:8080` (`ARTHIK_LISTEN`) |
| User | `.env` `PUID:PGID`, `init`, `read_only` root fs, `tmpfs /tmp`, all capabilities dropped, `no-new-privileges`, `stop_grace_period 30s` |
| Volumes | `$FINANCE_PATH → /data/finance` (rw) · `$STATE_PATH → /state` (rw, `bans.json`; not synced) |
| Network | external `ARTHIK_NETWORK` (`homelab`); Caddy proxies `arthik.nihars.com` / `arthik.nimory` → `arthik:8080` (no host port) |
| Health | `GET /healthz`; `arthik health` is the Docker healthcheck (60 s interval) |
| Logs | stdout (json-file 10 MB × 3) |
| Build context | `.dockerignore` drops `android/`, `patch/`, zips, root `*.md`, `.gitlab*`, `scripts/` |

`.env` variables — compose only: `FINANCE_PATH STATE_PATH PUID PGID ARTHIK_NETWORK`.
Passed into the container (and nothing else, so `.env` can be shared): `TZ` and

| Variable | Default | Meaning |
|---|---|---|
| `ARTHIK_USER` | `admin` in code (`me` in `.env.example`) | owner's sign-in name |
| `ARTHIK_PASSWORD_HASH` / `ARTHIK_PASSWORD` | — (one is required) | bcrypt hash (must start `$2`) or plain text |
| `ARTHIK_SECRET` | random per start | HMAC key for session cookies |
| `ARTHIK_SESSION_HOURS` | 12 | length of a sign-in without the device key |
| `ARTHIK_DEVICE_KEY` | off | ≥ 24 chars; a device that presents it stays signed in |
| `ARTHIK_LOGIN_MAX_FAILS`, `ARTHIK_BAN_DAYS` | 5, 30 | lock-out |
| `ARTHIK_BAN_EXEMPT` | none | CIDRs never banned |
| `ARTHIK_TRUSTED_PROXIES` | loopback + RFC 1918 + `fc00::/7` | peers whose forwarding headers are believed |
| `ARTHIK_DEMO_USER`, `ARTHIK_DEMO_PASSWORD` | off | both set = read-only demo login |
| `ARTHIK_WATCH_SECONDS` | 3 | poll interval for external edits and the inbox runner |
| `ARTHIK_PRIMARY` | `true` | `false` = replica |
| `ARTHIK_INBOX_MAX_AGE_DAYS`, `_KEEP_DAYS`, `_PAIR_DAYS`, `_RESCAN_MINUTES` | 30, 120, 2, 10 | inbox limits (`KEEP` must be > `MAX_AGE`, checked at start) |
| `ARTHIK_AI_URL`, `ARTHIK_AI_MODEL`, `ARTHIK_AI_TIMEOUT_SECONDS` | off, off, 120 | optional Ollama |

Set by compose/Dockerfile, not by you: `ARTHIK_DATA=/data/finance`, `ARTHIK_STATE_DIR=/state`,
`ARTHIK_LISTEN=:8080`. Local mode only (set by the Android app): `ARTHIK_MODE=local`,
`ARTHIK_LOCAL_TOKEN`, `ARTHIK_DEMO_DIR`, `ARTHIK_EXIT_ON_STDIN_EOF=1`.

Rebuild after code or front-end changes (`docker compose up -d --build`; assets are
embedded); restart after `.env` changes. Bump `VERSION` in `web/static/sw.js` only to make
installed PWAs drop their offline cache.

---

## 3. Process model

`arthik serve` (`cmd/arthik/main.go`) is one process, no workers, no queue:

```
main
 ├─ config.LoadDotEnv(".env") + config.Load()        env → Config (fails fast on bad values)
 ├─ own book   book.OpenWith(ARTHIK_DATA, {Replica: !Primary})     name "main" / "phone"
 │   ├─ go own.Watch(ctx, WatchEvery)                 external edits → reload
 │   ├─ go own.RunScheduler(ctx, 30s)                 day change → recompute
 │   └─ go inbox.Runner{Book, Cfg, AI}.Run(ctx, WatchEvery)   producer files → drafts, AI batch
 ├─ demo book  book.Open(DemoDir, "demo", readOnly)   seeded when .demo-seed is stale
 │   ├─ go demo.RunScheduler(ctx, 30s)
 │   └─ go refreshDemo (every 6 h: reseed on a new month, then Reload)
 ├─ guard.New (server mode only)                      bans.json in ARTHIK_STATE_DIR
 ├─ auth.New(secret, opts, accounts…)                 owner + demo; EnableLocal(token) in local mode
 └─ http.Server{Handler: server.New(...)}             Read 30 s / Write 60 s / Idle 120 s
     shutdown: SIGINT/SIGTERM (or stdin EOF in local mode) → srv.Shutdown(15 s)
```

One `book.Book` per dataset; a user is bound to exactly one book (`httpx.User.Book`).
All state is the book in memory plus the files; restart loses nothing but the in-memory
failed-sign-in counters, the inbox runner's signatures and status, and (without
`ARTHIK_SECRET`) every session.

CLI (same binary, each opens the dataset itself):
`serve` · `hash-password` · `check [--demo]` (read-only open, exit 1 on problems) ·
`rebuild` (main open = rewrite computed files) · `demo-seed` · `health` ·
`summary` (read-only JSON, used by the phone's widgets/alert) · `parse-sms TEXT|stdin` ·
`bans` · `unban IP|all` · `version`.

---

## 4. Code map

```
cmd/arthik/main.go          CLI and the wiring in §3 (version from the VERSION file via -ldflags)
internal/config/            .env loader + Config{Security, Inbox}; server vs local mode validation
internal/money/             Amount = int64 paise; Parse ("1,23,456.7", "₹ 500", "(12.05)"; >2 decimals is an error),
                            String/Format(₹ Indian grouping)/MulDiv(half away from zero); YAML + JSON as "123.45"
internal/dates/             Date (calendar day at UTC midnight), AddMonths with anchor day, MonthKey "YYYYMM"
internal/period/            weekly|monthly|quarterly|halfyearly|yearly|custom (+ daily for report buckets);
                            Of/Between/Shift/Buckets(max 400)/Resolve; week start, month start day, year start month; labels
internal/store/             YAML .md read/write: FileError(file,line), atomic write (temp+fsync+rename),
                            skip-if-unchanged (ignores the header and `updated:`), emoji unescape
internal/model/             records as stored: Settings(+Look, Alerts, HomeSection), Account(+AccountTypes, Reported),
                            Category, Posting, Transaction, Draft, Rule; allowed values (Palettes, CardStyles, Densities,
                            LookColors, Charts, HomeBuiltins, HomeCustom, NavItems); retired keys; NormNav
internal/book/              the ledger engine (one Book per dataset) — §7
  book.go                     Data/Book/Change/Problem, Open, View, Mutate, DryRun, reload, persist, norm*, indexes
  entry.go                    Entry (UI shape) ↔ Transaction (postings): BuildTx, EntryOf, ValidateTx, Slug, ids
  compute.go                  Computed (balances, projected, net worth, default-period totals); writes money.md
  schedule.go                 RunScheduler: recompute when the day changes
  digits.go                   SMS digits on accounts: CleanDigits, DigitsFree, AccountByDigits
  watch.go                    polling watcher (acts once a change is stable for one tick)
  sync.go                     stamps, tombstones, conflict-copy merge, main vs replica
internal/inbox/             the review inbox, no HTTP — §9
  sources.go                  ReadKronos / ReadSMS → Items; SMSID, Key, Pretty, Phones, LastCapture
  sms.go + sms_rules.yaml     data-driven SMS parser (user file inbox/sms_rules.md first)
  engine.go                   Ingest, Categorise, pair/Unpair, purge, noticeKronos
  review.go                   Accept / Match / Reject / Restore, learn, rememberAccountDigits
  view.go                     duplicate ("same"), warnings, confidence
  notices.go                  Kronos reminders: paid/proof matching, dismiss/reopen
  runner.go                   background import schedule + AI batch + Status
  request.go                  inbox/run_request.md ("import now" asked by another device)
internal/ai/                Ollama /api/chat client: picks a category id from the given list + a clean title
internal/plugin/            personal plugins: Plugin{ID, Title, Icon, File, Register}; on when <data>/<File> exists
  loans/                      loans.md → schedule, projection, FY totals, EMI split
internal/query/             shared transaction filter (list, search, export, reports)
internal/view/              API shapes: Tx, Account(+Card, +Check), BalanceAt, Checks, Mismatched, Excluded, BudgetFor
internal/httpx/             Router.Handle (auth, CSRF header, read-only guard, panic recovery), JSON helpers, StatusError
internal/auth/              users, HMAC session cookie, bcrypt/plain password, device key, /local/enter
internal/guard/             failed sign-ins → IP ban (bans.json), ClientIP behind trusted proxies, middleware
internal/feature/<name>/    one package per feature, each with Register(rt) — routes in §6
internal/server/            mux: public pages, auth, features, plugins, static files, security headers
internal/demo/              demo generator (6 months back from today, deterministic per month, seedVersion "2")
internal/e2e/               two-device Syncthing simulation tests (main + phone), incl. the inbox
web/embed.go                //go:embed static
web/static/                 the front end (no build step) — §11
android/                    Kotlin WebView shell + the engine as libarthik.so — §13
docker/builder/Dockerfile   arthik-builder image (JDK 17, Android SDK 34, Go 1.23.4)
scripts/                    release.sh, build-in-container.sh, upload-release.sh, check-public.sh, dev.sh
```

Dependencies: `gopkg.in/yaml.v3` and `golang.org/x/crypto` (bcrypt only), both through
GitHub mirrors (`replace` in go.mod, same as Omnimo/Kronos). `go 1.23` in go.mod, standard library
`net/http` with 1.22 route patterns.

---

## 5. Request path

```
client → Caddy (→ Cloudflare Tunnel) → arthik:8080
  headers()                 CSP, nosniff, Referrer-Policy same-origin, X-Frame-Options DENY, Permissions-Policy
  guard.Middleware          banned IP → 403 "Forbidden" on every path except /healthz   (server mode only)
  http.ServeMux
    GET /healthz            "ok"
    auth routes             POST /api/login · POST /api/logout · GET /api/me ·
                            POST|DELETE /api/device-key · GET /local/enter            (plain handlers)
    feature routes          rt.Handle(pattern, fn)  ← everything in §6
    GET /api/               404 JSON "no such API endpoint"
    GET /{$}                start page (about.html with {{VERSION}} {{APK}} {{REPO}} {{DEMO}}); local mode: 302 /app
    GET /app, /app/         app.html (no-cache)
    GET /                   embedded static files: /sw.js, /js/, /css/ no-cache; the rest max-age=86400
```

`httpx.Router.Handle` wraps every feature handler: recover (500 + stack in the log) →
`Resolve(r)` (401 "please sign in") → for non-GET/HEAD: require header `X-Arthik`
(403) and refuse read-only users (403) → `Cache-Control: no-store` → handler. A returned
error becomes `{"error": msg}`: `*StatusError` keeps its code, `book.ErrReadOnly` is 403,
anything else 400. Bodies: JSON, 2 MB max (`Decode`); CSV import 20 MB; auth bodies 64 KB.

---

## 6. HTTP API

All `/api/*` below need a session; amounts are decimal strings (`"1234.50"`), dates `YYYY-MM-DD`.
"Period query" = `?period=&date=&from=&to=&offset=` resolved by `period.Resolve`.

| Package | Route | Notes |
|---|---|---|
| auth | `POST /api/login` `{user,password,key?}` | sets the cookie; `key` = device key → remembered session |
| | `POST /api/logout` · `GET /api/me` | `me`: `signed_in,user,read_only,remembered,device_key,expires,session_hours,demo{user,password}`, or `local:true` |
| | `POST /api/device-key {key}` · `DELETE /api/device-key` | make this session remembered / normal again |
| | `GET /local/enter?token=&user=&to=` | local mode only; `to` must match `^[a-z]+(/[A-Za-z0-9_.-]+)?$` |
| state | `GET /api/state` | `user, read_only, version, today, settings, accounts[view.Account], categories[+total], inbox{pending}, notices{active}, plugins, account_types, period_kinds, look_options, category_uses, summary{net_worth, excluded, excluded_ids, checks_off, unchecked, assets, liabilities, period, income, expense, net}, problems, load_error?` |
| | `GET /api/period` | period query → `{range, prev, next}` |
| accounts | `POST /api/accounts` · `PUT /api/accounts/{id}` · `DELETE /api/accounts/{id}` | delete refused while transactions use it |
| | `GET /api/accounts/{id}/history?from=&to=&bucket=` | balance at each bucket end (default 12 months, monthly) |
| | `POST /api/accounts/{id}/check {balance,date}` | bank balance check (not in the future) |
| categories | `POST /api/categories` · `PUT /api/categories/{id}` | children take the parent's kind; a parent's kind change flows down |
| | `DELETE /api/categories/{id}?move_to=` | 409 when used and no `move_to`; drafts and rules follow |
| | `GET /api/categories/totals` | period query → `{range, prev, next, totals, budgets, income, expense, net}` |
| transactions | `GET /api/transactions` | filter `from,to,type,account,category,q,min,max,when=posted\|scheduled` + `limit` (200, max 5000), `offset`, `running=1` (one account: `balances`) → `{items, total, sums}` |
| | `GET /api/transactions/{id}` · `PUT` · `DELETE` | `PUT` keeps hand-added keys (`Extra`) |
| | `POST /api/transactions` | body = `book.Entry`; a known or tombstoned client `id` → `{duplicate:true}` |
| | `POST /api/transactions/bulk {ids, action:"delete"}` | |
| | `GET /api/suggest?q=&type=&limit=` | latest valid non-journal transaction per title |
| review | `GET /api/inbox?status=pending\|done\|all` | `{items[inbox.View], pending, status, main, phones, sms_time}` |
| | `POST /api/inbox/{id}/accept` (`Entry` + `remember_digits`) · `/match {txn}` · `/reject` · `/restore` · `/unpair` | |
| | `POST /api/inbox/import` | import now → `{added,last_capture}`, or `{requested:true}` after writing `inbox/run_request.md` |
| | `GET /api/inbox/rules?q=&offset=&limit=` (50, max 200) · `DELETE /api/inbox/rules/{id}` | most used first |
| | `GET /api/notices[?all=1]` · `POST /api/notices/{id}/dismiss` · `/reopen` | |
| budgets | `GET /api/budgets` | period query → `{range, prev, next, lines, total}` |
| reports | `GET /api/reports/cashflow` · `categories?kind=&parent=` · `trend?category=` · `networth` · `balance-sheet?date=` · `accounts` · `calendar?month=YYYY-MM` | range: period query, or `from/to`, or the last 12 months; `bucket=daily\|weekly\|monthly\|…`; narrowed by `account,type,q` |
| summary | `GET /api/summary?newest=` | `{today, period, income, expense, budget, inbox{pending,out,in,newest}, net_worth{now,assets,liabilities,change,trend[12]}, alert{time,show,mismatch,count,limit,text}}` |
| settings | `PUT /api/settings` | whole `model.Settings`; a missing `nav_order`, `home`, `look`, `alerts` or `balance_check_days` keeps the saved one |
| importexport | `GET /api/export.csv` (same filter as the list) · `POST /api/import?dry_run=1&create_missing=1` (text/csv) | columns `id,date,time,type,title,amount,account,to_account,category,notes,splits` |
| plugin loans | `GET /api/plugins/loans` · `GET /api/plugins/loans/split?amount=&date=` | only useful when `loans.md` exists |

---

## 7. Ledger engine (`internal/book`)

### Double-entry model

- A **Transaction** has ≥ 2 **postings**; each names exactly one `account` or `category`;
  amounts are signed (`+` debit, `−` credit) and must **sum to zero**.
- Accounts are the balance-sheet side; categories are the income/expense side. Opening
  balances post against an implicit equity account (`opening_equity` in money.md), so
  `Σ accounts(projected) + Σ category postings + opening_equity = 0` — checked and written
  as `ledger.balanced`.
- UI shapes → postings (`BuildTx`), and back (`EntryOf`; anything it cannot map is shown
  as a journal):

| Type | Postings |
|---|---|
| expense A from X to C | `X: −A`, `C: +A` (split: one `+` line per category, or per account the money moves into) |
| income A into X from C | `X: +A`, `C: −A` |
| transfer A from X to Y | `X: −A`, `Y: +A` (exactly two accounts, no category) |
| refund (expense, negative A) | signs flip |
| journal | free-form postings (hand-typed); edited in the file, not the form |

- Account balance sign is natural: liabilities owed are **negative** (`opening_balance`
  too). Category "natural" amount (`Natural`): expense → posting; income/both → −posting.
  `IncomeExpense` splits a `both` category into income (gains) and expense (losses).
- Category kinds: `expense`, `income`, `both`. Two levels only (parent › child).
- Account types (`model.AccountTypes`): assets `bank cash wallet investment deposit property
  receivable other_asset`; liabilities `credit_card loan payable other_liability`. An unknown
  type is a problem and counts as an asset.
- **Future-dated** transactions are stored normally; they count in `projected_balance` and
  in nothing else until their date (`Computed.Effective`). Period totals, reports and
  list sums exclude them (the list counts them as `upcoming`).
- **Validation** (`ValidateTx`): id shape, date, `HH:MM` time, non-empty title, ≥ 2
  postings, each posting one known account or category and a non-zero amount, sum zero,
  and the per-type shape. An invalid stored transaction becomes a `Problem` and is left
  out of every total.
- **Recurring bills** are not Arthik's job: Kronos drops due bills into `inbox/kronos.md`;
  since 1.0.3 they are reminders (draft status `notice`), never transactions.

### Data, reads and writes

`Data` = Settings, Accounts, Categories, Drafts (+ `SMSImport` stamp), Rules, Txns (sorted
by date, time, id) plus private `tombs` (deleted ids), `src` (id → month file) and lookup
indexes. `Computed` = `Today`, `Valid`, `Balance`, `Projected`, `OpeningEquity`, `NetWorth`,
`Assets`, `Liabilities`, default `Period`, `CatTotals` (parents include children),
`Income`, `Expense`, `Problems`.

| Call | Lock | What it does |
|---|---|---|
| `View(fn)` | read | read data + computed |
| `Mutate(fn)` | write | reload first if a file changed on disk → refuse while `loadErr` is set → run `fn` on a **shallow clone** → sort, reindex → `persistLocked` (only files marked in `Change`) → swap in → `recomputeLocked` |
| `DryRun(fn)` | read (clone) | import preview; nothing saved |
| `Reload()` | write | re-read every file |
| `Recompute()` | write | day change |

Rules for a `Mutate` function: validate first, then change; **replace records, never edit
nested slices or maps in place** (the clone shares them with the live data); mark what you
touched in `Change` (`Settings Accounts Categories Drafts Rules`, `Month(key)`); use
`SetTx` / `DeleteTx` / `SetDraft` / `SetRule` / `DeleteRule`, which stamp `updated` and
mark the files. If persisting fails, the book reloads from disk.

`reloadLocked`: read the five definition files (missing = empty) → drop retired keys →
glob `transactions/transaction_*.md` including conflict copies → `resolveTxns` (below) →
merge definition and inbox conflict copies → `fixup` (missing account/category ids) →
on the main device persist repairs and delete merged copies → recompute. A parse error
sets `loadErr`: reads keep the last good data (or whatever parsed, on first load), every
write is refused with `file line N: …`, nothing is written. A broken *conflict copy* is
only a warning.

`recomputeLocked(write)`: rebuild `Computed`, mirror `balance`/`projected_balance` onto
accounts and `period`/`total` onto categories, and — main device, clean load only — write
`accounts.md`, `categories.md` and `money.md` (each only if its content changed).

`normSettings` repairs a hand-edited `settings.md` on load: bad period/week/month/year
values → defaults, unknown look values dropped (`NormLook`), bad alert time repaired
(`NormAlerts`), retired home block types and menu items removed.

### Watcher and scheduler

`Watch` polls size + mtime of the five definition files, every month file and every
conflict copy (not `money.md`, not the producer files). A change is acted on once it has
been identical for one tick, so half-synced files are not read mid-write. `RunScheduler`
ticks every 30 s and recomputes when the calendar day (container `TZ`) changes.

---

## 8. Files and sync contract

All under the dataset folder; YAML with a `# header` and a top-level `updated:` line;
written atomically (0644) and only when content changed, so Syncthing isn't spammed.

| File | Edited by | Computed fields (rewritten) |
|---|---|---|
| `settings.md` | user/app | — |
| `accounts.md` | user/app | `balance`, `projected_balance` |
| `categories.md` | user/app | `period`, `total` (default period, parents include children) |
| `transactions/transaction_YYYYMM.md` | user/app | — |
| `money.md` | Arthik (main) only | everything: `overview`, `period`, `by_type`, `ledger{transactions, scheduled, opening_equity, balanced, problems}`; not watched |
| `inbox/drafts.md` | Arthik (+ your decisions from any device) | whole file; top-level `sms_imported:` stamp |
| `inbox/learned.md` | Arthik (edit/delete freely) | `uses`, `updated` |
| `inbox/kronos.md` | **Kronos only** | read only for Arthik |
| `inbox/sms_<device>.md` | **phone app only** | read only for Arthik (`device`, `schedule`, `last_run`, `messages[{id,from,at,body}]`) |
| `inbox/sms_rules.md` | you (optional) | — |
| `inbox/run_request.md` | a device that does not import | deleted by the main device after the import |
| `loans.md` | you (optional) | read only; switches the Loans plugin on |
| `.demo-seed` (demo only) | Arthik | month + seed version the demo was generated for |
| `<state>/bans.json` | Arthik / `arthik unban` | outside the dataset, not synced, 0600 |

Record rules: unknown keys are preserved (`Extra` inline maps on every record); amounts
accept quoted strings or bare numbers; retired keys are dropped on load and vanish at the
next write (`status`, `labels` on transactions; `labels` on drafts and rules;
`cleared_balance` on accounts); an old `reminders.md` / `labels.md` is ignored.
IDs: accounts/categories are slugs of the name (`food-groceries` for a child, `-2` on a
clash); transactions `t-YYYYMMDD-xxxxxx`, client-created `c-…`, accepted drafts
`d-<draft id>`; any id must match `^[A-Za-z0-9][A-Za-z0-9_.:-]{0,79}$`.

### Multi-device sync

| Piece | Rule |
|---|---|
| `updated` | set by `SetTx` / `SetDraft` / `SetRule` on every change, UTC RFC 3339 with ms |
| tombstone | `DeleteTx` keeps `{id, date, deleted}` in the month file; purged by the main device after 90 days (`TombstoneTTL`) |
| resolving transactions | every record from main files, conflict copies and stale copies in old months is resolved by id: newest `updated`/`deleted` wins; tie → main file beats a copy; identical duplicates collapse; two *different* records sharing an id in main files → the second is renamed (a Problem); missing ids assigned; a record in the wrong month file is moved |
| conflict copies | `transaction_YYYYMM.sync-conflict-*.md` and `<file>.sync-conflict-*.md` are read on load, merged, then deleted (main only) |
| definition files | `accounts` / `categories` copies: union by id, **main copy wins** on the same id; `settings` / `money` copies: main wins outright |
| inbox files | `drafts` / `learned` copies merge by id (rules by `match`), newest `updated` wins; the later `sms_imported` wins; producer files are never merged (their owner rewrites them) |
| main vs replica | `Book.Replica` (`ARTHIK_PRIMARY=false`): never writes computed fields or `money.md`, never repairs or deletes files; merges conflict copies in memory so its totals are right, and saves only user changes (including accept/reject of drafts) |
| notices | merges are reported as `Problem{Level:"info"}` (Data health), not as warnings |

---

## 9. Inbox (`internal/inbox` + `feature/review`)

```
 phone app ── daily ───► inbox/sms_<device>.md ┐
 Kronos ──── 02:12 ────► inbox/kronos.md       ├─► main: Runner → Ingest ─► inbox/drafts.md ─► Inbox (SMS) / 🔔 Notifications (Kronos)
 another device ───────► inbox/run_request.md  ┘      (parse, account by digits,            accept → transaction d-<id>
                                                       categorise, pair, purge)              + inbox/learned.md
```

**Sources.** `ReadKronos`: each `drafts[]` entry with an id and a positive amount →
`kronos-<slug>` (hashed tail when longer than 60). `ReadSMS`: every message of every
`sms_*.md` (conflict copies skipped) goes through the parser; id = `sms-<16 hex>` from the
phone, else sha256 of sender, time and text; date/time from the SMS text, else from `at`.

**SMS parser** (`sms.go`, rules in `sms_rules.yaml`): macros (`{amount} {date} {time}
{cur}`), `ignore` patterns (OTP, "will be debited", collect requests, declined, due
reminders, offers), then rules in order — the user's `inbox/sms_rules.md` first, then
built-ins `card-spent`, `card-spent-verb-first`, `card-payment`, `card-payment-received`,
`account-first`, `amount-first`, and the catch-all `generic`. Named groups: `amount`
(required) `digits merchant date time dir balance available`. Rule type `expense | income
| transfer_in | auto` (auto = the `dir` group, else the first of `credit_words` /
`debit_words` in the text; neither → skipped). `disable: [name]` or `enabled: false`
switches a rule off. Test with `arthik parse-sms`.

**Ingest** (main only, idempotent): skip ids already drafted or already a `d-` transaction;
skip items older than `MAX_AGE` or dated after tomorrow; build the draft; turn any pending
Kronos draft into a `notice`; `pair`; `purge` reviewed drafts older than `KEEP_DAYS`.

- **Account**: `sms_digits` suffix match (either way round); Kronos account names match an
  account id or name; a learned rule's account only when the SMS has no digits. *Remember*
  on accept moves the last four digits onto the chosen account.
- **Category order** (`Categorise`, sets `via`): learned rule (key = `Key(merchant)`) →
  newest past transaction whose title key matches (not `d-` ones) → the source's category
  name (`tech/software` → last part) → AI (later, see runner) → `settings.inbox_*_category`.
- **Pairing**: a pending, unedited SMS expense from an *asset* account + a same-amount
  card "payment received" (`transfer_in`) or income on another account within
  `PAIR_DAYS` → one transfer draft (`pair`), the other half `merged`. *Not a transfer*
  (`Unpair`) splits it and marks both edited.
- **Draft statuses**: `pending accepted matched rejected merged` and, for reminders,
  `notice dismissed`.

**Review** (`review.go`): `Accept` (entry fields missing in the form fall back to the
draft; creates `d-<draft id>` unless it exists, so two devices make one transaction;
learns; settles the pair too), `Match` (link to an existing transaction; learns),
`Reject`, `Restore`, `Unpair`. `learn` writes `learned.md`: `match`, `title`, `category`,
`account` (only when the source had no digits), `uses++`.

**Checks on view** (`view.go`, never stored): likely duplicate (same account, same amount,
±1 day); warnings — catch-all pattern, unknown digits, unknown account name, bank balance
or card limit not matching Arthik's (counting earlier pending drafts). **Confidence**:
`high` (learned or past, nothing off), `medium` (source or AI category, or a warning),
`low` (fallback or no category, unknown account, possible duplicate).

**Reminders** (`notices.go`): a notice shows `paid` when a valid transaction of the same
type and amount lies within 7 days of its due date, else `proof` when a pending SMS draft
does. `ActiveNotices` (the bell badge) counts unpaid open notices.

**Runner** (`runner.go`), one tick every `ARTHIK_WATCH_SECONDS`, main device only (on a
replica `Run` returns at once and nothing is registered):

| Source | Read when |
|---|---|
| Kronos | `kronos.md` signature changed, the day changed, or `RESCAN` minutes passed |
| SMS, `inbox_sms_time` empty | SMS files or `sms_rules.md` changed, or the same day/rescan triggers |
| SMS, `inbox_sms_time` = `HH:MM` | once after each daily slot (`SMSDue`; runs once when back after downtime), plus a *late sync*: a capture made before the last slot that arrived after it |
| both | `inbox/run_request.md` exists (deleted after a successful import), or `POST /api/inbox/import` |

Signatures are path + mtime + size and are only taken after a successful import, so a
failed read is retried. While the book has a load error the import pauses (logged once,
shown in `Status.Error`). `Status` (`GET /api/inbox`): `ai, last_import, sms_time,
next_sms, last_sms, error, ai_error`.

**AI** (optional, `internal/ai`): after each tick, up to 5 pending unedited drafts with
no category or only the fallback are sent to Ollama `/api/chat` (`format: json`,
temperature 0) with the categories of that kind and up to 30 past title → category
examples. The reply may only pick a listed id; a title is taken when ≤ 40 runes and the
category was not learned. Each draft is asked once (`ai_tried`). A failure pauses the AI
for 5 minutes.

---

## 10. Settings (`settings.md`)

| Key | Values |
|---|---|
| `default_period` | weekly · monthly · quarterly · halfyearly · yearly |
| `week_start`, `month_start_day`, `year_start_month` | weekday name · 1–28 · 1–12 |
| `upcoming_days`, `balance_check_days` | 1–365 (30) · 30 |
| `theme` | auto · light · dark (used by the nss palette) |
| `hide_amounts`, `home_locked` | booleans |
| `nav_order` | order of `home tx accounts reports calendar categories budgets`; first four = phone bottom bar |
| `home` | list of `{type, ref?, title?, style?}`, max 40: built-ins `networth period inbox spending budgets upcoming cards accounts recent cashflow` (once each) or `account` / `category` with a `ref`; empty = default layout |
| `inbox_expense_category`, `inbox_income_category` | fallback category ids (kind checked) |
| `inbox_sms_time` | `HH:MM`, or empty = as soon as SMS arrive |
| `look` | `palette` nss·amoled·galaxy·white·gold·blackpink · `cards` none·glass·matte·opaque · `density` comfortable·compact · `colors{accent,bg,surface,text,muted,line,income,expense,warn → #rrggbb}` · `charts{spending,budgets,cashflow,networth,reports → type}` |
| `alerts` | `time` (09:00) · `mismatch` (true) · `count` (30, 0 = off) |

The server validates everything against the lists in `internal/model`; the app reads the
allowed values from `state.look_options`. Home sections pointing at a deleted account or
category are dropped at the next settings save.

---

## 11. Front end (`web/static`)

No build step, no CDN: ES modules, one CSS file, hand-drawn SVG, bundled fonts.

```
about.html                  public start page (server fills {{VERSION}} {{APK}} {{REPO}} {{DEMO}})
app.html                    shell: loads css/app.css and js/main.js
sw.js                       service worker (cache name arthik-v10; every file listed in FILES)
manifest.webmanifest        PWA manifest + shortcuts
css/app.css                 all styles; palettes, card styles, density, motion
fonts/                      Plus Jakarta Sans + JetBrains Mono (woff2, OFL)
js/main.js                  boot, sign-in, shell, hash router, banners, badges, offline sync loop, back button
js/api.js                   fetch wrapper (X-Arthik), ApiError, offline queue, device key storage
js/ui.js                    store S, loadState, applyLook, lookups, toasts, sheets, period bar, celebrate
js/util.js                  html`` templating (escapes every value), money/date helpers, calc, newId
js/charts.js                SVG charts; js/icons.js line icons (Lucide paths)
js/txform.js                transaction sheet (add/edit/duplicate, splits, calculator, suggestions, draft review)
js/txlist.js                day-grouped transaction list; js/picker.js category/account picker
js/plugins.js               plugin registry; js/plugins/loans.js
js/native.js                Android bridge wrapper (no-op in a browser); js/appmode.js welcome / lock / engine error
js/views/                   dashboard transactions accounts categories budgets inbox notifications reports
                            calendar settings look menu phone  — one module per screen or settings card
```

- **Boot**: `GET /api/me` → not signed in: login screen (or the app gate when `local`) →
  `GET /api/state` into `S.state` → load plugin modules → shell → route → flush the
  offline queue. A 401 on any call fires `arthik:signedout` and returns to sign-in.
- **Router**: hash routes `#/home`, `#/tx`, `#/account/<id>`, `#/category/<id>`,
  `#/reports/<tab>`, `#/inbox`, `#/notifications`, `#/p/<plugin>`, `#/settings` …; each view
  exports `render(root, ctx)` (`ctx.alive()` guards against a late render). Mobile: bottom
  bar (first 4 of `nav_order` + Settings) and the ＋ button; ≥ 900 px: sidebar. Inbox tray
  and 🔔 bell in the top bar with counts; the eye toggles `hide_amounts`.
- **Home** (`views/dashboard.js`): blocks from `settings.home`; only the data the chosen
  blocks need is fetched; ✎ edits (hidden when `home_locked`).
- **Inbox** (`views/inbox.js`): Accept / Edit (the transaction form in draft mode posts to
  `/accept`) / Same / Reject / Not a transfer; *Done* tab with Restore; swipe right accepts,
  left rejects; *Accept N ready*; the "How drafts arrive" card with *Run now*.
- **Transactions**: the add form's title asks `/api/suggest`; picking sets only the title
  and category. Category picker: "Most used" from `state.category_uses` (90 days), "Recent"
  from this device.
- **Offline (PWA)**: the service worker caches the shell and the last answer of each
  `GET /api/*` (network first; exports always go to the network). Transaction
  creates/edits/deletes made offline go to a localStorage queue (`arthik.queue.v1`) and
  are replayed in order (`online` event, every 30 s, at start); creates carry a client id,
  so a replay returns `duplicate: true`. A change the server rejects is dropped and
  reported. Other edits need a connection.
- **Device-local storage**: `arthik.queue.v1`, `arthik.device_key`, `arthik.lastAccount`,
  the picker's recent list, the home trend span.
- **Look**: nine base colour tokens on `:root` per palette; every other colour is
  `color-mix`ed from them, so `settings.look.colors` overrides (set inline on `<html>`)
  flow everywhere. `data-palette`, `data-theme`, `data-cards`, `data-density` on `<html>`.
  In the phone app `applyLook` sends surface, opacity, text, muted and accent to
  `ArthikNative.setWidgetTheme`.
- **Charts**: hand-drawn SVG at the box's pixel width, with tooltips; the type per chart
  comes from `settings.look.charts`.
- **Motion**: screen fade-up, sliding sheets, growing bars, count-up amounts, toasts;
  all off under `prefers-reduced-motion`.
- **App mode** (`window.ArthikNative` present): the sign-in screen becomes the app gate,
  Settings gets the Phone card, sign-out becomes lock, no service worker, and
  `window.arthikBack()` answers the Android back button.

---

## 12. Plugins

A plugin is `internal/plugin/<id>/` calling `plugin.Add` in `init` (blank-imported in
`server.go`); its routes are always registered, and it is listed in `state.plugins` only
when its data file exists. The web module `js/plugins/<id>.js` calls
`register(id, {title, icon, render, formHook})`; the screen is `#/p/<id>`.

**Loans** (`loans.md`, read only): per loan — lender details, `account` (the Arthik loan
account), `pay_from`, `interest_category`, `rate`, `emi`, `emi_day`, `fy_start_month`,
`statement{as_of, principal_outstanding, unpaid_installments, fy_principal_paid,
fy_interest_paid}`, `schedule` rows and `schedule_text` (the lender's table pasted as
is). Rows merge by due date and are checked for continuity; the projection continues
from the last row at the latest rate until payoff (max 480 rows). The view adds EMIs
left, interest left, payoff date, next EMI and this financial year's principal and
interest (statement figures + what Arthik recorded after `as_of`). `split` finds the EMI
of that amount within 16 days and returns principal (→ loan account) and interest
(→ category) for *Split as EMI*.

---

## 13. Android app (`android/`)

> From `android/README.md`, the manifest and the web bridge; Kotlin not re-read on 2026-11-16.

Package `com.nihars.arthik` (debug: `.debug`), minSdk 26, targetSdk/compileSdk 34, no
AndroidX, JDK 17, Gradle 8.10.2. `versionName` = `VERSION`; `versionCode` =
`20000 + major·10000 + minor·100 + patch` (1.0.5 → 30005).

- **Engine**: the same `cmd/arthik` built per ABI as `jniLibs/<abi>/libarthik.so`
  (`arm64-v8a` GOOS=android; `armeabi-v7a` and `x86_64` as static linux binaries), run on
  `127.0.0.1:<port>` with `ARTHIK_MODE=local`. The app opens a session with
  `GET /local/enter?token=…&user=me|demouser[&to=screen]`; the engine exits when the app
  closes its stdin.
- **Local mode in the engine**: no passwords, no guard, no start page (`/` → `/app`);
  token ≥ 16 chars, constant-time check; cookie `SameSite=Strict`, 400-day expiry; the
  signing secret is random per start unless `ARTHIK_SECRET` is set, so each launch needs
  the app; the demo is always on (`demouser`, `ARTHIK_DEMO_DIR`); `/api/me` says
  `local: true`.
- **Roles**: replica by default (*online mode*); *Settings → Phone → this phone is the
  main device* makes it primary (*offline mode*).
- **Kotlin files**: `MainActivity` (WebView, start-up, picker, back, downloads, bars) ·
  `Bridge` (`window.ArthikNative.*`) · `Engine` · `ArthikApp` · `Prefs` · `AppConfig` ·
  `Storage` · `Paths` · `Pin` · `Lock` · `SmsRules` · `SmsStore` · `SmsSync` ·
  `DailyAlarm` · `Summary` · `Rupees` · `Widget` · `WidgetLook` · `InboxNotify` ·
  `Downloads` · `Pages`. Unit tests: AppConfig, Paths, Pin, Sms, Summary, WidgetLook.
- **Receivers**: `SmsAlarm`, `NotifyAlarm`, `Reschedule` (boot, package replaced, time or
  zone change, exact-alarm permission change) and four widget providers (`ArthikWidget`
  4×2, `BudgetWidget`, `NetWorthWidget`, `InboxWidget` 2×2).
- **Permissions**: INTERNET (loopback only), MANAGE_EXTERNAL_STORAGE (READ/WRITE on
  Android ≤ 10), USE_BIOMETRIC, READ_SMS, SCHEDULE_EXACT_ALARM, RECEIVE_BOOT_COMPLETED,
  POST_NOTIFICATIONS. `allowBackup=false`.
- **Bridge methods the web UI calls**: `config saveConfig loadConfig chooseConfigDir
  setConfigDir clearConfigDir · chooseFolder useFolder setFolderPath useDemo
  requestFilesAccess · enter unlock unlockBiometric unlockDevice lockNow setPin clearLock
  setBiometric setDeviceLock setLockAfter · setPrimary restartEngine log · requestSms
  setSms setSmsTime setSmsLookback smsRunNow openExactAlarms openAppSettings · setNotify
  setNotifyTime notifyNow · setWidgetTheme refreshWidget · setMaterialYou accent setBars
  haptic`. Native → web events arrive through `window.arthikNative.onEvent`.
- **Phone-only work**: the daily bank-SMS copy into `inbox/sms_<device>.md`, the daily
  alert (reads `alert` from `arthik summary`), and the widgets (same `summary` command,
  read-only).

---

## 14. Security

- **Passwords**: owner from `.env`; bcrypt (cost 12 from `arthik hash-password`) or plain
  text compared in constant time; every failure sleeps 400 ms.
- **Sessions**: cookie `arthik_session` = base64url(`user|expiry|kind|HMAC-SHA256`),
  `HttpOnly`, `SameSite=Lax`, `Secure` when the request came over TLS or with
  `X-Forwarded-Proto: https`. No session store. A normal sign-in lasts
  `ARTHIK_SESSION_HOURS`. A remembered device (device key at sign-in or via
  `POST /api/device-key`) has no expiry; its `kind` is a tag derived from the key, so a
  new key in `.env` ends them all; the cookie (400 days) is renewed on each `/api/me`.
- **CSRF**: every non-GET needs header `X-Arthik`; read-only users can change nothing.
- **Lock-out** (`guard`): 5 failed sign-ins or wrong device keys per IP within 24 h → the
  IP gets 403 on every path but `/healthz` for `ARTHIK_BAN_DAYS`; a good sign-in resets
  the count; failures are in memory, bans persist in `/state/bans.json`; only
  `arthik unban` (or editing the file — the server re-reads it when it changes) lifts one.
- **Client IP**: forwarding headers are believed only when the peer is in
  `ARTHIK_TRUSTED_PROXIES`: `CF-Connecting-IP`, then the right-most untrusted
  `X-Forwarded-For` hop.
- **Browser**: CSP `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';
  img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'`,
  `nosniff`, no framing; every templated value is escaped by `html```.
- **Container**: your uid, read-only root fs, no capabilities, only `ARTHIK_*` + `TZ` in
  the environment.
- **Phone engine**: no passwords, bans or expiry; loopback only; the app lock (PIN hashed
  with PBKDF2, or biometrics/device lock) guards it.
- **Public repo hygiene**: `scripts/check-public.sh` (pre-push) blocks tracked `.env`,
  keys, patch zips, password-looking values and words in `.private-words`.

---

## 15. Build, release, CI

| Piece | What it does |
|---|---|
| `scripts/dev.sh` | `go run` on a throwaway folder, `127.0.0.1:8080`, `me` / `dev` + the demo |
| `scripts/release.sh "vX.Y.Z notes"` | on `main`, up to date, tag free → writes `VERSION`, adds a CHANGELOG section if missing, `go vet` + `go test` when Go is installed, `check-public.sh`, commit, annotated tag, push `main` and the tag |
| `.gitlab-ci.yml` `test` | `golang:1.26`: `go vet ./... && go test ./...` on every push, MR and tag (shared runner) |
| `release-files` (tag `vX.Y.Z`) | on gtlbrunner (runner tag `arthik-app`, image `arthik-builder:1`): tag must equal `VERSION` → `scripts/build-in-container.sh ci` → `scripts/upload-release.sh` |
| `scripts/build-in-container.sh` | needs `BUILD_CACHE` and `KEY_DIR` (`arthik.jks` + `keystore.env`); runs `android/scripts/build-in-container.sh release`; leaves `dist/release/arthik-vX.Y.Z.apk` + `SHA256SUMS` |
| `android/scripts/build-in-container.sh` | Go vet + tests → engine per ABI (`ARTHIK_ABIS`) → Gradle unit tests → signed APK → `android/dist/` (`ARTHIK_QUICK=1` skips the tests) |
| `scripts/upload-release.sh` | uploads to the generic package registry (`arthik/<tag>/`), writes `release-notes.md` from the CHANGELOG section |
| `release` | GitLab release with those notes; APK link with `direct_asset_path: /arthik.apk` → `/-/releases/permalink/latest/downloads/arthik.apk` |
| `android/build.sh [keystore\|release\|debug\|quick]` | the same build locally in Docker (image `arthik-android-builder` from `docker/builder/Dockerfile`); key in `$KEYS_HOME/.arthik-keys` |

Nimory patch flow (outside git): a versioned zip of changed files at repo paths in
`patch/` (gitignored), applied with `unzip -o`, then `docker compose up -d --build`.

---

## 16. Tests

82 Go tests in 17 files; no tests yet for `config`, `dates`, `ai`, `auth` (covered through
`server`), `query`, `view`, `store`, the feature packages other than `accounts`,
`review` and `summary`, or any JavaScript.

| Package | Covers |
|---|---|
| `money`, `period` | parsing, formatting, MulDiv; periods, shifts, labels, daily buckets |
| `book` | double entry, unbalanced rejected, future dates, broken file never overwritten, external edit repaired, unknown fields survive, entry round trip, read-only, split into account, retired keys; sync: conflict merge, newest wins, tombstones + TTL, moved date, definition union, replica in memory only |
| `inbox` | category priority, ingest once, pair/unpair, accept learns and is idempotent, match/reject/restore, balance warning, runner (files, AI, rescan, SMS schedule, late sync, run request, replica never imports, rescan leaves drafts.md alone), SMS samples, ignores, user rules, Key/Pretty, phone and Kronos file formats, confidence, notices |
| `guard` | ban after max fails survives restart, expiry, reset, exempt, old failures expire and are swept, ClientIP |
| `server` | API walk-through, local mode, suggest, nav order, inbox API, home layout, landing page, balance check, look and alerts, running balance; security: ban, device key, changed key |
| `feature/accounts`, `feature/review`, `feature/summary`, `plugin/loans` | deleted account forgotten; rule paging; summary build and alert; projection, split, FY, appended schedule, schedule problems |
| `e2e` | two folders with simulated Syncthing: online entry from the phone, offline phone then reconnect, account added offline, inbox across devices |
| Android (JUnit) | AppConfig, Paths, Pin, SMS rules + store, summary texts, widget look |

---

## 17. Adding things

**A feature**
1. `internal/feature/<name>/<name>.go` with `func Register(rt *httpx.Router)`; read with
   `c.Book.View`, change with `c.Book.Mutate(func(d *book.Data, ch *book.Change) error)`
   following the rules in §7.
2. Add `<name>.Register(rt)` in `internal/server/server.go`.
3. Screen: `web/static/js/views/<name>.js` exporting `render(root, ctx)`, a route in
   `main.js` (`ROUTES`, and `NAV_ITEMS` + `model.NavItems` if it belongs in the menu), and
   the file listed in `sw.js` `FILES`.
4. Tests: `internal/book/book_test.go` (engine) and `internal/server/server_test.go` (API).

**A settings key**: field in `model.Settings` (+ default, + `normSettings`), validation in
`feature/settings`, and — if a form may omit it — the "keep the saved one" block in the
`PUT` handler.

**A stored field**: add it to the `model` struct with `yaml`/`json` tags; never rename a
key without adding the old one to the `Retired*Keys` lists.

**An SMS pattern**: a rule in `inbox/sms_rules.md` (yours) or `sms_rules.yaml` (built in)
plus a sample in `internal/inbox/sms_test.go`.

**A personal plugin**: see §12.

**A release**: write the `## X.Y.Z` CHANGELOG section, then `scripts/release.sh`. The
version changes only there (`VERSION`); the Android version code follows from it.

---

## 18. Known limits (by design)

- Web PWA: only transaction changes queue offline (the Android app has no such limit).
- Edits to the *same account/category* on two devices while apart: main copy wins.
  `settings.md` is saved whole, so the last save wins.
- One person per dataset (the demo is a separate folder); INR only; no attachments.
- The watcher polls (default 3 s); everything is in memory (personal scale).
- SMS reading runs only on the phone app. A message the patterns cannot read is skipped
  unless the catch-all finds an amount and a direction.
- The AI runs on the main instance only and only when `ARTHIK_AI_URL`/`ARTHIK_AI_MODEL` are set.
- Report buckets are capped at 400 per request.
- Display name is "Arthik"; binary, package, URLs, cookies and storage keys stay lowercase.

---

## 19. Review log

### Fixed in 1.0.6 (from the 2026-11-16 review)

- A replica no longer imports the inbox: `Runner.Run` returns at once unless
  `Book.Main()`, so `Runs(dir)` is false there and `POST /api/inbox/import` writes
  `inbox/run_request.md` (`TestReplicaNeverImports`).
- `inbox/drafts.md` is no longer rewritten at every rescan: `sms_imported` is stamped only
  when a daily time is set or drafts were added (`TestRescanLeavesDraftsAlone`).
  `Status.LastSMS` (in memory) still shows the last read.
- Deleting an account clears it from pending drafts, reminders and learned rules
  (`accounts.forget`, `TestForget`).
- Sign-in with an unknown user spends one bcrypt check (`Auth.burn`); `guard` sweeps
  addresses whose failures are all older than 24 h once more than 1000 are held.
- `hash-password` text, start-page and manifest wording, the `balance_check_days` error
  text, dead code (`state.nonNil`, `book.parseStamp`, the `fixup` parameter, the
  `scheduled` mapping in `main.js`) and the stale comments.
- Server image on Go 1.26 / Alpine 3.23; CI `test` on `golang:1.26`.

### Still open

1. `docker/builder/Dockerfile` still installs Go 1.23.4 for the phone engine. Bumping it
   means a new `arthik-builder:N` here and in gtlbrunner, then rebuilding that image.
2. `golang.org/x/crypto` is pinned at v0.31.0 (only bcrypt is used). A newer one pulls
   `golang.org/x/*` modules that the GitHub-mirror `replace` lines do not cover yet.
3. CI `test` runs only Go; the Android unit tests run only inside a release build.
   `registry.gitlab.com/gitlab-org/cli:latest` is unpinned.
4. CHANGELOG 1.0.3 says `release.sh` skips local tests on an old Go and pushes to every
   push URL of `origin`; the current script does neither.
5. An old tag `v1.3.6` (from before the numbering restarted) sorts above every 1.0.x tag.
6. `/api/state` recomputes each account's bank check and card cycle by scanning every
   transaction. Fine today; the first place to cache if it gets slow.
7. `PUT /api/settings` treats `balance_check_days: 0` as "keep the saved value".
8. `inbox.ReadSources` is only called by its test.
9. The Kotlin sources have not been reviewed in this pass (§13).
