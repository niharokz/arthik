# Arthik for Android

The same Arthik you use at `arthik.nihars.com`, running **entirely on your phone**:
same screens, same features, works with no internet. It reads and writes the finance
folder that Syncthing keeps in sync with nimory, so the phone and the server always
show the same books.

## How it works

```
 ┌──────────────── phone ────────────────┐            ┌──────── nimory ────────┐
 │  arthik app (WebView, same UI)        │            │  arthik server (main)  │
 │        │  http://127.0.0.1:<port>     │            │                        │
 │  arthik engine (Go, libarthik.so)     │            │                        │
 │        │ reads / writes               │            │        │               │
 │  …/notes/data/finance   ◄──────── Syncthing ───────►  …/notes/data/finance  │
 └───────────────────────────────────────┘            └────────────────────────┘
```

- The APK contains the **same Go engine** as the server (built from `cmd/arthik`) plus
  a thin Kotlin shell. The engine starts when you open the app and stops when you
  close it. The only background work is the optional nightly SMS copy (below);
  nothing is sent anywhere except through your Syncthing folder.
- The phone is a **replica**: it saves what you enter (and your inbox decisions), but
  leaves writing balances, importing the inbox and merging sync conflicts to nimory
  (the **main device**). If you
  ever use the phone without the server, turn on *Settings → Phone → This phone is
  the main device*.
- **Offline safe.** Every transaction carries an `updated` time and deletions leave a
  small tombstone. When the phone and nimory both changed the same month file while
  apart, Syncthing saves a `.sync-conflict-…` copy; nimory merges it by transaction id
  (newest change wins) within seconds and deletes the copy. Nothing is lost.

## What you need

- Docker on the machine that builds the APK (nimory is fine). No Android Studio.
- No Google account, subscription or Play Store. You install the APK yourself.
- A phone with Android 8 or newer.
- nimory running Arthik **1.3.1** (same version as the app). Redeploy the server first.

## Build the APK

```sh
cd /home/datar/docker/arthik          # the repo
cp android/.env.example android/.env  # set ARTHIK_KEYSTORE_PASSWORD
./android/build.sh keystore           # once: creates ~/.arthik-keys/arthik.jks
./android/build.sh                    # → android/dist/arthik-<version>.apk
```

The first build downloads the Android SDK, Gradle and Go modules (a few minutes, cached
in `android/.cache` afterwards). Each build runs the Go tests and the Android unit tests
before it packages anything.

**Back up `~/.arthik-keys/arthik.jks` and its password.** Android only accepts an
update signed with the same key. (If you lose it: uninstall, install the new APK and
pick the folder again — your data lives in the folder, so nothing is lost.)

`./android/build.sh debug` builds a debug APK without a key; it installs next to the
release app as a second "Arthik" (`com.nihars.arthik.debug`).

## Install on the phone

1. Copy `arthik-<version>.apk` to the phone — easiest is a Syncthing folder.
2. Tap it in the Files app. Allow **Install unknown apps** for that app when asked (once).
3. Play Protect may say "unrecognised developer": **More details → Install anyway**.
4. Open Arthik. Choose **Explore the demo** to look around, or **Use my finance folder**:
   allow **All files access**, then pick `…/notes/data/finance` in the folder picker.

To update, install the new APK over the old one. Settings and data are kept.

> From 2027 Google will require apps installed outside the Play Store to come from a
> registered developer (a free "hobbyist" account covers this), or to be installed with
> `adb install arthik.apk` over USB. Check developer.android.com/developer-verification.

## Permissions to give

