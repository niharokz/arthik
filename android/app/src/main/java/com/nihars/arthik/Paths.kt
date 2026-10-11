package com.nihars.arthik

/** Turns Android folder-picker results into real file paths. Pure code, unit tested. */
object Paths {
    /**
     * Converts a tree document id from the system folder picker to a path.
     * "primary:Notes/data/finance" → "/storage/emulated/0/Notes/data/finance"
     * "1A2B-3C4D:finance"         → "/storage/1A2B-3C4D/finance" (SD card)
     * "raw:/storage/emulated/0/x" → "/storage/emulated/0/x" (Downloads provider)
     * Returns null for ids that are not real folders (cloud providers, etc.).
     */
    fun fromTreeDocId(docId: String, primaryRoot: String = "/storage/emulated/0"): String? {
        if (docId.startsWith("raw:")) return clean(docId.removePrefix("raw:"))
        val colon = docId.indexOf(':')
        if (colon <= 0) return null
        val volume = docId.substring(0, colon)
        val rel = docId.substring(colon + 1).trim('/')
        val root = when {
            volume == "primary" -> primaryRoot
            volume == "home" -> "$primaryRoot/Documents" // "Documents" shortcut
            Regex("^[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}$").matches(volume) -> "/storage/$volume"
            else -> return null
        }
        return clean(if (rel.isEmpty()) root else "$root/$rel")
    }

    /** Trims spaces and trailing slashes; rejects relative paths and "..". */
    fun clean(path: String): String? {
        val p = path.trim().trimEnd('/')
        if (!p.startsWith("/") || p.split('/').any { it == ".." }) return null
        return p.ifEmpty { null }
    }
}
