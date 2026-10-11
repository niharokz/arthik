package com.nihars.arthik

import android.content.Context

/** App settings stored on the phone only (never in the synced folder). */
class Prefs(ctx: Context) {
    private val sp = ctx.getSharedPreferences("arthik", Context.MODE_PRIVATE)

    /** "" (first run), "demo" or "folder". */
    var mode: String
        get() = sp.getString("mode", "") ?: ""
        set(v) = sp.edit().putString("mode", v).apply()

    /** Absolute path of the finance folder ("" = none chosen). */
    var folder: String
        get() = sp.getString("folder", "") ?: ""
        set(v) = sp.edit().putString("folder", v).apply()

    /** This phone writes computed files and imports the inbox (off when nimory runs Arthik). */
    var primary: Boolean
        get() = sp.getBoolean("primary", false)
        set(v) = sp.edit().putBoolean("primary", v).apply()

    /** Hashed PIN ("" = no lock). */
    var pinHash: String
        get() = sp.getString("pin_hash", "") ?: ""
        set(v) = sp.edit().putString("pin_hash", v).apply()

    /** Number of digits in the PIN (drawn as dots). */
    var pinLength: Int
        get() = sp.getInt("pin_length", 4)
        set(v) = sp.edit().putInt("pin_length", v).apply()

    /** Fingerprint unlock on top of the PIN. */
    var biometric: Boolean
        get() = sp.getBoolean("biometric", false)
        set(v) = sp.edit().putBoolean("biometric", v).apply()

    /** "pin" (Arthik PIN), "device" (fingerprint/face with the phone's screen lock as backup) or "none". */
    var lockMode: String
        get() = sp.getString("lock_mode", null) ?: if (pinHash.isNotEmpty()) "pin" else "none"
        set(v) = sp.edit().putString("lock_mode", v).apply()

    /** Seconds in the background before the app locks again (default 5 minutes). */
    var lockAfter: Int
        get() = sp.getInt("lock_after", 300)
        set(v) = sp.edit().putInt("lock_after", v).apply()

    /** Wall-clock time the unlocked app went to the background (0 = not unlocked when left). */
    var leftAt: Long
        get() = sp.getLong("left_at", 0)
        set(v) { sp.edit().putLong("left_at", v).commit() } // before Android may stop the process

    /** True when the app locks (PIN or device lock). */
    val lockOn: Boolean get() = lockMode == "device" || (lockMode == "pin" && pinHash.isNotEmpty())

    // ---- nightly SMS capture ----

    /** Copy new bank SMS into the finance folder every night. */
    var smsEnabled: Boolean
        get() = sp.getBoolean("sms_enabled", false)
        set(v) = sp.edit().putBoolean("sms_enabled", v).apply()

    /** Start of the day the capture was switched on (nothing older is read). */
    var smsEnabledAt: Long
        get() = sp.getLong("sms_enabled_at", 0)
        set(v) = sp.edit().putLong("sms_enabled_at", v).apply()

    /** Messages up to this time (epoch ms) are already saved. */
    var smsCheckpoint: Long
        get() = sp.getLong("sms_checkpoint", 0)
        set(v) { sp.edit().putLong("sms_checkpoint", v).commit() } // written before the alarm's process may end

    /** After a missed night, read at most this many days back. */
    var smsLookback: Int
        get() = sp.getInt("sms_lookback", 3)
        set(v) = sp.edit().putInt("sms_lookback", v).apply()

    /** Time of the nightly run. */
    var smsHour: Int
        get() = sp.getInt("sms_hour", 23)
        set(v) = sp.edit().putInt("sms_hour", v).apply()
    var smsMinute: Int
        get() = sp.getInt("sms_minute", 30)
        set(v) = sp.edit().putInt("sms_minute", v).apply()

    /** Name in the file name (inbox/sms_<device>.md), so two phones never share a file. */
    var smsDevice: String
        get() = sp.getString("sms_device", "phone") ?: "phone"
        set(v) = sp.edit().putString("sms_device", v).apply()

    var smsLastRun: Long
        get() = sp.getLong("sms_last_run", 0)
        set(v) = sp.edit().putLong("sms_last_run", v).apply()
    var smsLastResult: String
        get() = sp.getString("sms_last_result", "") ?: ""
        set(v) = sp.edit().putString("sms_last_result", v).apply()
    var smsNextRun: Long
        get() = sp.getLong("sms_next_run", 0)
        set(v) = sp.edit().putLong("sms_next_run", v).apply()
}
