package com.nihars.arthik

import android.app.Activity
import android.app.KeyguardManager
import android.content.Context
import android.content.Intent
import android.hardware.biometrics.BiometricManager
import android.hardware.biometrics.BiometricManager.Authenticators.BIOMETRIC_STRONG
import android.hardware.biometrics.BiometricManager.Authenticators.BIOMETRIC_WEAK
import android.hardware.biometrics.BiometricManager.Authenticators.DEVICE_CREDENTIAL
import android.hardware.biometrics.BiometricPrompt
import android.os.Build
import android.os.CancellationSignal

/** Fingerprint / face unlock and the phone's own screen lock, through the system prompts. */
object Lock {
    /** True when the phone has an enrolled biometric the app can use (Android 10+). */
    fun biometricAvailable(ctx: Context): Boolean {
        if (Build.VERSION.SDK_INT < 29) return false
        val bm = ctx.getSystemService(BiometricManager::class.java) ?: return false
        return if (Build.VERSION.SDK_INT >= 30) {
            bm.canAuthenticate(BIOMETRIC_STRONG or BIOMETRIC_WEAK) == BiometricManager.BIOMETRIC_SUCCESS
        } else {
            @Suppress("DEPRECATION")
            bm.canAuthenticate() == BiometricManager.BIOMETRIC_SUCCESS
        }
    }

    /** True when the phone has a screen lock (PIN, pattern, password) to fall back on. */
    fun deviceSecure(ctx: Context): Boolean = ctx.getSystemService(KeyguardManager::class.java)?.isDeviceSecure == true

    /** Fingerprint prompt on top of the Arthik PIN; result(false) = use the PIN. */
    fun prompt(activity: Activity, result: (Boolean) -> Unit) {
        if (Build.VERSION.SDK_INT < 29) return result(false)
        val run = activity.mainExecutor
        val prompt = BiometricPrompt.Builder(activity)
            .setTitle("Unlock Arthik")
            .setNegativeButton("Use PIN", run) { _, _ -> result(false) }
            .build()
        prompt.authenticate(CancellationSignal(), run, callback(result))
    }

    /**
     * Device lock: fingerprint or face, with the phone's PIN/pattern as backup.
     * Android 8–9 have no such prompt, so they get the screen-lock check as an
     * activity ([confirmIntent], result in onActivityResult).
     */
    fun promptDevice(activity: Activity, result: (Boolean) -> Unit): Boolean {
        if (Build.VERSION.SDK_INT < 29) return false
        val run = activity.mainExecutor
        val b = BiometricPrompt.Builder(activity).setTitle("Unlock Arthik").setSubtitle("Fingerprint, face or your phone's screen lock")
        if (Build.VERSION.SDK_INT >= 30) b.setAllowedAuthenticators(BIOMETRIC_WEAK or DEVICE_CREDENTIAL)
        else @Suppress("DEPRECATION") b.setDeviceCredentialAllowed(true)
        b.build().authenticate(CancellationSignal(), run, callback(result))
        return true
    }

    /** Screen-lock check for Android 8–9 (null when the phone has no screen lock). */
    @Suppress("DEPRECATION")
    fun confirmIntent(ctx: Context): Intent? =
        ctx.getSystemService(KeyguardManager::class.java)?.createConfirmDeviceCredentialIntent("Unlock Arthik", null)

    private fun callback(result: (Boolean) -> Unit) = object : BiometricPrompt.AuthenticationCallback() {
        override fun onAuthenticationSucceeded(r: BiometricPrompt.AuthenticationResult?) = result(true)
        override fun onAuthenticationError(code: Int, msg: CharSequence?) = result(false)
    }
}
