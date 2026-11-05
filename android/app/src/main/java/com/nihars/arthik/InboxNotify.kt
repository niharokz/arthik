package com.nihars.arthik

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
import android.os.Build

/**
 * The daily alert: once a day, at the time in Settings → Daily alert (settings.md, the
 * same for every phone), tells you when an account doesn't match the bank or when the
 * inbox and notifications hold more than the chosen count. Silent otherwise. Switched
 * on per phone in Settings → Phone.
 */
object InboxNotify {
    private const val ALARM = 1 // DailyAlarm request code (SMS uses 0)
    private const val TAP = 20 // PendingIntent request code
    private const val CHANNEL = "inbox"
    private const val ID = 1

    /** True when Arthik may show notifications (Android 13+ asks; any version can block them). */
    fun permitted(ctx: Context): Boolean {
        if (Build.VERSION.SDK_INT >= 33 &&
            ctx.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) return false
        return ctx.getSystemService(NotificationManager::class.java)?.areNotificationsEnabled() == true
    }

    /** Sets (or clears) the next daily notice from the settings. */
    fun schedule(ctx: Context) {
        val p = Prefs(ctx)
        if (!p.notifyEnabled) return DailyAlarm.cancel(ctx, NotifyAlarm::class.java, ALARM)
        p.notifyNextRun = DailyAlarm.set(ctx, NotifyAlarm::class.java, ALARM, p.notifyHour, p.notifyMinute)
    }

    /**
     * Checks the alert and notifies when something is due; [test] ("Check now") also
     * shows a notification when nothing is. Follows the shared alert time. Returns a
     * one-line result for Settings. Call off the main thread.
     */
    fun run(ctx: Context, test: Boolean = false): String {
        val s = Summary.load(ctx) ?: return "Couldn't read the finance folder"
        Widget.draw(ctx, s)
        follow(ctx, s.alert)
        val a = s.alert
        val nm = ctx.getSystemService(NotificationManager::class.java) ?: return "No notification service"
        if (!a.show && !test) {
            nm.cancel(ID)
            return "Nothing to tell"
        }
        if (!permitted(ctx)) return "Notifications are off for Arthik"
        nm.createNotificationChannel(NotificationChannel(CHANNEL, "Daily alert", NotificationManager.IMPORTANCE_DEFAULT).apply {
            description = "Bank balance mismatches and a full inbox, once a day"
        })
        val b = Notification.Builder(ctx, CHANNEL)
            .setSmallIcon(R.drawable.ic_stat_arthik)
            .setColor(Color.parseColor(TEAL))
            .setContentIntent(MainActivity.openIntent(ctx, a.route, TAP))
            .setAutoCancel(true)
            .setCategory(Notification.CATEGORY_REMINDER)
            .setShowWhen(true)
        if (!a.show) {
            b.setContentTitle("All good").setContentText("Nothing needs a look. This is how the daily alert looks.")
        } else {
            val style = Notification.InboxStyle().setBigContentTitle(a.title)
            a.mismatch.forEach { style.addLine("❌ $it") }
            if (a.limit > 0 && a.count > a.limit) s.notifyLines.forEach { style.addLine(it) }
            b.setContentTitle(a.title).setContentText(a.text).setStyle(style)
            if (a.count > 0) b.setNumber(a.count)
        }
        nm.notify(ID, b.build())
        return if (!a.show) "Test alert shown" else "Alerted: ${a.title}"
    }

    /** Moves this phone's alarm to the alert time in settings.md when it changed there. */
    private fun follow(ctx: Context, a: Summary.Alert) {
        val (h, m) = a.clock ?: return
        val p = Prefs(ctx)
        if (p.notifyHour == h && p.notifyMinute == m) return
        p.notifyHour = h
        p.notifyMinute = m
    }

    private const val TEAL = "#3A807A"
}

/** Fires at the chosen time: notify, then set tomorrow's run. */
class NotifyAlarm : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        val app = ctx.applicationContext
        val done = goAsync()
        Thread {
            try {
                Prefs(app).notifyLastResult = InboxNotify.run(app)
            } finally {
                InboxNotify.schedule(app)
                done.finish()
            }
        }.start()
    }
}
