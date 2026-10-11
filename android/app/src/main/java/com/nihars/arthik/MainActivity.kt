package com.nihars.arthik

import android.Manifest
import android.annotation.SuppressLint
import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.Intent
import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import android.content.res.Configuration
import android.graphics.Color
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.SystemClock
import android.provider.Settings
import android.view.Gravity
import android.view.HapticFeedbackConstants
import android.view.View
import android.view.ViewGroup
import android.view.WindowInsetsController
import android.webkit.CookieManager
import android.webkit.RenderProcessGoneDetail
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView
import android.widget.Toast
import org.json.JSONObject

/**
 * The only screen: a full-screen WebView showing the same UI as arthik.nihars.com,
 * served by the engine running on this phone. This class starts the engine, decides
 * which page to open (welcome, lock or the app) and handles everything Android-side:
 * folder picker, file access, fingerprint, back button, downloads and status bar.
 */
class MainActivity : Activity() {
    val arthik: ArthikApp get() = application as ArthikApp
    private val prefs get() = arthik.prefs
    private val engine get() = arthik.engine

    private lateinit var web: WebView
    private lateinit var splash: View
    private var fileCallback: ValueCallback<Array<Uri>>? = null
    private var pickAfterAccess = false
    private var awayForResult = false // an external screen (picker, settings) is open
    private var stoppedAt = 0L
    private var clearHistoryNext = false

    companion object {
        private const val REQ_FOLDER = 1
        private const val REQ_ACCESS = 2
        private const val REQ_FILE = 3
        private const val REQ_SMS = 4
        private const val REQ_CONFIRM = 5
    }

