package com.nihars.arthik

import android.os.Build
import android.os.SystemClock
import android.webkit.JavascriptInterface
import org.json.JSONObject
import java.time.LocalDate
import java.time.ZoneId

/**
 * Methods the web app calls as window.ArthikNative.* (see web/static/js/native.js).
 * They run on a WebView background thread; anything touching the screen is posted
 * to the main thread by MainActivity.
 */
class Bridge(private val a: MainActivity) {
    private val app get() = a.arthik
    private val prefs get() = app.prefs
    private var failedPins = 0
    private var blockedUntil = 0L

    /** Phone-side settings as JSON (read by the welcome, lock and Settings → Phone screens). */
    @JavascriptInterface
    fun config(): String = JSONObject().apply {
        put("mode", prefs.mode)
        put("folder", prefs.folder)
        put("folder_ok", Storage.looksUsable(prefs.folder))
        put("files_access", Storage.hasAccess(a))
        put("primary", prefs.primary)
        put("lock", if (prefs.lockOn) prefs.lockMode else "none")
        put("device_lock_available", Lock.deviceSecure(a))
        put("locked", prefs.lockOn && !app.unlocked)
        put("pin_length", prefs.pinLength)
        put("biometric_available", Lock.biometricAvailable(a))
        put("biometric_on", prefs.biometric && Lock.biometricAvailable(a))
        put("lock_after", prefs.lockAfter)
        put("version", a.versionLabel())
        put("device", "${Build.MANUFACTURER} ${Build.MODEL} · Android ${Build.VERSION.RELEASE}")
        put("engine_error", app.engineError)
        put("sms", smsJson())
        put("notify", notifyJson())
        put("accent", accentJson())
        put("widget", Widget.placed(a))
        put("widget_look", widgetLookJson())
        put("config", configJson())
    }.toString()

    private fun configJson() = JSONObject().apply {
        val dir = prefs.configDir
        put("dir", dir)
        put("file", AppConfig.FILE)
        put("exists", dir.isNotEmpty() && java.io.File(dir, AppConfig.FILE).isFile)
        put("error", app.configError)
    }

    private fun notifyJson() = JSONObject().apply {
        put("enabled", prefs.notifyEnabled)
        put("permission", InboxNotify.permitted(a))
        put("hour", prefs.notifyHour)
        put("minute", prefs.notifyMinute)
        put("next_run", if (prefs.notifyEnabled) prefs.notifyNextRun else 0)
        put("last_result", prefs.notifyLastResult)
        put("exact", DailyAlarm.exactAllowed(a))
    }

    // The phone's wallpaper colours (Material You) for the page, Android 12+.
    private fun accentJson() = JSONObject().apply {
        val available = Build.VERSION.SDK_INT >= 31
        put("available", available)
        put("on", available && prefs.materialYou)
        if (available) {
            put("light", hex(a.getColor(android.R.color.system_accent1_600)))
            put("dark", hex(a.getColor(android.R.color.system_accent1_200)))
        }
    }

    private fun widgetLookJson() = JSONObject().apply {
        put("bg", prefs.widgetBg)
        put("opacity", prefs.widgetOpacity)
        put("text", prefs.widgetText)
        put("presets", JSONObject().apply {
            WidgetLook.PRESETS.forEach { (id, p) -> put(id, JSONObject().put("bg", p.first).put("opacity", p.second).put("text", p.third)) }
        })
    }

    private fun hex(c: Int) = String.format("#%06X", c and 0xFFFFFF)

    private fun smsJson() = JSONObject().apply {
        put("permission", SmsSync.permitted(a))
        put("enabled", prefs.smsEnabled)
        put("hour", prefs.smsHour)
        put("minute", prefs.smsMinute)
        put("lookback", prefs.smsLookback)
        put("device", prefs.smsDevice)
        put("exact", SmsSync.exactAllowed(a))
        put("exact_settings", Build.VERSION.SDK_INT >= 31)
        put("restricted_settings", Build.VERSION.SDK_INT >= 33) // sideloaded apps need "Allow restricted settings"
        put("last_run", prefs.smsLastRun)
        put("last_result", prefs.smsLastResult)
        put("next_run", if (prefs.smsEnabled) prefs.smsNextRun else 0)
        put("checkpoint", prefs.smsCheckpoint)
    }

    /** Opens the app for the current mode (refused while locked). */
    @JavascriptInterface
    fun enter() = a.ui { a.enter() }

    /** Shows the read-only demo. */
    @JavascriptInterface
    fun useDemo() = a.ui { a.useDemo() }

