package com.nihars.arthik

import kotlin.math.abs

/** Money as the engine writes it ("16363.00") and as the widget shows it ("₹16,363"). */
object Rupees {
    /** "16363.50" → 1636350 paise; 0 for anything that isn't an amount. */
    fun paise(s: String?): Long {
        val t = s?.trim()?.replace(",", "") ?: return 0
        if (!Regex("-?\\d+(\\.\\d{1,2})?").matches(t)) return 0
        val neg = t.startsWith("-")
        val (whole, frac) = t.removePrefix("-").split(".").let { it[0] to (it.getOrNull(1) ?: "") }
        val p = whole.toLong() * 100 + frac.padEnd(2, '0').toLong()
        return if (neg) -p else p
    }

    /** Whole rupees with Indian digit grouping: 1636350 → "₹16,364", -50000 → "-₹500". */
    fun format(paise: Long): String {
        val rupees = (abs(paise) + 50) / 100
        val digits = rupees.toString()
        val grouped = if (digits.length <= 3) digits else {
            val head = digits.dropLast(3)
            head.reversed().chunked(2).joinToString(",").reversed() + "," + digits.takeLast(3)
        }
        return (if (paise < 0 && rupees > 0) "-₹" else "₹") + grouped
    }
}
