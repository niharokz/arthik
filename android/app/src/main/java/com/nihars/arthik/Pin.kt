package com.nihars.arthik

import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64
import javax.crypto.SecretKeyFactory
import javax.crypto.spec.PBEKeySpec

/** PIN hashing (PBKDF2-SHA256 with a random salt). Pure JVM code, unit tested. */
object Pin {
    private const val ITERATIONS = 60_000 // ~100 ms on a phone
    private const val BITS = 256

    /** True for 4–8 digits. */
    fun valid(pin: String): Boolean = pin.length in 4..8 && pin.all { it in '0'..'9' }

    /** Returns "v1:iterations:salt:hash" for storage. */
    fun hash(pin: String, salt: ByteArray = randomSalt()): String {
        val enc = Base64.getEncoder()
        return "v1:$ITERATIONS:${enc.encodeToString(salt)}:${enc.encodeToString(derive(pin, salt, ITERATIONS))}"
    }

    /** Checks a PIN against a stored hash (constant time). */
    fun verify(pin: String, stored: String): Boolean {
        val parts = stored.split(":")
        if (parts.size != 4 || parts[0] != "v1") return false
        return try {
            val dec = Base64.getDecoder()
            val iterations = parts[1].toInt()
            val expected = dec.decode(parts[3])
            MessageDigest.isEqual(expected, derive(pin, dec.decode(parts[2]), iterations))
        } catch (e: IllegalArgumentException) {
            false
        }
    }

    private fun derive(pin: String, salt: ByteArray, iterations: Int): ByteArray {
        val spec = PBEKeySpec(pin.toCharArray(), salt, iterations, BITS)
        try {
            return SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256").generateSecret(spec).encoded
        } finally {
            spec.clearPassword()
        }
    }

    private fun randomSalt(): ByteArray = ByteArray(16).also { SecureRandom().nextBytes(it) }
}
