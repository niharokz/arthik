package com.nihars.arthik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.nio.file.Files

class AppConfigTest {
    private val values = mapOf<String, Any?>(
        "mode" to "folder", "folder" to "/storage/emulated/0/Notes/data/finance", "primary" to false,
        "lock_mode" to "device", "lock_after" to 300, "sms_enabled" to true, "sms_enabled_at" to 1760900000000L,
        "sms_hour" to 1, "sms_minute" to 30, "widget_bg" to "#000000", "widget_text" to "say \"hi\" \\ there",
        "pin_hash" to "v1:60000:salt:hash", "left_at" to 123L, // never written
    )

    @Test fun roundTrip() {
        val text = AppConfig.write(values, "2026-10-20T14:00:00+05:30")
        assertFalse("the PIN hash must stay on the phone", text.contains("pin_hash"))
        assertFalse(text.contains("left_at"))
        val back = AppConfig.read(text)
        for ((k, v) in values) if (k in AppConfig.KEYS) assertEquals(k, v, back[k])
        assertNull(back["pin_hash"])
    }

    @Test fun unknownKeysIgnoredBadValuesRejected() {
        assertEquals(mapOf("sms_hour" to 2), AppConfig.read("# note\nsaved: \"x\"\nfuture_key: 1\nsms_hour: 2\n"))
        try {
            AppConfig.read("sms_enabled: maybe\n")
            throw AssertionError("bad boolean accepted")
        } catch (e: IllegalArgumentException) {
            assertTrue(e.message!!.contains("sms_enabled"))
        }
    }

    @Test fun saveAndLoad() {
        val dir = Files.createTempDirectory("cfg").toFile()
        assertNull(AppConfig.load(dir.path))
        AppConfig.save(dir.path + "/nested", values, "now")
        assertEquals("folder", AppConfig.load(dir.path + "/nested")!!["mode"])
        dir.deleteRecursively()
    }
}
