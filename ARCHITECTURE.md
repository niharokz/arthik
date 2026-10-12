# Arthik — Architecture (internal)

> Internal reference for the Nimory homelab. Share this file (plus only the files being
> changed) instead of the full source. No secret values — only names and paths.
> Last reviewed: 2026-10-11 (v1.2.1: inbox for SMS + Kronos drafts, learned rules, optional Ollama, reminders removed, device lock + nightly SMS on Android, new logo).

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
| Network | external `homelab`; Caddy proxies `arthik.nihars.com` / `arthik.nimory` → `arthik:8080` (no host port) |
| `.env` | compose: `FINANCE_PATH PUID PGID TZ` · app: `ARTHIK_USER ARTHIK_PASSWORD ARTHIK_SECRET ARTHIK_DEMO ARTHIK_DEMO_USER ARTHIK_DEMO_PASSWORD ARTHIK_WATCH_SECONDS ARTHIK_PRIMARY` |
| Health | `GET /healthz`; `arthik health` is the Docker healthcheck |
| Logs | stdout (json-file 10 MB × 3) |

Rebuild after code changes (`docker compose up -d --build`); restart after `.env` changes.
Front-end changes need a rebuild too (assets are embedded). Bump `VERSION` in `web/static/sw.js`
only if you want installed PWAs to drop their offline cache.

---

## 3. Code map

