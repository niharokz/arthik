package com.nihars.arthik

import android.content.ContentValues
import android.content.Context
import android.os.Build
import android.os.Environment
import android.provider.MediaStore
import android.webkit.CookieManager
import android.webkit.URLUtil
import java.io.File
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

/** Saves files the web app offers for download (CSV exports) into Downloads. */
object Downloads {
    /** Downloads url with the session cookie; returns the saved file name. Call off the UI thread. */
    @Throws(IOException::class)
    fun save(ctx: Context, url: String, disposition: String?, mime: String?): String {
        val c = URL(url).openConnection() as HttpURLConnection
        CookieManager.getInstance().getCookie(url)?.let { c.setRequestProperty("Cookie", it) }
        if (c.responseCode != 200) throw IOException("download failed (HTTP ${c.responseCode})")
        val type = mime ?: c.contentType ?: "text/csv"
        val name = URLUtil.guessFileName(url, disposition ?: c.getHeaderField("Content-Disposition"), type)
        c.inputStream.use { input ->
            if (Build.VERSION.SDK_INT >= 29) { // MediaStore: no permission needed
                val values = ContentValues().apply {
                    put(MediaStore.Downloads.DISPLAY_NAME, name)
                    put(MediaStore.Downloads.MIME_TYPE, type.substringBefore(';'))
                    put(MediaStore.Downloads.IS_PENDING, 1)
                }
                val r = ctx.contentResolver
                val uri = r.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values) ?: throw IOException("can't create the file in Downloads")
                r.openOutputStream(uri)?.use { input.copyTo(it) } ?: throw IOException("can't write to Downloads")
                values.clear()
                values.put(MediaStore.Downloads.IS_PENDING, 0)
                r.update(uri, values, null, null)
            } else {
                @Suppress("DEPRECATION")
                val dir = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS)
                dir.mkdirs()
                File(dir, name).outputStream().use { input.copyTo(it) }
            }
        }
        c.disconnect()
        return name
    }
}