| Permission | Where Android asks | Why |
|---|---|---|
| **All files access** | Settings → Phone → File access → *Allow* (system screen: Arthik → on) | read and write the Syncthing finance folder |
| **SMS** (optional) | Settings → Phone → Bank SMS → *Allow* | the nightly copy of bank SMS |
| **Allow restricted settings** (Android 13+, once) | App info → ⋮ → *Allow restricted settings* | Android blocks SMS for apps not from the Play Store until you do this; the app shows the steps and opens App info |
| **Alarms & reminders** (optional, Android 12+) | Settings → Phone → Bank SMS → *Allow exact time* | run at the exact minute; without it Android runs it within ~15 minutes |
| **Notifications** (optional, Android 13+ asks) | Settings → Phone → Inbox notice → switch on | the morning inbox notice |
| **Battery: Unrestricted** (recommended) | App info → Battery → *Unrestricted* | some phones (Xiaomi, OnePlus, Samsung) otherwise skip background alarms |
| Fingerprint/face | nothing to grant | uses what is enrolled on the phone |

## Nightly bank SMS → Inbox

Settings → Phone → **Bank SMS → Inbox**: allow SMS, switch on *Copy new bank SMS every
night*, pick the time (default 23:30).

- At that time the app copies **new** messages from named senders (AX-HDFCBK …) that
  mention an amount, skipping OTPs, into `inbox/sms_phone.md` in your finance folder.
  It never sends anything anywhere; Syncthing carries the file to nimory, where Arthik
  turns the messages into drafts (account, category, title) for the **Inbox** screen.
- **No duplicates, no back-fill.** Each run reads only messages after the last
  *successful* run. Switching it on starts from that day; nothing older is ever read.
  If a night is missed (phone off), the next run picks up the gap — at most *N* days
  back (Settings: 1–7, default 3). Every message also has a fixed id, and nimory never
  imports an id twice.
- *Run now* copies immediately (handy the first time). The card shows the last result.
- The file keeps 45 days of messages; only the phone writes it.

## Morning inbox notice and widget

- **Notice.** Settings → Phone → **Inbox notice & widget** → *Morning notification*
  (default 07:00). Once a day the phone reads the inbox from your folder and shows
  "6 drafts to review · ₹4,594 out" with the newest few; tapping it opens the Inbox.
  Nothing is shown when the inbox is empty. *Show now* previews it. If you use it,
  remove Kronos's `arthik_inbox_notify` job (Telegram) so you aren't told twice.
- **Widgets.** Long-press the home screen → Widgets → Arthik:
  - **Overview** (4×2): this month's spending against your budgets, a budget bar and the inbox. Tap → Inbox.
  - **Budget** (2×2): spent, of how much, what's left. Tap → Budgets.
  - **Net worth** (2×2): now, this month's change, a 12-month trend. Tap → Overview.
  - **Inbox** (2×2): drafts waiting, money out, the newest. Tap → Inbox.

  They refresh hourly and whenever you leave the app. **Settings → Phone → Widgets**
  sets their look for all of them: Auto (follows light/dark), Light, Dark, AMOLED
  (pure black), Glass (35% black), Clear (no background) or Custom — any colour,
  opacity 0–100% and light/dark/auto text — with a live preview.
- Both read the folder with the engine's read-only `summary` command, so the numbers
  are exactly the app's, and nothing is written.

## Look

- **Edge to edge** (Android 11+): the app draws behind the status and navigation bars,
  whose icons follow the app's light/dark theme.
- **Material You** (Android 12+): Settings → Phone → *Use my phone's colours* swaps
  Arthik teal for your wallpaper accent.
- The launch screen shows the ₹ hand logo where Android's own splash put it, then
  the app slides in.

## First run and settings

| Screen | What it does |
|---|---|
| Welcome | Demo (read-only sample data, generated on the phone) or your own folder. |
| Settings → Phone | Change folder, switch to demo, file access, main-device switch, app lock, phone colours, inbox notice, widget, bank SMS copy, engine log, restart. |
| Lock | Either a 4–8 digit Arthik PIN (stored hashed, optional fingerprint on top; five wrong PINs pause entry for 30 s) or **Fingerprint / face** with the phone's own screen lock as backup (needs a screen lock on the phone). Locks again after the delay you choose. |

Phone settings live in the app, not in the synced folder, so the phone and nimory
never fight over them.

