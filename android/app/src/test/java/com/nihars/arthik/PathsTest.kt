package com.nihars.arthik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class PathsTest {
    @Test
    fun internalStorage() {
        assertEquals("/storage/emulated/0/Notes/data/finance", Paths.fromTreeDocId("primary:Notes/data/finance"))
        assertEquals("/storage/emulated/0", Paths.fromTreeDocId("primary:"))
        assertEquals("/storage/emulated/0/a b", Paths.fromTreeDocId("primary:a b/"))
    }

    @Test
    fun sdCardAndShortcuts() {
        assertEquals("/storage/1A2B-3C4D/finance", Paths.fromTreeDocId("1A2B-3C4D:finance"))
        assertEquals("/storage/emulated/0/Documents/x", Paths.fromTreeDocId("home:x"))
        assertEquals("/storage/emulated/0/Download/f", Paths.fromTreeDocId("raw:/storage/emulated/0/Download/f"))
    }

    @Test
    fun rejectsNonFolders() {
        assertNull(Paths.fromTreeDocId("msd:123"))
        assertNull(Paths.fromTreeDocId("nocolon"))
        assertNull(Paths.fromTreeDocId("raw:relative/path"))
        assertNull(Paths.clean("/storage/emulated/0/../../etc"))
        assertNull(Paths.clean(""))
    }

    @Test
    fun customPrimaryRoot() {
        assertEquals("/sdcard/f", Paths.fromTreeDocId("primary:f", "/sdcard"))
    }
}
