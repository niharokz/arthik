package com.nihars.arthik

import android.content.Context
import org.json.JSONException
import org.json.JSONObject
import java.io.File
import java.util.TimeZone
import java.util.concurrent.TimeUnit

/**
 * This month's spending against the budgets, the drafts waiting in the inbox and net
 * worth — what the home-screen widgets and the morning notification show. The engine computes
 * it (`libarthik.so summary`, read-only), so the numbers match the app exactly.
 */
data class Summary(
    val period: String, // "Oct 2026"
    val expense: Long, // paise spent this month (all categories)
    val budget: Long, // paise budgeted (0 = no budgets set)
    val spent: Long, // paise spent in budgeted categories
    val percent: Int,
    val pending: Int,
    val out: Long, // paise of pending expenses
    val newest: List<Draft>,
    val worth: Long = 0, // paise, now
    val worthChange: Long = 0, // paise since the end of last month
    val trend: List<Long> = emptyList(), // net worth at the last 12 month-ends, oldest first
) {
    data class Draft(val title: String, val type: String, val amount: Long)

    // ---- the words shown on the widget and in the notification ----

    /** Headline amount: spent against the budget when there is one, else everything spent. */
    val headline: String get() = Rupees.format(if (budget > 0) spent else expense)

    /** "of ₹31,500 · 51%", "₹2,000 over budget", or "spent this month". */
    val budgetLine: String get() = when {
        budget <= 0 -> "spent this month"
        spent > budget -> "${Rupees.format(spent - budget)} over budget"
        else -> "of ${Rupees.format(budget)} · $percent%"
    }

    val overBudget: Boolean get() = budget > 0 && spent > budget

    /** "₹15,137 left", "₹2,000 over" or "no budgets set". */
    val leftLine: String get() = when {
        budget <= 0 -> "no budgets set"
        spent > budget -> "${Rupees.format(spent - budget)} over"
        else -> "${Rupees.format(budget - spent)} left"
    }

    /** "▲ ₹79,008 this month", "▼ ₹1,200 this month" or "No change this month". */
    val worthLine: String get() = when {
        worthChange > 0 -> "▲ ${Rupees.format(worthChange)} this month"
        worthChange < 0 -> "▼ ${Rupees.format(-worthChange)} this month"
        else -> "No change this month"
    }

    /** Big number on the inbox widget's second line. */
    val inboxSub: String get() = when (pending) {
        0 -> "Inbox clear"
        1 -> "draft to review"
        else -> "drafts to review"
    }

    /** "6 to review · ₹4,594 out" or "Inbox clear". */
    val inboxLine: String get() = when {
        pending == 0 -> "Inbox clear"
        out > 0 -> "$pending to review · ${Rupees.format(out)} out"
        else -> "$pending to review"
    }

    /** Notification title: "1 draft to review" / "6 drafts to review". */
    val notifyTitle: String get() = if (pending == 1) "1 draft to review" else "$pending drafts to review"

    /** One-line notification text: money out and the newest titles. */
    val notifyText: String get() {
        val names = newest.joinToString(", ") { it.title }
        return listOfNotNull(if (out > 0) "${Rupees.format(out)} out" else null, names.ifEmpty { null }).joinToString(" · ")
    }

    /** Expanded notification lines: "Swiggy · ₹486". */
    val notifyLines: List<String> get() = newest.map { d ->
        val sign = if (d.type == "income") "+" else ""
        "${d.title} · $sign${Rupees.format(d.amount)}"
    } + if (pending > newest.size) listOf("and ${pending - newest.size} more") else emptyList()

    companion object {
        private const val TIMEOUT_S = 20L

        /** Reads the engine's JSON (see internal/feature/summary). */
        fun parse(json: String): Summary? = try {
            val o = JSONObject(json)
            val b = o.optJSONObject("budget") ?: JSONObject()
            val box = o.optJSONObject("inbox") ?: JSONObject()
            val list = box.optJSONArray("newest")
            val nw = o.optJSONObject("net_worth") ?: JSONObject()
            val trend = nw.optJSONArray("trend")
            val newest = (0 until (list?.length() ?: 0)).map { i ->
                val d = list!!.getJSONObject(i)
                Draft(d.optString("title"), d.optString("type"), Rupees.paise(d.optString("amount")))
            }
            Summary(
                period = o.optJSONObject("period")?.optString("label") ?: "",
                expense = Rupees.paise(o.optString("expense")),
                budget = Rupees.paise(b.optString("budget")),
                spent = Rupees.paise(b.optString("spent")),
                percent = b.optInt("percent"),
                pending = box.optInt("pending"),
                out = Rupees.paise(box.optString("out")),
                newest = newest,
                worth = Rupees.paise(nw.optString("now")),
                worthChange = Rupees.paise(nw.optString("change")),
                trend = (0 until (trend?.length() ?: 0)).map { Rupees.paise(trend!!.optString(it)) },
            )
        } catch (e: JSONException) {
            null
        }

        /** The last summary read (instant; may be a few hours old). */
        fun cached(ctx: Context): Summary? = Prefs(ctx).summaryJson.takeIf { it.isNotEmpty() }?.let(::parse)

        /**
         * Asks the engine for a fresh summary of the finance folder and remembers it.
         * Null when no folder is set up or the engine fails. Call off the main thread.
         */
        fun load(ctx: Context): Summary? {
            val prefs = Prefs(ctx)
            if (prefs.mode != "folder" || !Storage.looksUsable(prefs.folder)) return null
            val bin = File(ctx.applicationInfo.nativeLibraryDir, "libarthik.so")
            if (!bin.canExecute()) return null
            val pb = ProcessBuilder(bin.absolutePath, "summary").directory(ctx.filesDir)
            pb.environment().apply {
                keys.removeAll { it.startsWith("ARTHIK_") }
                put("ARTHIK_DATA", prefs.folder)
                put("TZ", TimeZone.getDefault().id)
                put("HOME", ctx.filesDir.absolutePath)
            }
            return try {
                val p = pb.start()
                p.outputStream.close()
                val watchdog = Thread { if (!p.waitFor(TIMEOUT_S, TimeUnit.SECONDS)) p.destroy() }.apply { isDaemon = true; start() }
                val out = p.inputStream.bufferedReader().readText()
                val err = p.errorStream.bufferedReader().readText()
                watchdog.join()
                if (p.exitValue() != 0) {
                    (ctx.applicationContext as? ArthikApp)?.engine?.log?.append("summary failed: ${err.trim()}")
                    return null
                }
                parse(out)?.also {
                    prefs.summaryJson = out.trim()
                    prefs.summaryAt = System.currentTimeMillis()
                }
            } catch (e: Exception) {
                null
            }
        }
    }
}