Everything else — accounts, categories, budgets, inbox, reports, calendar,
import/export (saved to *Downloads*) — is the same as on the web.

## Troubleshooting

| Problem | Fix |
|---|---|
| "Arthik needs file access" | Settings → Phone → File access → Allow (Android: *All files access* → Arthik on). |
| "The engine couldn't open …" | The folder moved or access was removed. Choose the folder again; the log shows the exact error. |
| Changes from nimory don't appear | Check Syncthing is running on the phone; Arthik picks up synced files within 2 s. |
| A conflict notice in Data health | Informational: the phone includes conflict copies in its totals until nimory merges them. |
| SMS “Allow” does nothing | Android 13+: App info → ⋮ → Allow restricted settings, then Permissions → SMS → Allow. |
| No SMS drafts appear | Settings → Phone → Bank SMS: check *Last run*. Then check `inbox/sms_phone.md` synced to nimory, and `docker exec arthik arthik parse-sms "<the message>"` on nimory. |
| The nightly run is late or skipped | Allow exact time; set Battery → Unrestricted. |
| No morning notice | Settings → Phone → Inbox notice: check *Last notice*; allow notifications in App info; set Battery → Unrestricted. |
| Widget says "choose your finance folder" | Open the app once with your folder; the widget then updates. |
| Something odd | Settings → Phone → View log (copy it into an issue/chat). |

## For developers

```
android/
  build.sh                      one-command build (Docker)
  Dockerfile                    toolchain image: JDK 17, Android SDK 34, Go
  scripts/build-in-container.sh Go tests → engine per CPU → unit tests → APK
  app/build.gradle.kts          version = cmd/arthik version; signing from env
  app/src/main/java/com/nihars/arthik/
    MainActivity.kt   the screen: WebView, start-up flow, picker, back, downloads, bars
    Bridge.kt         window.ArthikNative.* methods used by web/static/js/native.js
    Engine.kt         starts/stops the Go engine, waits until ready, rotating log
    ArthikApp.kt      process-wide state (engine survives screen recreation)
    Prefs.kt          phone-only settings
    Storage.kt        file access permission, folder picker → path, folder checks
    Paths.kt          picker id → path (pure, tested)
    Pin.kt            PBKDF2 PIN hashing (pure, tested)
    Lock.kt           fingerprint prompt; device lock (biometric + screen lock)
    SmsRules.kt       what to copy, the read window, next run time, YAML quoting (pure, tested)
    SmsStore.kt       inbox/sms_<device>.md writer: add once, prune, atomic (tested)
    SmsSync.kt        reads the SMS inbox, the nightly run, SmsAlarm receiver
    DailyAlarm.kt     once-a-day alarms (SMS copy, notice) + Reschedule receiver (boot/update)
    Summary.kt        runs `libarthik.so summary`: month spend vs budget, inbox (texts tested)
    Rupees.kt         amounts in/out of the engine, ₹ with Indian grouping (pure, tested)
    Widget.kt         home-screen widgets: Overview, Budget, NetWorth, Inbox providers + drawing
    WidgetLook.kt     widget background/opacity/text colours (pure, tested)
    InboxNotify.kt    morning inbox notice + NotifyAlarm receiver
    Downloads.kt      saves CSV exports to Downloads
    Pages.kt          fallback page if the engine can't start
  app/src/main/res/    widget layouts (4×2 + three 2×2), splash logo, notification icon, themes (v31 splash)
  app/src/test/…      JUnit tests (Paths, Pin, SMS rules + store, summary texts, widget look)
```

The web UI is not duplicated: `web/static` is embedded in the engine. The app-only
screens (welcome, lock, Settings → Phone) are in `web/static/js/appmode.js` and
`views/phone.js`, shown only when `window.ArthikNative` exists.

Open `android/` in Android Studio to work on the Kotlin side; run
`./android/build.sh debug` once first so `jniLibs/` contains the engine.
