package com.nihars.arthik

import java.io.File
import java.time.LocalDate

/**
 * inbox/sms_<device>.md in the finance folder. Only this phone writes it; Arthik on
 * nimory reads it and turns bank messages into drafts. One message per line, so the
 * file stays easy to read and to append to:
 *
 *   - {id: "…", from: "AX-HDFCBK", at: "2026-10-09T20:34:40+05:30", body: "…"}
 *
 * The header also says when the phone copies SMS ("schedule") and when it last did,
 * so the inbox on any device can show both steps' timings.
 */
class SmsStore(folder: String, device: String, private val keepDays: Long = 45) {
    data class Message(val id: String, val from: String, val at: String, val body: String)

    val file = File(File(folder, "inbox"), "sms_${device}.md")
    private val header = "# Arthik SMS capture — written only by the Arthik app on \"$device\". Do not edit; Arthik reads it into the inbox.\n"
    private val line = Regex("""^\s*- \{id: "([0-9a-f]{16})", .*at: "([^"]+)".*\}\s*$""")

    /**
     * Adds messages not saved yet and drops lines older than keepDays. Returns how many
     * were new. [schedule] ("23:30") and [ranAt] (ISO time) go into the header.
     */
    fun add(messages: List<Message>, today: LocalDate, schedule: String = "", ranAt: String = ""): Int {
        val kept = mutableListOf<String>()
        val ids = HashSet<String>()
        if (file.exists()) {
            for (l in file.readLines()) {
                val m = line.find(l) ?: continue // header and anything unexpected
                val day = SmsRules.dayOf(m.groupValues[2])
                if (day != null && day.isBefore(today.minusDays(keepDays))) continue
                if (ids.add(m.groupValues[1])) kept += l
            }
        }
        val fresh = messages.filter { ids.add(it.id) }
        if (fresh.isEmpty() && ranAt.isEmpty() && file.exists() && kept.size == countLines()) return 0
        kept += fresh.map { "  - {id: ${SmsRules.quote(it.id)}, from: ${SmsRules.quote(it.from)}, at: ${SmsRules.quote(it.at)}, body: ${SmsRules.quote(it.body)}}" }
        val times = (if (schedule.isNotEmpty()) "schedule: ${SmsRules.quote(schedule)}\n" else "") +
            (if (ranAt.isNotEmpty()) "last_run: ${SmsRules.quote(ranAt)}\n" else "")
        write(header + "device: ${SmsRules.quote(file.nameWithoutExtension.removePrefix("sms_"))}\n" + times + "messages:\n" +
            (if (kept.isEmpty()) "  []\n" else kept.joinToString("\n", postfix = "\n")))
        return fresh.size
    }

    private fun countLines() = file.readLines().count { line.matches(it) }

    // Temp file + rename, so Syncthing never picks up half a file.
    private fun write(text: String) {
        file.parentFile?.mkdirs()
        val tmp = File(file.parentFile, ".${file.name}.tmp")
        tmp.writeText(text)
        if (!tmp.renameTo(file)) {
            file.writeText(text)
            tmp.delete()
        }
    }
}
