package com.nihars.arthik

import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.res.Configuration
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Path
import android.view.View
import android.widget.RemoteViews

// One provider per widget the launcher lists; they all draw through Widget.
/** 4×2 overview: month spend vs budget and the inbox. Tap → inbox. */
class ArthikWidget : AppWidgetProvider() {
    override fun onUpdate(ctx: Context, mgr: AppWidgetManager, ids: IntArray) = Widget.refreshAsync(ctx.applicationContext, goAsync())
}

/** 2×2 budget: spent, budget and what's left. Tap → budgets. */
class BudgetWidget : AppWidgetProvider() {
    override fun onUpdate(ctx: Context, mgr: AppWidgetManager, ids: IntArray) = Widget.refreshAsync(ctx.applicationContext, goAsync())
}

/** 2×2 net worth with this month's change and a 12-month trend. Tap → overview. */
class NetWorthWidget : AppWidgetProvider() {
    override fun onUpdate(ctx: Context, mgr: AppWidgetManager, ids: IntArray) = Widget.refreshAsync(ctx.applicationContext, goAsync())
}

/** 2×2 inbox: drafts waiting and the newest one. Tap → inbox. */
class InboxWidget : AppWidgetProvider() {
    override fun onUpdate(ctx: Context, mgr: AppWidgetManager, ids: IntArray) = Widget.refreshAsync(ctx.applicationContext, goAsync())
}

object Widget {
    private enum class Kind(val provider: Class<out AppWidgetProvider>, val layout: Int, val route: String, val tap: Int) {
        OVERVIEW(ArthikWidget::class.java, R.layout.widget_arthik, "inbox", 10),
        BUDGET(BudgetWidget::class.java, R.layout.widget_budget, "budgets", 11),
        WORTH(NetWorthWidget::class.java, R.layout.widget_networth, "home", 12),
        INBOX(InboxWidget::class.java, R.layout.widget_inbox, "inbox", 13),
    }

    private fun ids(ctx: Context, k: Kind): IntArray =
        AppWidgetManager.getInstance(ctx).getAppWidgetIds(ComponentName(ctx, k.provider))

    /** True when at least one Arthik widget is on the home screen. */
    fun placed(ctx: Context) = Kind.entries.any { ids(ctx, it).isNotEmpty() }

    /** Shows the last numbers at once, then reads fresh ones in the background. */
    fun refreshAsync(ctx: Context, done: BroadcastReceiver.PendingResult?) {
        if (!placed(ctx)) { done?.finish(); return }
        draw(ctx, Summary.cached(ctx))
        Thread {
            try {
                draw(ctx, Summary.load(ctx) ?: Summary.cached(ctx))
            } finally {
                done?.finish()
            }
        }.start()
    }

    /** The look chosen in Settings → Phone, for the phone's current light/dark mode. */
    fun look(ctx: Context): WidgetLook {
        val p = Prefs(ctx)
        val night = ctx.resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK == Configuration.UI_MODE_NIGHT_YES
        return WidgetLook.resolve(p.widgetBg, p.widgetOpacity, p.widgetText, night)
    }

    /** Draws [s] on every placed widget (null = folder not set up yet). */
    fun draw(ctx: Context, s: Summary?) {
        val mgr = AppWidgetManager.getInstance(ctx)
        val look = look(ctx)
        for (k in Kind.entries) {
            val ids = ids(ctx, k)
            if (ids.isEmpty()) continue
            val v = RemoteViews(ctx.packageName, k.layout)
            paint(v, look)
            when {
                s == null -> empty(v, k)
                k == Kind.OVERVIEW -> overview(ctx, v, s, look)
                k == Kind.BUDGET -> budget(ctx, v, s, look)
                k == Kind.WORTH -> worth(v, s, look)
                k == Kind.INBOX -> inbox(v, s)
            }
            v.setOnClickPendingIntent(R.id.widget_root, MainActivity.openIntent(ctx, k.route, k.tap))
            mgr.updateAppWidget(ids, v)
        }
    }

    // Background tint and opacity, text colours. Every layout has these views.
    private fun paint(v: RemoteViews, l: WidgetLook) {
        v.setInt(R.id.w_bg, "setColorFilter", l.bg or (0xFF shl 24))
        v.setInt(R.id.w_bg, "setImageAlpha", l.alpha)
        for (id in intArrayOf(R.id.w_title, R.id.w_big)) v.setTextColor(id, l.text)
        for (id in intArrayOf(R.id.w_period, R.id.w_sub, R.id.w_line)) v.setTextColor(id, l.muted)
    }

    private fun empty(v: RemoteViews, k: Kind) {
        v.setTextViewText(R.id.w_period, "")
        v.setTextViewText(R.id.w_big, "—")
        v.setTextViewText(R.id.w_sub, "Open Arthik and choose your finance folder")
        v.setTextViewText(R.id.w_line, "")
        if (k == Kind.OVERVIEW || k == Kind.BUDGET) bars(v, -1, false)
        if (k == Kind.WORTH) v.setViewVisibility(R.id.w_spark, View.GONE)
    }