    /** Switches back to the chosen folder. */
    @JavascriptInterface
    fun useFolder() = a.ui { a.useFolder() }

    /** Asks for file access if needed, then opens the folder picker (result: "folder" event). */
    @JavascriptInterface
    fun chooseFolder() = a.ui { a.chooseFolder() }

    /** Uses a typed folder path; returns {ok, error}. */
    @JavascriptInterface
    fun setFolderPath(path: String): String {
        val err = Storage.check(a, path)
        if (err == null) a.ui { a.folderChosen(Paths.clean(path)!!) }
        return JSONObject().put("ok", err == null).put("error", err ?: "").toString()
    }

    // ---- settings file ----

    /** Opens the folder picker for the settings file (result: "config_dir" event). */
    @JavascriptInterface
    fun chooseConfigDir() = a.ui { a.chooseConfigDir() }

    /** Uses a typed folder for the settings file; returns {ok, path, exists, error}. */
    @JavascriptInterface
    fun setConfigDir(path: String): String = a.configDirChosen(Paths.clean(path) ?: path).toString()

    /** Stops keeping a settings file (the file itself stays). */
    @JavascriptInterface
    fun clearConfigDir() {
        prefs.configDir = ""
        app.configError = ""
    }

    /** Writes the settings file now. */
    @JavascriptInterface
    fun saveConfig() = app.saveConfig()

    /** Loads the settings file and applies it (the engine restarts); returns "" or an error. */
    @JavascriptInterface
    fun loadConfig(): String = a.restoreConfig()

    /** Opens the system screen to allow file access (result: "access" event). */
    @JavascriptInterface
    fun requestFilesAccess() = a.ui { a.requestAccess(pickAfter = false) }

    /** Main device on/off (restarts the engine). */
    @JavascriptInterface
    fun setPrimary(on: Boolean) {
        prefs.primary = on
        a.ui { a.restartEngine() }
    }

    /** Stores a new PIN (hashed); true on success. */
    @JavascriptInterface
    fun setPin(pin: String): Boolean {
        if (!Pin.valid(pin)) return false
        prefs.pinHash = Pin.hash(pin)
        prefs.pinLength = pin.length
        prefs.lockMode = "pin"
        app.unlocked = true
        return true
    }

    /** Removes the app lock. */
    @JavascriptInterface
    fun clearLock() {
        prefs.pinHash = ""
        prefs.biometric = false
        prefs.lockMode = "none"
    }

    /** Lock with fingerprint/face, the phone's screen lock as backup; false without a screen lock. */
    @JavascriptInterface
    fun setDeviceLock(): Boolean {
        if (!Lock.deviceSecure(a)) return false
        prefs.lockMode = "device"
        prefs.pinHash = ""
        prefs.biometric = false
        app.unlocked = true
        return true
    }

    /** Shows the device-lock prompt (failure: "unlock" event). */
    @JavascriptInterface
    fun unlockDevice() = a.ui { a.deviceUnlock() }

    // ---- nightly SMS capture ----

    /** SMS capture settings and last result as JSON. */
    @JavascriptInterface
    fun sms(): String = smsJson().toString()

    /** Asks for SMS permission (result: "sms_permission" event {granted, blocked}). */
    @JavascriptInterface
    fun requestSms() = a.ui { a.requestSms() }

    /** Switches the nightly capture on/off; false when SMS permission is missing. */
    @JavascriptInterface
    fun setSms(on: Boolean): Boolean {
        if (on && !SmsSync.permitted(a)) return false
        if (on && !prefs.smsEnabled) { // from today on: never back-fill older messages
            val start = LocalDate.now().atStartOfDay(ZoneId.systemDefault()).toInstant().toEpochMilli()
            prefs.smsEnabledAt = start
            if (prefs.smsCheckpoint < start) prefs.smsCheckpoint = start
        }
        prefs.smsEnabled = on
        SmsSync.schedule(a)
        return true
    }

    /** Time of the nightly run. */
    @JavascriptInterface
    fun setSmsTime(hour: Int, minute: Int) {
        prefs.smsHour = hour.coerceIn(0, 23)
        prefs.smsMinute = minute.coerceIn(0, 59)
        SmsSync.schedule(a)
    }

    /** After a missed night, read at most this many days back (1–7). */
    @JavascriptInterface
    fun setSmsLookback(days: Int) {
        prefs.smsLookback = days.coerceIn(1, 7)
    }

    /** Copies new messages now (result: "sms" event). */
    @JavascriptInterface
    fun smsRunNow() {
        app.bg.execute {
            val r = SmsSync.run(a)
            a.ui { a.smsDone(r) }
        }
    }

