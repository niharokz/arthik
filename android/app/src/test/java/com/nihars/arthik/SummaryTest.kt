package com.nihars.arthik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class SummaryTest {
    @Test
    fun readsEngineAmounts() {
        assertEquals(1636350L, Rupees.paise("16363.50"))
        assertEquals(1636300L, Rupees.paise("16363"))
        assertEquals(1636310L, Rupees.paise("16,363.1"))
        assertEquals(-50000L, Rupees.paise("-500.00"))
        assertEquals(0L, Rupees.paise("abc"))
        assertEquals(0L, Rupees.paise(null))
    }

    @Test
    fun formatsIndianRupees() {
        assertEquals("₹0", Rupees.format(0))
        assertEquals("₹486", Rupees.format(48600))
        assertEquals("₹16,364", Rupees.format(1636350)) // rounds to whole rupees
        assertEquals("₹1,28,500", Rupees.format(12850000))
        assertEquals("₹1,23,45,678", Rupees.format(1234567800))
        assertEquals("-₹500", Rupees.format(-50000))
    }

    private fun s(budget: Long = 3150000, spent: Long = 1636300, pending: Int = 6, out: Long = 459400) = Summary(
        period = "Oct 2026", expense = 4949200, budget = budget, spent = spent, percent = if (budget > 0) (spent * 100 / budget).toInt() else 0,
        pending = pending, out = out,
        newest = listOf(Summary.Draft("Swiggy", "expense", 48600), Summary.Draft("Refund", "income", 30000)),
    )

    @Test
    fun widgetWords() {
        assertEquals("₹16,363", s().headline)
        assertEquals("of ₹31,500 · 51%", s().budgetLine)
        assertFalse(s().overBudget)
        assertEquals("₹2,000 over budget", s(spent = 3350000).budgetLine)
        assertTrue(s(spent = 3350000).overBudget)
        assertEquals("₹49,492", s(budget = 0).headline) // no budgets: everything spent
        assertEquals("spent this month", s(budget = 0).budgetLine)
        assertEquals("6 to review · ₹4,594 out", s().inboxLine)
        assertEquals("Inbox clear", s(pending = 0).inboxLine)
        assertEquals("2 to review", s(pending = 2, out = 0).inboxLine)
    }

    @Test
    fun notificationWords() {
        assertEquals("6 drafts to review", s().notifyTitle)
        assertEquals("1 draft to review", s(pending = 1).notifyTitle)
        assertEquals("₹4,594 out · Swiggy, Refund", s().notifyText)
        assertEquals(listOf("Swiggy · ₹486", "Refund · +₹300", "and 4 more"), s().notifyLines)
        assertEquals(listOf("Swiggy · ₹486", "Refund · +₹300"), s(pending = 2).notifyLines)
    }
}
