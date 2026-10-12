package com.nihars.arthik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class WidgetLookTest {
    @Test
    fun autoFollowsLightAndDark() {
        val day = WidgetLook.resolve("auto", 100, "auto", night = false)
        assertEquals(0xFFFFFF, day.bg)
        assertFalse(day.dark)
        val night = WidgetLook.resolve("auto", 100, "auto", night = true)
        assertEquals(0x1A2120, night.bg)
        assertTrue(night.dark)
    }

    @Test
    fun amoledAndCustomColours() {
        val amoled = WidgetLook.resolve("#000000", 100, "auto", night = false)
        assertEquals(0x000000, amoled.bg)
        assertEquals(255, amoled.alpha)
        assertTrue("light text on black", amoled.dark)
        assertFalse("dark text on pale yellow", WidgetLook.resolve("#fff3c4", 100, "auto", false).dark)
        assertEquals("a bad colour falls back to white", 0xFFFFFF, WidgetLook.resolve("nope", 100, "auto", false).bg)
    }

    @Test
    fun opacityAndText() {
        assertEquals(0, WidgetLook.resolve("#000000", 0, "auto", false).alpha)
        assertEquals(89, WidgetLook.resolve("#000000", 35, "auto", false).alpha)
        assertTrue("see-through: light text", WidgetLook.resolve("#ffffff", 10, "auto", false).dark)
        assertFalse("forced dark text", WidgetLook.resolve("#000000", 0, "dark", false).dark)
        val l = WidgetLook.resolve("#000000", 100, "auto", false)
        assertEquals(l.text and 0xFFFFFF, l.muted and 0xFFFFFF) // muted = text, fainter
        assertEquals(0xB3, l.muted ushr 24)
    }

    @Test
    fun parsesHex() {
        assertEquals(0x1A2120, WidgetLook.rgb("#1a2120"))
        assertEquals(0xABCDEF, WidgetLook.rgb("ABCDEF"))
        assertNull(WidgetLook.rgb("#abc"))
        assertEquals(0, WidgetLook.luma(0))
        assertEquals(255, WidgetLook.luma(0xFFFFFF))
    }

    @Test
    fun widgetWords() {
        val s = Summary("Oct 2026", 4949200, 3150000, 1636300, 51, 1, 48600, listOf(Summary.Draft("Swiggy", "expense", 48600)),
            worth = 75249600, worthChange = 7900800, trend = listOf(1, 2))
        assertEquals("₹15,137 left", s.leftLine)
        assertEquals("▲ ₹79,008 this month", s.worthLine)
        assertEquals("▼ ₹500 this month", s.copy(worthChange = -50000).worthLine)
        assertEquals("No change this month", s.copy(worthChange = 0).worthLine)
        assertEquals("draft to review", s.inboxSub)
        assertEquals("drafts to review", s.copy(pending = 3).inboxSub)
        assertEquals("Inbox clear", s.copy(pending = 0).inboxSub)
        assertEquals("₹2,000 over", s.copy(spent = 3350000).leftLine)
        assertEquals("no budgets set", s.copy(budget = 0).leftLine)
    }
}
