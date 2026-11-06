package com.nihars.arthik

/**
 * Colours of the home-screen widgets. They follow Settings → Look (theme, colours and
 * card background, from settings.md): the app sends the resolved colours whenever it
 * applies the look, encoded as "bg|opacity|text|muted|accent" (#rrggbb, opacity
 * 0..100 from the card background: none, glass, matte, opaque). Pure, so it is unit tested.
 */
data class WidgetLook(
    val bg: Int, // RGB of the background
    val alpha: Int, // 0..255
    val text: Int, // ARGB
    val muted: Int, // ARGB, secondary text
    val accent: Int, // ARGB, sparkline and bar
    val dark: Boolean, // light text (a dark theme or a see-through card)
) {
    companion object {
        private const val OPAQUE = 0xFF shl 24
        private const val LIGHT_TEXT = 140 // text brighter than this reads as a dark theme

        /** Until the app has sent its look: the nss dark, matte card. */
        val DEFAULT = WidgetLook(0x0D1412, 255, 0xFFE4EBE8.toInt(), 0xFF93A29E.toInt(), 0xFF5CBDB1.toInt(), true)

        /** "#1a2120" → 0x1A2120; null when it isn't a colour. */
        fun rgb(hex: String): Int? {
            val h = hex.trim().removePrefix("#")
            return if (Regex("[0-9a-fA-F]{6}").matches(h)) h.toInt(16) else null
        }

        /** Perceived brightness 0..255 of an RGB colour. */
        fun luma(rgb: Int): Int = ((rgb shr 16 and 0xFF) * 299 + (rgb shr 8 and 0xFF) * 587 + (rgb and 0xFF) * 114) / 1000

        /** Reads what the app sent; null when it is malformed. */
        fun decode(s: String): WidgetLook? {
            val p = s.split("|")
            if (p.size != 5) return null
            val bg = rgb(p[0]) ?: return null
            val opacity = p[1].trim().toIntOrNull()?.takeIf { it in 0..100 } ?: return null
            val text = rgb(p[2]) ?: return null
            val muted = rgb(p[3]) ?: return null
            val accent = rgb(p[4]) ?: return null
            return WidgetLook(
                bg = bg, alpha = (opacity * 255 + 50) / 100, text = text or OPAQUE, muted = muted or OPAQUE,
                accent = accent or OPAQUE, dark = luma(text) > LIGHT_TEXT,
            )
        }
    }
}
