package com.nihars.arthik

import android.app.Application
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

/** Process-wide state that must survive the screen being recreated (rotation, dark mode). */
class ArthikApp : Application() {
    lateinit var prefs: Prefs
        private set
    lateinit var engine: Engine
        private set

    /** One background thread for engine starts, downloads, etc. */
    val bg: ExecutorService = Executors.newSingleThreadExecutor()

    /** PIN or fingerprint accepted since the last lock. */
    @Volatile var unlocked = false

    /**
     * True when the app was unlocked when you left it and the lock delay has not run
     * out — also after Android stopped the process while you were in another app.
     */
    fun stillUnlocked(): Boolean {
        val left = prefs.leftAt
        return left > 0 && System.currentTimeMillis() - left in 0 until prefs.lockAfter * 1000L
    }
    /** Last folder error shown on the welcome/error screen ("" = none). */
    @Volatile var engineError = ""

    override fun onCreate() {
        super.onCreate()
        prefs = Prefs(this)
        engine = Engine(this)
    }
}