```
cmd/arthik/main.go          CLI: serve | hash-password | check [--demo] | rebuild | demo-seed | health | summary | parse-sms | version
internal/config/            .env loader + Config (container-side values only)
internal/money/             Amount = int64 paise; Parse/String/Format(₹ Indian grouping)/MulDiv; YAML+JSON as "123.45"
internal/dates/             Date (calendar day), AddMonths with anchor day
internal/period/            weekly|monthly|quarterly|halfyearly|yearly|custom ranges; week/month/year start opts; Buckets; Resolve
internal/store/             YAML .md read/write: FileError(file,line), atomic write, skip-if-unchanged, emoji unescape
internal/model/             records as stored: Settings, Account(+AccountTypes, sms_digits), Category, Label, Posting, Transaction, Draft, Rule
internal/book/              the ledger engine (one Book per dataset)
  book.go                     load/reload, fixups, Mutate (copy → validate → persist touched files → recompute), DryRun
  entry.go                    Entry (UI shape) ↔ Transaction (postings): BuildTx, EntryOf, ValidateTx, ids, EnsureLabels
  compute.go                  Computed: balances, cleared, projected, net worth, period totals; writes computed fields + money.md
  schedule.go                 RunScheduler: recompute when the day changes (future-dated transactions start counting)
  digits.go                   SMS digits on accounts: CleanDigits, DigitsFree, AccountByDigits
  watch.go                    polling watcher for external edits (acts once a change is stable for one tick)
  sync.go                     multi-device merge: stamps, tombstones, conflict copies, main/replica
internal/inbox/             the review inbox (no HTTP): sources.go reads inbox/kronos.md + inbox/sms_*.md → Items;
                              sms.go + sms_rules.yaml parse SMS (data-driven, user file inbox/sms_rules.md first);
                              engine.go Ingest/Categorise/pair/purge; review.go Accept/Match/Reject/Restore/Unpair/learn;
                              view.go duplicates ("same") + warnings (balance check); runner.go background import + AI
internal/ai/                Ollama /api/chat client: picks a category id from the given list + a clean title (JSON)
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
  review       GET /api/inbox?status=pending|done|all, POST /api/inbox/{id}/{accept,match,reject,restore,unpair},
               GET /api/inbox/rules?q=&offset=&limit= (paged, most used first), DELETE /api/inbox/rules/{id};
               drafts carry confidence high|medium|low + checks (view.go)
  budgets      GET /api/budgets
  reports      GET /api/reports/{cashflow,categories,trend,networth,balance-sheet,labels,accounts,calendar}
               (?bucket=daily|weekly|monthly|… ; daily only for reports)
  summary      GET /api/summary (+ `arthik summary`): month spend vs budget, inbox, net worth + 12-month trend (phone widgets/notice)
  settings     PUT /api/settings
  importexport GET /api/export.csv, POST /api/import (?dry_run=1&create_missing=1)
internal/e2e/               two-device Syncthing simulation tests (main + phone)
internal/demo/              demo generator (6 months, relative to today, deterministic per month; monthly bills; inbox drafts + rules)
internal/server/            mux: public pages, auth, features, security headers (CSP self-only)
web/static/                 embedded front end (no build step)
  about.html app.html sw.js manifest.webmanifest icon.svg icon-*.png css/app.css
  js/main.js(shell, router, sign-in, sync) util.js api.js(offline queue) ui.js charts.js txform.js txlist.js
  js/views/<feature>.js     one module per screen
  js/native.js appmode.js views/phone.js   Android app bridge, welcome/lock, Settings → Phone
android/                    Android app (Kotlin WebView shell + engine), see android/README.md
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
- **Recurring bills** are not Arthik's job (reminders were removed in 1.2.1): Kronos drops
  due subscriptions into the inbox and you accept them. Old `reminder:` keys on
  transactions survive as unknown keys; an old `reminders.md` is simply ignored.

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
| `inbox/drafts.md` | arthik (+ your decisions from any device) | whole file managed by arthik |
| `inbox/learned.md` | arthik (edit/delete freely) | `uses`, `updated` |
| `inbox/kronos.md` | **Kronos only** | — (read only for arthik) |
| `inbox/sms_<device>.md` | **phone app only** | — (read only for arthik) |
| `inbox/sms_rules.md` | you (optional) | — |
| `money.md` | arthik only | everything (not watched) |
| `transactions/transaction_YYYYMM.md` | user/app | — |
| `.demo-seed` (demo only) | arthik | month + seed version the demo was generated for |

Load rules: missing file = empty; parse error = `loadErr` → reads keep last good data, every
write refused with `file line N: …`, nothing written (a broken *conflict copy* is only a
warning); all transaction records — main files, conflict copies, stale copies in old months —
are resolved by id in `book/sync.go` (newest `updated`/`deleted` wins; tie → main file;
identical duplicates collapse; different records sharing an id → second renamed); missing
ids assigned, misfiled months moved, unknown labels added to labels.md; invalid
transactions → `problems`, excluded from totals. Unknown keys preserved (`Extra` inline maps). Amounts accept quoted
strings or bare numbers. IDs: accounts/categories/labels are slugs of the name
(`food-groceries` for a child); transactions `t-YYYYMMDD-xxxxxx`, client-created `c-…`.

### Multi-device sync (v1.1)

| Piece | Rule |
|---|---|
| `updated` | set by `Data.SetTx` on every change, UTC RFC 3339 with ms |
| tombstone | `DeleteTx` keeps `{id, date, deleted}` in the month file; purged by the main instance after 90 days |
| conflict copies | `transaction_YYYYMM.sync-conflict-*.md` and `<file>.sync-conflict-*.md` are read on load, merged, then deleted (main only) |
| main vs replica | `Book.Replica` (env `ARTHIK_PRIMARY=false`): replica never writes computed fields/money.md, never imports the inbox, never repairs or deletes files; it merges conflict copies in memory and saves only user changes (including accept/reject of drafts) |
| inbox conflicts | `inbox/drafts.sync-conflict-*.md` / `learned.sync-conflict-*.md` merge by id, newest `updated` wins; producer files are never merged (their owner rewrites them) |
| info notices | merges are reported as `Problem{Level: "info"}` (Data health), not as warnings |

Scenario tests: `internal/e2e` simulates Syncthing between two folders (main + phone),
including offline edits on both sides, deletes, edits of the same transaction, an
account created offline, and the inbox (phone SMS file → drafts on nimory; phone
accepts offline while nimory imports a Kronos draft; one transaction, rule learned).

### Inbox (v1.2.1)

```
 phone app ── nightly ──► inbox/sms_phone.md ┐
 Kronos ──── 02:12 ─────► inbox/kronos.md    ├─► main arthik: Ingest ─► inbox/drafts.md ─► Inbox screen
                                             ┘     (parse, account by digits,           accept → transaction d-<id>
                                                    categorise, pair, purge)            + inbox/learned.md
 Kronos ──── 07:00 ── reads drafts.md ──► Omnimo outbox ──► Telegram