    private fun overview(ctx: Context, v: RemoteViews, s: Summary, l: WidgetLook) {
        v.setTextViewText(R.id.w_period, s.period)
        v.setTextViewText(R.id.w_big, s.headline)
        v.setTextViewText(R.id.w_sub, s.budgetLine)
        if (s.overBudget) v.setTextColor(R.id.w_sub, ctx.getColor(R.color.widget_over))
        bars(v, if (s.budget > 0) s.percent else -1, s.overBudget)
        v.setTextViewText(R.id.w_line, s.inboxLine)
        v.setTextColor(R.id.w_line, l.text)
        v.setImageViewResource(R.id.w_dot, if (s.pending > 0) R.drawable.widget_dot_on else R.drawable.widget_dot_off)
    }

    private fun budget(ctx: Context, v: RemoteViews, s: Summary, l: WidgetLook) {
        v.setTextViewText(R.id.w_period, s.period)
        v.setTextViewText(R.id.w_big, s.headline)
        v.setTextViewText(R.id.w_sub, if (s.budget > 0) "of ${Rupees.format(s.budget)}" else "spent this month")
        bars(v, if (s.budget > 0) s.percent else -1, s.overBudget)
        v.setTextViewText(R.id.w_line, if (s.budget > 0) "${s.leftLine} · ${s.percent}%" else s.leftLine)
        v.setTextColor(R.id.w_line, if (s.overBudget) ctx.getColor(R.color.widget_over) else l.muted)
    }

    private fun worth(v: RemoteViews, s: Summary, l: WidgetLook) {
        v.setTextViewText(R.id.w_big, Rupees.format(s.worth))
        v.setTextViewText(R.id.w_sub, s.worthLine)
        v.setTextColor(R.id.w_sub, when {
            s.worthChange > 0 -> if (l.dark) 0xFF4CC792.toInt() else 0xFF13895F.toInt()
            s.worthChange < 0 -> if (l.dark) 0xFFF0805F.toInt() else 0xFFC2462B.toInt()
            else -> l.muted
        })
        v.setTextViewText(R.id.w_line, "")
        if (s.trend.size >= 2) {
            v.setViewVisibility(R.id.w_spark, View.VISIBLE)
            v.setImageViewBitmap(R.id.w_spark, spark(s.trend, if (s.trend.last() < s.trend.first()) 0xFFE0703F.toInt() else l.accent))
        } else {
            v.setViewVisibility(R.id.w_spark, View.GONE)
        }
    }

    private fun inbox(v: RemoteViews, s: Summary) {
        v.setTextViewText(R.id.w_big, if (s.pending > 0) s.pending.toString() else "✓")
        v.setTextViewText(R.id.w_sub, s.inboxSub)
        val first = s.newest.firstOrNull()
        v.setTextViewText(R.id.w_line, when {
            first == null -> "Nothing waiting"
            s.out > 0 -> "${Rupees.format(s.out)} out · ${first.title}"
            else -> first.title
        })
        v.setImageViewResource(R.id.w_dot, if (s.pending > 0) R.drawable.widget_dot_on else R.drawable.widget_dot_off)
    }

    // percent < 0 hides both bars; over budget shows the full red bar.
    private fun bars(v: RemoteViews, percent: Int, over: Boolean) {
        v.setViewVisibility(R.id.w_bar, if (percent >= 0 && !over) View.VISIBLE else View.GONE)
        v.setViewVisibility(R.id.w_bar_over, if (over) View.VISIBLE else View.GONE)
        if (percent >= 0) v.setProgressBar(R.id.w_bar, 100, percent.coerceIn(0, 100), false)
    }

    /** A small trend line with a soft fill, drawn into a bitmap (widgets can't draw paths). */
    private fun spark(values: List<Long>, color: Int): Bitmap {
        val w = 360f
        val h = 96f
        val pad = 8f
        val bmp = Bitmap.createBitmap(w.toInt(), h.toInt(), Bitmap.Config.ARGB_8888)
        val c = Canvas(bmp)
        var lo = values.min()
        var hi = values.max()
        if (hi == lo) { hi += 100; lo -= 100 }
        val x = { i: Int -> pad + (w - 2 * pad) * i / (values.size - 1) }
        val y = { p: Long -> pad + (h - 2 * pad) * (1f - (p - lo).toFloat() / (hi - lo)) }
        val line = Path().apply { values.forEachIndexed { i, p -> if (i == 0) moveTo(x(i), y(p)) else lineTo(x(i), y(p)) } }
        val area = Path(line).apply { lineTo(x(values.size - 1), h); lineTo(x(0), h); close() }
        c.drawPath(area, Paint(Paint.ANTI_ALIAS_FLAG).apply { this.color = (color and 0x00FFFFFF) or (0x33 shl 24) })
        c.drawPath(line, Paint(Paint.ANTI_ALIAS_FLAG).apply {
            this.color = color; style = Paint.Style.STROKE; strokeWidth = 5f
            strokeCap = Paint.Cap.ROUND; strokeJoin = Paint.Join.ROUND
        })
        c.drawCircle(x(values.size - 1), y(values.last()), 7f, Paint(Paint.ANTI_ALIAS_FLAG).apply { this.color = color })
        return bmp
    }
}
