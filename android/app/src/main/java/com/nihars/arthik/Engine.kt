package com.nihars.arthik

import android.content.Context
import android.net.Uri
import android.util.Log
import java.io.File
import java.io.IOException
import java.net.HttpURLConnection
import java.net.InetAddress
import java.net.ServerSocket
import java.net.URL
import java.security.SecureRandom
import java.util.TimeZone

/**
 * Runs the arthik Go engine (lib/<abi>/libarthik.so, the same code as the server)
 * as a child process listening on 127.0.0.1. The WebView talks to it over HTTP.
 * The engine exits by itself when this app's process dies (its stdin closes).
 */
class Engine(private val ctx: Context) {
    /** Called on a background thread if the engine stops unexpectedly. */
    var onCrash: ((String) -> Unit)? = null

    val log = LogFile(File(ctx.filesDir, "engine.log"))
    var port = 0
        private set
    var token = ""
        private set
    /** Folder the running engine serves ("" = demo only). */
    var folder = ""
        private set

    @Volatile private var proc: Process? = null
    @Volatile private var stopping = false

    /** True while the engine process runs. */
    val running: Boolean get() = proc?.isAlive == true

    /** Starts the engine and waits until it answers; false if it failed. Call off the UI thread. */
    @Synchronized
    fun start(folder: String?, primary: Boolean): Boolean {
        stop()
        val bin = File(ctx.applicationInfo.nativeLibraryDir, "libarthik.so")
        if (!bin.canExecute()) {
            log.append("engine binary missing or not executable: $bin")
            return false
        }
        port = freePort()
        token = randomHex(20)
        this.folder = folder ?: ""
        val pb = ProcessBuilder(bin.absolutePath, "serve").directory(ctx.filesDir).redirectErrorStream(true)
        pb.environment().apply {
            keys.removeAll { it.startsWith("ARTHIK_") }
            put("ARTHIK_MODE", "local")
            put("ARTHIK_LISTEN", "127.0.0.1:$port")
            put("ARTHIK_LOCAL_TOKEN", token)
            put("ARTHIK_DEMO_DIR", File(ctx.filesDir, "demo").absolutePath)
            if (!folder.isNullOrEmpty()) put("ARTHIK_DATA", folder)
            put("ARTHIK_PRIMARY", primary.toString())
            put("ARTHIK_EXIT_ON_STDIN_EOF", "1")
            put("ARTHIK_WATCH_SECONDS", "2")
            put("TZ", TimeZone.getDefault().id) // Android has no /etc/localtime
            put("HOME", ctx.filesDir.absolutePath)
        }
        log.append("— starting engine (folder=${folder ?: "none"}, primary=$primary) —")
        val p = try {
            pb.start()
        } catch (e: IOException) {
            log.append("could not start engine: ${e.message}")
            return false
        }
        proc = p
        stopping = false
        Thread({ pump(p) }, "engine-log").start()
        Thread({ watch(p) }, "engine-watch").start()
        return waitReady(p)
    }

    /** Stops the engine gracefully (closes its stdin), then forcibly after 2 s. */
    @Synchronized
    fun stop() {
        val p = proc ?: return
        stopping = true
        try { p.outputStream.close() } catch (_: IOException) {}
        val deadline = System.currentTimeMillis() + 2000
        while (p.isAlive && System.currentTimeMillis() < deadline) Thread.sleep(50)
        if (p.isAlive) p.destroy()
        proc = null
    }

    /** URL of a page on the engine. */
    fun url(path: String) = "http://127.0.0.1:$port$path"

    /** URL that opens a session for a user ("me" or "demouser"), optionally on a screen ("inbox"). */
    fun enterUrl(user: String, to: String? = null) =
        url("/local/enter?token=$token&user=$user" + if (to.isNullOrEmpty()) "" else "&to=" + Uri.encode(to, "/"))

    // Copies engine output into the log file.
    private fun pump(p: Process) {
        try {
            p.inputStream.bufferedReader().forEachLine { log.append(it) }
        } catch (_: IOException) {}
    }

    // Reports an unexpected exit.
    private fun watch(p: Process) {
        val code = p.waitFor()
        log.append("— engine exited ($code) —")
        if (!stopping && proc === p) onCrash?.invoke(log.tail(20))
    }

    // Polls /healthz for up to 10 s.
    private fun waitReady(p: Process): Boolean {
        val deadline = System.currentTimeMillis() + 10_000
        while (System.currentTimeMillis() < deadline) {
            if (!p.isAlive) return false
            try {
                val c = URL(url("/healthz")).openConnection() as HttpURLConnection
                c.connectTimeout = 300
                c.readTimeout = 300
                val ok = c.responseCode == 200
                c.disconnect()
                if (ok) return true
            } catch (_: IOException) {}
            Thread.sleep(80)
        }
        return false
    }

    private fun freePort(): Int = ServerSocket(0, 1, InetAddress.getByName("127.0.0.1")).use { it.localPort }

    private fun randomHex(bytes: Int): String =
        ByteArray(bytes).also { SecureRandom().nextBytes(it) }.joinToString("") { "%02x".format(it) }
}

/** Small rotating log file (engine output plus app events), shown in Settings → Phone. */
class LogFile(private val file: File, private val maxBytes: Long = 256 * 1024) {
    /** Adds a line (keeps one older file when the log grows too large). */
    @Synchronized
    fun append(line: String) {
        Log.i("arthik", line)
        try {
            if (file.length() > maxBytes) file.renameTo(File(file.path + ".1"))
            file.appendText(line + "\n")
        } catch (_: IOException) {}
    }

    /** Last n lines. */
    @Synchronized
    fun tail(n: Int = 200): String = try {
        if (file.exists()) file.readLines().takeLast(n).joinToString("\n") else ""
    } catch (_: IOException) {
        ""
    }
}