```

- **Ids** are stable: SMS `sms-<16 hex>` (phone: sha256 of sender, time, text), Kronos
  `kronos-<event id>`. A draft is never re-imported; an accepted one becomes transaction
  `d-<draft id>`, so accepting on two devices makes one transaction.
- **Category order**: learned rule (key = `inbox.Key(merchant)`) → newest past transaction
  whose title matches → Kronos's category name → AI (`ai_tried`, only for drafts with the
  fallback or no category, never edited ones) → `settings.inbox_*_category`.
- **Account**: `sms_digits` suffix match; Kronos account names match account id/name; a
  learned rule's account only when the SMS has no digits. *Remember* on accept moves the
  digits onto the chosen account.
- **Pairing**: pending SMS expense from an asset account + same-amount income on another
  account or a card "payment received" within `ARTHIK_INBOX_PAIR_DAYS` → one transfer
  draft (`pair`); *Not a transfer* splits it and marks both edited.
- **Checks** (computed on view, never stored): likely duplicate (same account, same
  amount, ±1 day), unknown digits, catch-all pattern, bank balance / card limit vs
  Arthik's (including earlier pending drafts).
- **Retention**: source items older than `MAX_AGE` are ignored; reviewed drafts are
  purged after `KEEP_DAYS` (> `MAX_AGE`, enforced at start-up).

### Local mode (phone engine)

`ARTHIK_MODE=local` with `ARTHIK_LOCAL_TOKEN`, `ARTHIK_DEMO_DIR`, optional `ARTHIK_DATA`,
`ARTHIK_PRIMARY=false`, `ARTHIK_EXIT_ON_STDIN_EOF=1`. No passwords: the app opens a session
with `GET /local/enter?token=…&user=me|demouser` (constant-time token check, SameSite=Strict
cookie, secret random per start so every launch needs the app). `/` redirects to `/app`;
`/api/me` reports `local: true`. The process exits when the app closes its stdin. See
`android/README.md` for the app itself.

---

## 6. Front end

- Hash router (`#/home`, `#/tx`, `#/account/<id>`, `#/category/<id>`, `#/reports/<tab>` …).
  Mobile: bottom bar + ＋ button; ≥ 900 px: sidebar. Screen order comes from
  `settings.nav_order` (`main.js` `navOrder()`; edited in `views/menu.js`): first 4 = bottom
  bar, rest under More, Settings always last. Server validates keys against `model.NavItems`.
- Inbox (`views/inbox.js`): pending drafts with Accept / Edit (the transaction form in
  draft mode: `openTx({}, done, {draft})` posts to `/accept`) / Same / Reject / Not a
  transfer; *Done* tab with Restore. Menu badge = `state.inbox.pending`. Dashboard shows
  the newest drafts and future-dated transactions (the old "Upcoming reminders" card).
- Remembered transactions: the add form's title field asks `GET /api/suggest?q=&type=`
  (latest valid non-journal transaction per title, ranked prefix → same type → uses →
  recent). Picking fills all fields except date/time. Not shown when editing or offline.
- `html``…`` tagged template escapes every value (data comes from editable files).
- Offline: service worker caches the shell and the last answer of each `GET /api/*`
  (network first). Transaction creates/edits/deletes made offline go to a localStorage
  queue and are replayed in order (`online` event, every 30 s, at start). Creates carry a
  client id → server returns `duplicate: true` on replay. Other edits need a connection.
- Charts are hand-drawn SVG at the box's real pixel width (`charts.js`), with tooltips.
- Theme: nss teal `#3a807a`, square grid background, tokens on `:root`, dark mode auto or forced.
- Motion: new screens fade up with staggered cards/list rows, sheets slide up (pop on
  desktop) and slide out before removal, bars grow, chart bars rise and lines draw,
  stat amounts count up on first view, toasts slide in. All off under
  `prefers-reduced-motion`.
- App mode (`js/native.js`): when `window.ArthikNative` exists the sign-in screen becomes
  the app gate (`appmode.js`: welcome / lock / engine error), Settings gets the Phone card
  (`views/phone.js`), sign-out becomes lock, the service worker is not used and
  `window.arthikBack()` answers the Android back button.

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

- Web PWA: only transaction changes queue offline (the Android app has no such limit).
- Edits to the *same account/category* on two devices while apart: main copy wins.
- One person per dataset (the demo is a separate folder); INR only; no attachments (by design).
- Watcher is polling (default 3 s).
- SMS reading runs only on the phone app (the web app cannot read SMS). A message the
  patterns cannot read is skipped unless the catch-all finds an amount and a direction.
- The AI runs on the main instance only and only when `ARTHIK_AI_URL`/`ARTHIK_AI_MODEL` are set.
- Display name is "Arthik"; binary, package, URLs, cookies and storage keys stay lowercase.