    // ---- morning inbox notification ----

    /** Switches the daily notice on/off; false when notifications must be allowed first ("notify_permission" event follows). */
    @JavascriptInterface
    fun setNotify(on: Boolean): Boolean {
        if (on && !InboxNotify.permitted(a)) {
            a.ui { a.requestNotify() }
            return false
        }
        prefs.notifyEnabled = on
        InboxNotify.schedule(a)
        return true
    }

    /** Time of the daily notice. */
    @JavascriptInterface
    fun setNotifyTime(hour: Int, minute: Int) {
        prefs.notifyHour = hour.coerceIn(0, 23)
        prefs.notifyMinute = minute.coerceIn(0, 59)
        InboxNotify.schedule(a)
    }

    /** Shows the notice now, even with an empty inbox (result: "notify" event). */
    @JavascriptInterface
    fun notifyNow() {
        app.bg.execute {
            val r = InboxNotify.run(a, test = true)
            a.ui { a.notifyDone(r) }
        }
    }

    // ---- look ----

    /** Uses the phone's wallpaper colours; false before Android 12. */
    @JavascriptInterface
    fun setMaterialYou(on: Boolean): Boolean {
        if (on && Build.VERSION.SDK_INT < 31) return false
        prefs.materialYou = on
        return true
    }

    /** Accent colours as JSON ({available, on, light, dark}). */
    @JavascriptInterface
    fun accent(): String = accentJson().toString()

    /** Saves the widget look and redraws the widgets; false when a value is invalid. */
    @JavascriptInterface
    fun setWidgetLook(bg: String, opacity: Int, text: String): Boolean {
        if (bg != "auto" && WidgetLook.rgb(bg) == null) return false
        if (text !in listOf("auto", "light", "dark")) return false
        prefs.widgetBg = bg.lowercase()
        prefs.widgetOpacity = opacity.coerceIn(0, 100)
        prefs.widgetText = text
        Widget.draw(a.applicationContext, Summary.cached(a))
        return true
    }

    /** Redraws the home-screen widget with fresh numbers. */
    @JavascriptInterface
    fun refreshWidget() = Widget.refreshAsync(a.applicationContext, null)

    /** Opens this app's system settings (for "Allow restricted settings" and permissions). */
    @JavascriptInterface
    fun openAppSettings() = a.ui { a.openAppSettings() }

    /** Opens the system screen that allows exact alarms (Android 12+). */
    @JavascriptInterface
    fun openExactAlarms() = a.ui { a.openExactAlarms() }

    /** Fingerprint unlock on/off; false if the phone can't do it. */
    @JavascriptInterface
    fun setBiometric(on: Boolean): Boolean {
        if (on && (prefs.lockMode != "pin" || !Lock.biometricAvailable(a))) return false
        prefs.biometric = on
        return true
    }

    /** Seconds in the background before locking again. */
    @JavascriptInterface
    fun setLockAfter(seconds: Int) {
        prefs.lockAfter = seconds.coerceIn(0, 4 * 3600)
    }

    /** Checks the PIN; on success opens the app. Five wrong tries pause input for 30 s. */
    @JavascriptInterface
    fun unlock(pin: String): Boolean {
        if (SystemClock.elapsedRealtime() < blockedUntil) return false
        if (!Pin.verify(pin, prefs.pinHash)) {
            if (++failedPins >= 5) {
                failedPins = 0
                blockedUntil = SystemClock.elapsedRealtime() + 30_000
            }
            return false
        }
        failedPins = 0
        app.unlocked = true
        a.ui { a.enter() }
        return true
    }

    /** Shows the fingerprint prompt (failure: "unlock" event). */
    @JavascriptInterface
    fun unlockBiometric() = a.ui { a.biometricUnlock() }

    /** Locks the app now. */
    @JavascriptInterface
    fun lockNow() = a.ui { a.lockNow() }

    /** Last lines of the engine log. */
    @JavascriptInterface
    fun log(): String = app.engine.log.tail(300)

    /** Restarts the engine and reloads. */
    @JavascriptInterface
    fun restartEngine() = a.ui { a.restartEngine() }

    /** Vibration feedback: confirm | error | tick. */
    @JavascriptInterface
    fun haptic(kind: String) = a.ui { a.haptic(kind) }

    /** Colours the status/navigation bars (#rrggbb, dark page?). */
    @JavascriptInterface
    fun setBars(color: String, dark: Boolean) = a.ui { a.setBars(color, dark) }
}
