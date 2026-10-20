package com.nihars.arthik

import java.io.File

/**
 * The app's settings as a small YAML file (arthik-phone.yaml) in a folder chosen in
 * Settings → Phone, kept up to date on every change, so a reinstall or a new phone
 * gets them back. Secrets (the PIN hash) and run state stay on the phone only.
 * Pure code (java.io only), unit tested.
 */
object AppConfig {
    const val FILE = "arthik-phone.yaml"

    enum class Kind { STR, BOOL, INT, LONG }

    /** Settings kept in the file (SharedPreferences key → type), in file order. */
    val KEYS: Map<String, Kind> = linkedMapOf(
        "mode" to Kind.STR, "folder" to Kind.STR, "primary" to Kind.BOOL,
        "lock_mode" to Kind.STR, "biometric" to Kind.BOOL, "lock_after" to Kind.INT,
        "sms_enabled" to Kind.BOOL, "sms_enabled_at" to Kind.LONG, "sms_hour" to Kind.INT, "sms_minute" to Kind.INT,
        "sms_lookback" to Kind.INT, "sms_device" to Kind.STR,
        "notify_enabled" to Kind.BOOL, "notify_hour" to Kind.INT, "notify_minute" to Kind.INT,
        "material_you" to Kind.BOOL,
        "widget_bg" to Kind.STR, "widget_opacity" to Kind.INT, "widget_text" to Kind.STR,
    )

    /** File text for the given values (keys outside KEYS are left out). */
    fun write(values: Map<String, Any?>, savedAt: String): String = buildString {
        append("# Arthik phone settings — written by the Arthik app (Settings → Phone).\n")
        append("# Copy it to a new phone and choose this folder there to restore them.\n")
        append("saved: ").append(quote(savedAt)).append('\n')
        for ((k, kind) in KEYS) {
            val v = values[k] ?: continue
            append(k).append(": ").append(if (kind == Kind.STR) quote(v.toString()) else v.toString()).append('\n')
        }
    }

    /** Typed values from file text; unknown keys are ignored, a malformed line is an error. */
    fun read(text: String): Map<String, Any> {
        val out = LinkedHashMap<String, Any>()
        text.lines().forEachIndexed { i, raw ->
            val line = raw.trim()
            if (line.isEmpty() || line.startsWith("#")) return@forEachIndexed
            val colon = line.indexOf(':')
            require(colon > 0) { "line ${i + 1}: expected key: value" }
            val key = line.substring(0, colon).trim()
            val kind = KEYS[key] ?: return@forEachIndexed
            val v = line.substring(colon + 1).trim()
            out[key] = when (kind) {
                Kind.STR -> unquote(v)
                Kind.BOOL -> v.toBooleanStrictOrNull() ?: throw IllegalArgumentException("line ${i + 1}: $key must be true or false")
                Kind.INT -> v.toIntOrNull() ?: throw IllegalArgumentException("line ${i + 1}: $key must be a number")
                Kind.LONG -> v.toLongOrNull() ?: throw IllegalArgumentException("line ${i + 1}: $key must be a number")
            }
        }
        return out
    }

    /** Writes the file atomically in dir. */
    fun save(dir: String, values: Map<String, Any?>, savedAt: String) {
        val d = File(dir)
        if (!d.isDirectory && !d.mkdirs()) throw java.io.IOException("can't create $dir")
        val tmp = File(d, ".$FILE.tmp")
        tmp.writeText(write(values, savedAt))
        if (!tmp.renameTo(File(d, FILE))) {
            tmp.delete()
            throw java.io.IOException("can't write ${File(d, FILE)}")
        }
    }

    /** The values in dir's file, or null when there is no file. */
    fun load(dir: String): Map<String, Any>? {
        val f = File(dir, FILE)
        return if (f.isFile) read(f.readText()) else null
    }

    private fun quote(s: String) = "\"" + s.replace("\\", "\\\\").replace("\"", "\\\"") + "\""

    private fun unquote(v: String): String {
        if (v.length < 2 || !v.startsWith("\"") || !v.endsWith("\"")) return v
        val sb = StringBuilder()
        var esc = false
        for (c in v.substring(1, v.length - 1)) {
            if (esc) { sb.append(c); esc = false } else if (c == '\\') esc = true else sb.append(c)
        }
        return sb.toString()
    }
}
