package com.nihars.arthik

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import java.time.ZoneId

/** A once-a-day wake-up at a clock time, shared by the SMS copy and the inbox notice. */
object DailyAlarm {
    /** True when Android lets the app wake at an exact minute (else it runs within ~15 min). */
    fun exactAllowed(ctx: Context): Boolean {
        val am = ctx.getSystemService(AlarmManager::class.java) ?: return false
        return Build.VERSION.SDK_INT < 31 || am.canScheduleExactAlarms()
    }

    /** Sets the next run of [receiver] at hour:minute and returns its time (epoch ms). */
    fun set(ctx: Context, receiver: Class<out BroadcastReceiver>, code: Int, hour: Int, minute: Int): Long {
        val am = ctx.getSystemService(AlarmManager::class.java) ?: return 0
        val pi = pending(ctx, receiver, code)
        am.cancel(pi)
        val at = SmsRules.nextRun(System.currentTimeMillis(), hour, minute, ZoneId.systemDefault())
        if (exactAllowed(ctx)) am.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pi)
        else am.setWindow(AlarmManager.RTC_WAKEUP, at, 15 * 60_000L, pi)
        return at
    }

    /** Removes the wake-up of [receiver]. */
    fun cancel(ctx: Context, receiver: Class<out BroadcastReceiver>, code: Int) {
        ctx.getSystemService(AlarmManager::class.java)?.cancel(pending(ctx, receiver, code))
    }

    /** Opens the system screen for exact alarms (Android 12+). */
    fun exactAlarmIntent(ctx: Context): Intent? =
        if (Build.VERSION.SDK_INT >= 31) Intent(android.provider.Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM, Uri.parse("package:" + ctx.packageName)) else null

    private fun pending(ctx: Context, receiver: Class<out BroadcastReceiver>, code: Int): PendingIntent =
        PendingIntent.getBroadcast(ctx, code, Intent(ctx, receiver), PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
}

/** Alarms do not survive a reboot, an app update or a clock change: set them again. */
class Reschedule : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        val app = ctx.applicationContext
        SmsSync.schedule(app)
        InboxNotify.schedule(app)
        if (intent.action == Intent.ACTION_MY_PACKAGE_REPLACED) Widget.refreshAsync(app, goAsync())
    }
}
