package com.nihars.arthik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class PinTest {
    @Test
    fun acceptsOnlyFourToEightDigits() {
        assertTrue(Pin.valid("1234"))
        assertTrue(Pin.valid("12345678"))
        assertFalse(Pin.valid("123"))
        assertFalse(Pin.valid("123456789"))
        assertFalse(Pin.valid("12a4"))
    }

    @Test
    fun verifiesTheRightPinOnly() {
        val stored = Pin.hash("2468")
        assertTrue(Pin.verify("2468", stored))
        assertFalse(Pin.verify("2469", stored))
        assertFalse(Pin.verify("", stored))
    }

    @Test
    fun saltsEveryHash() {
        assertNotEquals(Pin.hash("1111"), Pin.hash("1111"))
        assertEquals(4, Pin.hash("1111").split(":").size)
    }

    @Test
    fun rejectsDamagedHashes() {
        assertFalse(Pin.verify("1234", ""))
        assertFalse(Pin.verify("1234", "v1:x:y:z"))
        assertFalse(Pin.verify("1234", "v2:60000:AAAA:AAAA"))
    }
}
