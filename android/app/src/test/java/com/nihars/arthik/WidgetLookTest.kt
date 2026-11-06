package com.nihars.arthik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class WidgetLookTest {
    @Test
    fun readsTheAppLook() {
        val gold = WidgetLook.decode("#15120c|100|#ece5d6|#a59c88|#c9a45c")!!
        assertEquals(0x15120C, gold.bg)
        assertEquals(255, gold.alpha)
        assertEquals(0xFFC9A45C.toInt(), gold.accent)
        assertTrue("light text on a dark theme", gold.dark)
        val white = WidgetLook.decode("#ffffff|100|#161a19|#5f6866|#2f6f69")!!
        assertFalse(white.dark)
    }

    @Test
    fun cardBackgroundIsOpacity() {
        assertEquals(0, WidgetLook.decode("#0d1412|0|#e4ebe8|#93a29e|#5cbdb1")!!.alpha) // none
        assertEquals(140, WidgetLook.decode("#0d1412|55|#e4ebe8|#93a29e|#5cbdb1")!!.alpha) // glass
    }

    @Test
    fun badInputIsRefused() {
        assertNull(WidgetLook.decode(""))
        assertNull(WidgetLook.decode("#0d1412|100|#e4ebe8|#93a29e")) // four parts
        assertNull(WidgetLook.decode("#0d1412|101|#e4ebe8|#93a29e|#5cbdb1"))
        assertNull(WidgetLook.decode("nope|100|#e4ebe8|#93a29e|#5cbdb1"))
        assertTrue(WidgetLook.DEFAULT.dark)
    }
}
