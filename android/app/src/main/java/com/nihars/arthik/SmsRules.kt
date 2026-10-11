package com.nihars.arthik

import java.security.MessageDigest
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter

/**
 * Pure rules for the nightly SMS capture (no Android types, unit-tested).
 * The phone only decides what to copy; reading amounts and merchants is the
 * engine's job (internal/inbox/sms_rules.yaml), so patterns live in one place.
 */
object SmsRules {
    private val money = Regex("(?i)(?:\\bINR|\\bRs\\.?|₹)\\s*[\\d,]+")
    private val secret = Regex("(?i)\\botp\\b|one[- ]time password|verification code|\\bcode is\\b")
    private val letters = Regex("[A-Za-z]")

    /** Bank and card messages come from named senders (AX-HDFCBK), never phone numbers. */
    fun fromBusiness(address: String?): Boolean = address != null && letters.containsMatchIn(address)

    /** Worth copying: from a business, mentions money, holds no secret code. */
    fun wanted(address: String?, body: String?): Boolean =
        body != null && fromBusiness(address) && money.containsMatchIn(body) && !secret.containsMatchIn(body)

    /**
     * Start of the window to read (epoch ms): everything after the last successful
     * run, but never before the day the capture was switched on and never more than
     * [lookbackDays] calendar days back.
     */
    fun windowStart(nowMs: Long, checkpointMs: Long, enabledAtMs: Long, lookbackDays: Int, zone: ZoneId): Long {
        val today = Instant.ofEpochMilli(nowMs).atZone(zone).toLocalDate()
        val floor = today.minusDays(lookbackDays.coerceIn(1, 30).toLong() - 1).atStartOfDay(zone).toInstant().toEpochMilli()
        return maxOf(checkpointMs, enabledAtMs, floor)
    }

    /** Next time (epoch ms) the clock shows hour:minute, strictly after now. */
    fun nextRun(nowMs: Long, hour: Int, minute: Int, zone: ZoneId): Long {
        val now = Instant.ofEpochMilli(nowMs).atZone(zone)
        var at = now.withHour(hour.coerceIn(0, 23)).withMinute(minute.coerceIn(0, 59)).withSecond(0).withNano(0)
        if (!at.isAfter(now)) at = at.plusDays(1)
        return at.toInstant().toEpochMilli()
    }

    /** Stable message id: the same text from the same sender at the same time is one message. */
    fun id(address: String, atIso: String, body: String): String {
        val d = MessageDigest.getInstance("SHA-256").digest("$address\n$atIso\n$body".toByteArray())
        return d.take(8).joinToString("") { "%02x".format(it) }
    }

    /** RFC 3339 time with offset, as the engine reads it. */
    fun iso(ms: Long, zone: ZoneId): String =
        ZonedDateTime.ofInstant(Instant.ofEpochMilli(ms), zone).truncatedTo(java.time.temporal.ChronoUnit.SECONDS)
            .format(DateTimeFormatter.ISO_OFFSET_DATE_TIME)

    /** Date part of an ISO time ("" when unreadable). */
    fun dayOf(iso: String): LocalDate? = try { ZonedDateTime.parse(iso).toLocalDate() } catch (_: Exception) { null }

    /** A YAML double-quoted string (escapes \ " and control characters). */
    fun quote(s: String): String {
        val b = StringBuilder("\"")
        for (c in s) {
            when {
                c == '\\' -> b.append("\\\\")
                c == '"' -> b.append("\\\"")
                c == '\n' -> b.append("\\n")
                c == '\r' -> b.append("\\r")
                c == '\t' -> b.append("\\t")
                c < ' ' || c == '\u007f' || c == ' ' || c == ' ' -> b.append("\\u%04x".format(c.code))
                else -> b.append(c)
            }
        }
        return b.append('"').toString()
    }
}
