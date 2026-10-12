package com.nihars.arthik

/**
 * Colours of the home-screen widgets, chosen in Settings → Phone: a background colour
 * ("auto" follows light/dark), its opacity (0 = fully transparent) and the text
 * colour ("auto" picks whatever reads on that background). Pure, so it is unit tested.
 */
data class WidgetLook(
    val bg: Int, // RGB of the background
    val alpha: Int, // 0..255
    val text: Int, // ARGB
    val muted: Int, // ARGB, secondary text
    val accent: Int, // ARGB, sparkline and bar
    val dark: Boolean, // light text on a dark (or see-through) background
) {
    companion object {
        private const val LIGHT_BG = 0xFFFFFF
        private const val DARK_BG = 0x1A2120
        private const val INK = 0xFF13201E.toInt()
        private const val PAPER = 0xFFF1F5F4.toInt()
        private const val TEAL = 0xFF3A807A.toInt()
        private const val TEAL_LIGHT = 0xFF6CC3B9.toInt()

        /** Ready-made looks offered in Settings: id → (background, opacity %, text). */
        val PRESETS = linkedMapOf(
            "auto" to Triple("auto", 100, "auto"),
            "light" to Triple("#FFFFFF", 100, "auto"),
            "dark" to Triple("#1A2120", 100, "auto"),
            "amoled" to Triple("#000000", 100, "auto"),
            "glass" to Triple("#000000", 35, "light"),
            "clear" to Triple("#000000", 0, "light"),
        )

        /** "#1a2120" → 0x1A2120; null when it isn't a colour. */
        fun rgb(hex: String): Int? {
            val h = hex.trim().removePrefix("#")
            return if (Regex("[0-9a-fA-F]{6}").matches(h)) h.toInt(16) else null
        }

        /** Perceived brightness 0..255 of an RGB colour. */
        fun luma(rgb: Int): Int = ((rgb shr 16 and 0xFF) * 299 + (rgb shr 8 and 0xFF) * 587 + (rgb and 0xFF) * 114) / 1000

        /** Resolves the saved settings; [night] is the phone's dark mode. */
        fun resolve(bg: String, opacity: Int, text: String, night: Boolean): WidgetLook {
            val color = if (bg == "auto") (if (night) DARK_BG else LIGHT_BG) else rgb(bg) ?: LIGHT_BG
            val alpha = (opacity.coerceIn(0, 100) * 255 + 50) / 100
            val dark = when (text) {
                "light" -> true
                "dark" -> false
                // mostly see-through: the wallpaper decides, and light text is the safer bet
                else -> if (opacity < 40) true else luma(color) < 140
            }
            val ink = if (dark) PAPER else INK
            return WidgetLook(
                bg = color, alpha = alpha, text = ink,
                muted = (ink and 0x00FFFFFF) or (0xB3 shl 24),
                accent = if (dark) TEAL_LIGHT else TEAL, dark = dark,
            )
        }
    }
}
