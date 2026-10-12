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
 * The morning notice: once a day, at the time set in Settings → Phone, tells you how
 * many drafts wait in the inbox (silent when there are none). Replaces Kronos's
 * Telegram message for people who prefer a phone notification.
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
     * Reads the inbox and notifies when drafts wait; [test] also shows a notice when
     * the inbox is empty. Returns a one-line result for Settings. Call off the main thread.
     */
    fun run(ctx: Context, test: Boolean = false): String {
        val s = Summary.load(ctx) ?: return "Couldn't read the finance folder"
        Widget.draw(ctx, s)
        val nm = ctx.getSystemService(NotificationManager::class.java) ?: return "No notification service"
        if (s.pending == 0 && !test) {
            nm.cancel(ID)
            return "Nothing to review"
        }
        if (!permitted(ctx)) return "Notifications are off for Arthik"
        nm.createNotificationChannel(NotificationChannel(CHANNEL, "Inbox", NotificationManager.IMPORTANCE_DEFAULT).apply {
            description = "Drafts from bank SMS and subscriptions waiting for review"
        })
        val b = Notification.Builder(ctx, CHANNEL)
            .setSmallIcon(R.drawable.ic_stat_arthik)
            .setColor(Color.parseColor(TEAL))
            .setContentIntent(MainActivity.openIntent(ctx, "inbox", TAP))
            .setAutoCancel(true)
            .setCategory(Notification.CATEGORY_REMINDER)
            .setShowWhen(true)
        if (s.pending == 0) {
            b.setContentTitle("Inbox clear").setContentText("Nothing waiting. This is how the morning notice looks.")
        } else {
            val style = Notification.InboxStyle().setBigContentTitle(s.notifyTitle)
            s.notifyLines.forEach { style.addLine(it) }
            if (s.out > 0) style.setSummaryText("${Rupees.format(s.out)} out")
            b.setContentTitle(s.notifyTitle).setContentText(s.notifyText).setStyle(style).setNumber(s.pending)
        }
        nm.notify(ID, b.build())
        return if (s.pending == 0) "Test notice shown" else "Notified: ${s.notifyTitle}"
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