    // ---- lifecycle -------------------------------------------------------------

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE != 0) WebView.setWebContentsDebuggingEnabled(true)
        web = buildWebView()
        splash = buildSplash()
        setContentView(FrameLayout(this).apply {
            addView(web, ViewGroup.LayoutParams(-1, -1))
            addView(splash, ViewGroup.LayoutParams(-1, -1))
        })
        // A new screen (app reopened, or the process was restarted by Android or the
        // nightly SMS job): stay unlocked only within the lock delay of leaving.
        arthik.unlocked = arthik.stillUnlocked() || (arthik.unlocked && prefs.leftAt == 0L)
        engine.onCrash = { tail -> ui { showError("The engine stopped unexpectedly.", tail) } }
        SmsSync.schedule(this) // keeps the nightly SMS run set after updates
        if (engine.running) openStart() else restartEngine()
    }

    override fun onStop() {
        super.onStop()
        if (!awayForResult) {
            stoppedAt = SystemClock.elapsedRealtime()
            prefs.leftAt = if (arthik.unlocked) System.currentTimeMillis() else 0
        }
    }

    override fun onStart() {
        super.onStart()
        // Lock again after being in the background long enough.
        if (stoppedAt > 0 && prefs.lockOn && arthik.unlocked &&
            SystemClock.elapsedRealtime() - stoppedAt >= prefs.lockAfter * 1000L) lockNow()
        stoppedAt = 0
        prefs.leftAt = 0
    }

    override fun onDestroy() {
        if (isFinishing) arthik.bg.execute { engine.stop() } // leaving the app: stop the engine
        web.destroy()
        super.onDestroy()
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        // Let the page close a sheet or go back a screen first.
        web.evaluateJavascript("window.arthikBack ? String(window.arthikBack()) : 'false'") { handled ->
            if (handled?.trim('"') != "true") moveTaskToBack(true)
        }
    }

    // ---- views -----------------------------------------------------------------

    @SuppressLint("SetJavaScriptEnabled", "JavascriptInterface")
    private fun buildWebView(): WebView = WebView(this).apply {
        alpha = 0f
        setBackgroundColor(bgColor())
        settings.javaScriptEnabled = true
        settings.domStorageEnabled = true
        settings.allowFileAccess = false
        settings.allowContentAccess = false
        settings.setSupportZoom(false)
        overScrollMode = View.OVER_SCROLL_NEVER
        addJavascriptInterface(Bridge(this@MainActivity), "ArthikNative")
        webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, req: WebResourceRequest): Boolean {
                val u = req.url
                if (u.host == "127.0.0.1" && u.port == engine.port) return false
                openExternal(u) // only the engine's pages open inside the app
                return true
            }

            override fun onPageFinished(view: WebView, url: String) {
                if (clearHistoryNext && !url.contains("/local/enter")) { view.clearHistory(); clearHistoryNext = false }
                if (url.startsWith("http")) revealWeb()
            }

            override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
                recreate() // the WebView crashed: rebuild the screen, the engine keeps running
                return true
            }
        }
        webChromeClient = object : WebChromeClient() {
            override fun onShowFileChooser(view: WebView, cb: ValueCallback<Array<Uri>>, params: FileChooserParams): Boolean {
                fileCallback?.onReceiveValue(null)
                fileCallback = cb
                return try {
                    away(); startActivityForResult(params.createIntent(), REQ_FILE); true
                } catch (e: ActivityNotFoundException) {
                    fileCallback = null; false
                }
            }
        }
        setDownloadListener { url, _, disposition, mime, _ -> download(url, disposition, mime) }
    }

    // Logo + spinner shown while the engine starts.
    private fun buildSplash(): View = LinearLayout(this).apply {
        orientation = LinearLayout.VERTICAL
        gravity = Gravity.CENTER
        setBackgroundColor(bgColor())
        addView(ImageView(context).apply { setImageDrawable(packageManager.getApplicationIcon(packageName)) }, LinearLayout.LayoutParams(dp(88), dp(88)))
        addView(ProgressBar(context).apply { isIndeterminate = true }, LinearLayout.LayoutParams(dp(36), dp(36)).apply { topMargin = dp(24) })
        addView(TextView(context).apply { text = "Starting Arthik…"; setTextColor(Color.GRAY); gravity = Gravity.CENTER }, LinearLayout.LayoutParams(-2, -2).apply { topMargin = dp(12) })
    }

    // Fades the page in and the splash out (once).
    private fun revealWeb() {
        if (splash.visibility != View.VISIBLE) return
        web.animate().alpha(1f).setDuration(220).start()
        splash.animate().alpha(0f).setDuration(220).withEndAction { splash.visibility = View.GONE }.start()
    }

    private fun showSplash() {
        splash.animate().cancel()
        splash.alpha = 1f
        splash.visibility = View.VISIBLE
        web.alpha = 0f
    }

    // ---- engine and navigation ---------------------------------------------------

    /** Runs code on the main thread. */
    fun ui(block: () -> Unit) = runOnUiThread(block)

    /** (Re)starts the engine for the current settings, then opens the right page. */
    fun restartEngine() {
        showSplash()
        val folder = prefs.folder.takeIf { it.isNotEmpty() }
        arthik.bg.execute {
            var err = ""
            var useFolder = folder
            if (useFolder != null) err = Storage.check(this, useFolder) ?: ""
            if (err.isNotEmpty()) useFolder = null
            var ok = engine.start(useFolder, prefs.primary)
            if (!ok && useFolder != null) { // folder broke the engine: keep the app usable with the demo
                err = "The engine couldn't open $useFolder.\n" + engine.log.tail(4)
                ok = engine.start(null, prefs.primary)
            }
            arthik.engineError = err
            ui { if (ok) openStart() else showError("The engine didn't start on this phone.", engine.log.tail(30)) }
        }
    }

    /** First page after the engine is up: lock / welcome / error screen, or straight in. */
    private fun openStart() {
        CookieManager.getInstance().removeAllCookies(null)
        val needsGate = prefs.mode.isEmpty() || (prefs.lockOn && !arthik.unlocked) ||
            (prefs.mode == "folder" && engine.folder.isEmpty())
        if (needsGate) web.loadUrl(engine.url("/app")) else enter()
    }

    /** Opens a session for the current mode. */
    fun enter() {
        if (prefs.lockOn && !arthik.unlocked) return
        val user = if (prefs.mode == "folder" && engine.folder.isNotEmpty()) "me" else "demouser"
        clearHistoryNext = true
        web.loadUrl(engine.enterUrl(user))
    }

    fun useDemo() {
        prefs.mode = "demo"
        if (engine.running) enter() else restartEngine()
    }

    fun useFolder() {
        if (prefs.folder.isEmpty()) return chooseFolder()
        prefs.mode = "folder"
        if (engine.running && engine.folder == prefs.folder) enter() else restartEngine()
    }

    /** A folder was picked or typed and checked: use it. */
    fun folderChosen(path: String) {
        prefs.folder = path
        prefs.mode = "folder"
        event("folder", JSONObject().put("ok", true).put("path", path))
        restartEngine()
    }

    fun lockNow() {
        if (!prefs.lockOn) return
        arthik.unlocked = false
        CookieManager.getInstance().removeAllCookies(null)
        web.loadUrl(engine.url("/app"))
    }

    fun biometricUnlock() = Lock.prompt(this) { ok -> unlocked(ok) }

    /** Device lock: biometric prompt, or the screen-lock check on Android 8–9. */
    fun deviceUnlock() {
        if (Lock.promptDevice(this) { ok -> unlocked(ok) }) return
        val i = Lock.confirmIntent(this) ?: return unlocked(true) // screen lock removed since: nothing to check
        away()
        startActivityForResult(i, REQ_CONFIRM)
    }

    private fun unlocked(ok: Boolean) {
        if (ok) { arthik.unlocked = true; enter() } else event("unlock", JSONObject().put("ok", false))
    }

    fun requestSms() {
        if (SmsSync.permitted(this)) return event("sms_permission", JSONObject().put("granted", true))
        away()
        requestPermissions(arrayOf(Manifest.permission.READ_SMS), REQ_SMS)
    }

    fun smsDone(result: String) = event("sms", JSONObject().put("result", result))

    fun openAppSettings() {
        away()
        openExternal(Uri.parse("package:$packageName"), Settings.ACTION_APPLICATION_DETAILS_SETTINGS)
    }

    fun openExactAlarms() {
        val i = SmsSync.exactAlarmIntent(this) ?: return
        away()
        try { startActivity(i) } catch (_: ActivityNotFoundException) { openAppSettings() }
    }

    private fun showError(message: String, log: String) {
        web.loadDataWithBaseURL(null, Pages.error(message, log), "text/html", "utf-8", null)
        web.alpha = 1f
        splash.visibility = View.GONE
    }

    // ---- folder access -------------------------------------------------------------

    fun chooseFolder() {
        if (!Storage.hasAccess(this)) return requestAccess(pickAfter = true)
        away()
        try {
            startActivityForResult(Storage.pickerIntent(), REQ_FOLDER)
        } catch (e: ActivityNotFoundException) {
            event("folder", JSONObject().put("ok", false).put("error", "This phone has no folder picker. Type the path instead."))
        }
    }

    fun requestAccess(pickAfter: Boolean) {
        pickAfterAccess = pickAfter
        away()
        Storage.requestAccess(this, REQ_ACCESS)
    }

    // An external screen is opening: don't treat the trip as "app left".
    private fun away() { awayForResult = true }

    private fun accessResult() {
        val granted = Storage.hasAccess(this)
        event("access", JSONObject().put("granted", granted))
        if (pickAfterAccess) {
            pickAfterAccess = false
            if (granted) chooseFolder()
            else event("folder", JSONObject().put("ok", false).put("error", "Arthik needs file access to read and write your finance folder."))
        }
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        awayForResult = false
        when (requestCode) {
            REQ_ACCESS -> accessResult()
            REQ_CONFIRM -> unlocked(resultCode == RESULT_OK)
            REQ_FILE -> {
                fileCallback?.onReceiveValue(WebChromeClient.FileChooserParams.parseResult(resultCode, data))
                fileCallback = null
            }
            REQ_FOLDER -> {
                val uri = data?.data
                if (resultCode != RESULT_OK || uri == null) return // cancelled
                val path = Storage.pathOf(uri)
                arthik.bg.execute {
                    val err = Storage.check(this, path)
                    ui {
                        if (err == null) folderChosen(path!!)
                        else event("folder", JSONObject().put("ok", false).put("error", err))
                    }
                }
            }
        }
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        awayForResult = false
        if (requestCode == REQ_ACCESS) accessResult()
        if (requestCode == REQ_SMS) {
            val granted = grantResults.firstOrNull() == PackageManager.PERMISSION_GRANTED
            // Denied without a dialog: Android 13+ blocks SMS for sideloaded apps until
            // "Allow restricted settings" is turned on in the app's info screen.
            val blocked = !granted && !shouldShowRequestPermissionRationale(Manifest.permission.READ_SMS)
            event("sms_permission", JSONObject().put("granted", granted).put("blocked", blocked))
        }
    }

    // ---- small helpers ---------------------------------------------------------------

    /** Sends an event to the page (window.arthikNative.onEvent). */
    private fun event(type: String, data: JSONObject) {
        data.put("type", type)
        web.evaluateJavascript("window.arthikNative && window.arthikNative.onEvent($data)", null)
    }

    private fun download(url: String, disposition: String?, mime: String?) {
        arthik.bg.execute {
            val msg = try {
                "Saved to Downloads: " + Downloads.save(this, url, disposition, mime)
            } catch (e: Exception) {
                "Download failed: ${e.message}"
            }
            ui { Toast.makeText(this, msg, Toast.LENGTH_LONG).show() }
        }
    }

    private fun openExternal(uri: Uri, action: String = Intent.ACTION_VIEW) {
        try { startActivity(Intent(action, uri)) } catch (_: ActivityNotFoundException) {}
    }

    fun haptic(kind: String) {
        val c = when {
            Build.VERSION.SDK_INT >= 30 && kind == "confirm" -> HapticFeedbackConstants.CONFIRM
            Build.VERSION.SDK_INT >= 30 && kind == "error" -> HapticFeedbackConstants.REJECT
            else -> HapticFeedbackConstants.KEYBOARD_TAP
        }
        web.performHapticFeedback(c)
    }

    /** Matches the system bars to the page colour. */
    @Suppress("DEPRECATION")
    fun setBars(color: String, dark: Boolean) {
        val c = try { Color.parseColor(color) } catch (_: IllegalArgumentException) { return }
        window.statusBarColor = c
        window.navigationBarColor = c
        if (Build.VERSION.SDK_INT >= 30) {
            val flags = WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS or WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS
            window.insetsController?.setSystemBarsAppearance(if (dark) 0 else flags, flags)
        } else {
            var f = window.decorView.systemUiVisibility
            f = if (dark) f and View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR.inv() else f or View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR
            if (Build.VERSION.SDK_INT >= 27) {
                f = if (dark) f and View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR.inv() else f or View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR
            }
            window.decorView.systemUiVisibility = f
        }
    }

    /** "1.1.0 (10100)". */
    fun versionLabel(): String {
        val info = packageManager.getPackageInfo(packageName, 0)
        val code = if (Build.VERSION.SDK_INT >= 28) info.longVersionCode else @Suppress("DEPRECATION") info.versionCode.toLong()
        return "${info.versionName} ($code)"
    }

    private fun bgColor(): Int {
        val night = resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK == Configuration.UI_MODE_NIGHT_YES
        return if (night) Color.parseColor("#0f1413") else Color.parseColor("#f6f8f7")
    }

    private fun dp(v: Int): Int = (v * resources.displayMetrics.density).toInt()
}
