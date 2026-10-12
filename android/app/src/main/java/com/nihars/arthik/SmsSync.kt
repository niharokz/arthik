package com.nihars.arthik

import android.Manifest
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.provider.Telephony
import java.time.Instant
import java.time.ZoneId

/**
 * Nightly copy of new bank SMS into the finance folder (inbox/sms_<device>.md).
 * Reads only messages after the last successful run (at most a few days back, never
 * before the capture was switched on), so nothing is read twice or back-filled.
 */
object SmsSync {
    /** True when the app may read SMS. */
    fun permitted(ctx: Context) = ctx.checkSelfPermission(Manifest.permission.READ_SMS) == PackageManager.PERMISSION_GRANTED

    /** Reads and saves new messages now; returns a one-line result (also stored in prefs). */
    @Synchronized
    fun run(ctx: Context): String {
        val p = Prefs(ctx)
        val now = System.currentTimeMillis()
        val result = try {
            when {
                !p.smsEnabled -> "Off"
                !permitted(ctx) -> "No SMS permission"
                p.mode != "folder" || !Storage.looksUsable(p.folder) -> "No finance folder"
                else -> {
                    val zone = ZoneId.systemDefault()
                    val from = SmsRules.windowStart(now, p.smsCheckpoint, p.smsEnabledAt, p.smsLookback, zone)
                    val scan = read(ctx, from, now, zone)
                    val added = SmsStore(p.folder, p.smsDevice).add(scan.wanted, Instant.ofEpochMilli(now).atZone(zone).toLocalDate())
                    p.smsCheckpoint = now // only after the file was written
                    val since = SmsRules.iso(from, zone).take(16).replace('T', ' ')
                    "Saved $added new · since $since: ${scan.total} SMS, ${scan.wanted.size} from banks" + if (scan.total == 0) hint(ctx, now, zone) else ""
                }
            }
        } catch (e: Exception) {
            "Failed: ${e.message}" // checkpoint unchanged: the next run reads this window again
        }
        p.smsLastRun = now
        p.smsLastResult = result
        return result
    }

    /** What one read found: every SMS in the window, and the bank ones worth copying. */
    private class Scan(var total: Int = 0, val wanted: MutableList<SmsStore.Message> = mutableListOf())

    // Nothing in the window: say whether older bank SMS are visible at all (not copied),
    // which tells "no new messages yet" apart from "the phone hides them from apps".
    private fun hint(ctx: Context, now: Long, zone: ZoneId): String {
        val week = read(ctx, now - 7 * 86_400_000L, now, zone)
        return when {
            week.wanted.isNotEmpty() -> " · ${week.wanted.size} older bank SMS from the last 7 days are visible (before the start, not copied)"
            week.total > 0 -> " · ${week.total} SMS from the last 7 days are visible, none from banks"
            else -> " · the app can't see any SMS: check the SMS permission (some phones, e.g. Xiaomi, also have a separate permission for service/notification SMS)"
        }
    }

    private fun read(ctx: Context, from: Long, to: Long, zone: ZoneId): Scan {
        val out = Scan()
        val cols = arrayOf(Telephony.Sms.ADDRESS, Telephony.Sms.BODY, Telephony.Sms.DATE)
        ctx.contentResolver.query(
            Telephony.Sms.Inbox.CONTENT_URI, cols, "${Telephony.Sms.DATE} > ? AND ${Telephony.Sms.DATE} <= ?",
            arrayOf(from.toString(), to.toString()), "${Telephony.Sms.DATE} ASC",
        )?.use { c ->
            while (c.moveToNext()) {
                out.total++
                val address = c.getString(0) ?: continue
                val body = c.getString(1) ?: continue
                if (!SmsRules.wanted(address, body)) continue
                val at = SmsRules.iso(c.getLong(2), zone)
                out.wanted += SmsStore.Message(SmsRules.id(address, at, body), address, at, body)
            }
        }
        return out
    }

    // ---- schedule ----------------------------------------------------------------

    private const val ALARM = 0

    /** True when Android lets the app wake at an exact minute (else it runs within ~15 min). */
    fun exactAllowed(ctx: Context) = DailyAlarm.exactAllowed(ctx)

    /** Sets (or clears) the next nightly run from the settings. */
    fun schedule(ctx: Context) {
        val p = Prefs(ctx)
        if (!p.smsEnabled) return DailyAlarm.cancel(ctx, SmsAlarm::class.java, ALARM)
        p.smsNextRun = DailyAlarm.set(ctx, SmsAlarm::class.java, ALARM, p.smsHour, p.smsMinute)
    }

    /** Opens the system screen for exact alarms (Android 12+). */
    fun exactAlarmIntent(ctx: Context): Intent? = DailyAlarm.exactAlarmIntent(ctx)
}

/** Fires at the chosen time: copy new SMS, then set tomorrow's run. */
class SmsAlarm : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        val done = goAsync()
        Thread {
            try {
                SmsSync.run(ctx.applicationContext)
            } finally {
                SmsSync.schedule(ctx.applicationContext)
                done.finish()
            }
        }.start()
    }
}
