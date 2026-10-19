package com.nihars.arthik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.nio.file.Files
import java.time.LocalDate
import java.time.ZoneId
import java.time.ZonedDateTime

class SmsTest {
    private val ist = ZoneId.of("Asia/Kolkata")
    private fun ms(s: String) = ZonedDateTime.parse(s).toInstant().toEpochMilli()

    @Test
    fun copiesOnlyBankMoneyMessages() {
        assertTrue(SmsRules.wanted("AX-HDFCBK", "Spent Rs.2000 On HDFC Bank Card 4111 At GYFTR"))
        assertTrue(SmsRules.wanted("JD-IDFCFB", "Your A/C XXXXX819001 is credited with INR 400.00"))
        assertFalse(SmsRules.wanted("+919812345678", "Send me Rs 500 please")) // a person
        assertFalse(SmsRules.wanted("AX-HDFCBK", "123456 is your OTP for txn of Rs 500"))
        assertFalse(SmsRules.wanted("AX-SWIGGY", "Your order is on the way"))
        assertFalse(SmsRules.wanted(null, "Rs 10"))
    }

    @Test
    fun windowNeverBackfills() {
        val now = ms("2026-10-11T23:30:00+05:30")
        val enabled = ms("2026-10-11T00:00:00+05:30")
        // first night: from the start of the day it was switched on
        assertEquals(enabled, SmsRules.windowStart(now, 0, enabled, 3, ist))
        // normal night: from the last run
        val last = ms("2026-10-10T23:30:00+05:30")
        assertEquals(last, SmsRules.windowStart(now, last, 0, 3, ist))
        // a week missed: only the last 3 days
        val old = ms("2026-10-01T23:30:00+05:30")
        assertEquals(ms("2026-10-09T00:00:00+05:30"), SmsRules.windowStart(now, old, 0, 3, ist))
    }

    @Test
    fun nextRunIsTonightOrTomorrow() {
        assertEquals(ms("2026-10-11T23:30:00+05:30"), SmsRules.nextRun(ms("2026-10-11T10:00:00+05:30"), 23, 30, ist))
        assertEquals(ms("2026-10-12T23:30:00+05:30"), SmsRules.nextRun(ms("2026-10-11T23:30:00+05:30"), 23, 30, ist))
    }

    @Test
    fun quoteIsYamlSafe() {
        assertEquals("\"a \\\"b\\\" \\\\ c\\nd ₹5/6\"", SmsRules.quote("a \"b\" \\ c\nd ₹5/6"))
        assertEquals("\"\\u0007\"", SmsRules.quote("\u0007"))
        assertEquals(16, SmsRules.id("AX-HDFCBK", "2026-10-09T20:34:40+05:30", "x").length)
        assertEquals(SmsRules.id("A", "t", "b"), SmsRules.id("A", "t", "b"))
    }

    @Test
    fun storeAddsOnceAndPrunes() {
        val dir = Files.createTempDirectory("fin").toFile()
        val s = SmsStore(dir.path, "phone", keepDays = 45)
        val today = LocalDate.of(2026, 10, 11)
        val a = SmsStore.Message(SmsRules.id("AX", "2026-10-11T09:00:00+05:30", "Rs 1"), "AX", "2026-10-11T09:00:00+05:30", "Rs 1 \"quoted\"")
        val old = SmsStore.Message(SmsRules.id("AX", "2026-07-01T09:00:00+05:30", "Rs 2"), "AX", "2026-07-01T09:00:00+05:30", "Rs 2")
        assertEquals(2, s.add(listOf(a, old), LocalDate.of(2026, 7, 2)))
        assertEquals(0, s.add(listOf(a), LocalDate.of(2026, 7, 2)))
        val text = File(dir, "inbox/sms_phone.md").readText()
        assertTrue(text.contains("device: \"phone\""))
        assertTrue(text.contains("body: \"Rs 1 \\\"quoted\\\"\""))
        assertEquals(0, s.add(emptyList(), today)) // prunes the July message
        assertFalse(File(dir, "inbox/sms_phone.md").readText().contains("Rs 2"))
        assertTrue(File(dir, "inbox/sms_phone.md").readText().contains("Rs 1"))
        // A run with nothing new still records when it ran, for the inbox screen.
        assertEquals(0, s.add(emptyList(), today, "23:30", "2026-10-11T23:30:02+05:30"))
        val stamped = File(dir, "inbox/sms_phone.md").readText()
        assertTrue(stamped.contains("schedule: \"23:30\"\nlast_run: \"2026-10-11T23:30:02+05:30\"\nmessages:"))
        assertTrue(stamped.contains("Rs 1"))
        dir.deleteRecursively()
    }
}
