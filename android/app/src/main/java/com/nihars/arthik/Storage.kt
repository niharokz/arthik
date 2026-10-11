package com.nihars.arthik

import android.Manifest
import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.DocumentsContract
import android.provider.Settings
import java.io.File

/** Access to the shared-storage folder that Syncthing keeps in sync. */
object Storage {
    /** True when the app may read and write files by path. */
    fun hasAccess(ctx: Context): Boolean =
        if (Build.VERSION.SDK_INT >= 30) Environment.isExternalStorageManager()
        else ctx.checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED

    /** Asks for access: "All files access" screen on Android 11+, a permission dialog before. */
    fun requestAccess(activity: Activity, requestCode: Int) {
        if (Build.VERSION.SDK_INT >= 30) {
            val pkg = Uri.parse("package:" + activity.packageName)
            try {
                activity.startActivityForResult(Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION, pkg), requestCode)
            } catch (e: ActivityNotFoundException) { // some phones only have the general list
                activity.startActivityForResult(Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION), requestCode)
            }
        } else {
            activity.requestPermissions(arrayOf(Manifest.permission.READ_EXTERNAL_STORAGE, Manifest.permission.WRITE_EXTERNAL_STORAGE), requestCode)
        }
    }

    /** System folder picker. */
    fun pickerIntent(): Intent = Intent(Intent.ACTION_OPEN_DOCUMENT_TREE)
        .putExtra("android.content.extra.SHOW_ADVANCED", true) // show internal storage

    /** Real path for a picked folder, or null if the picker returned a non-folder provider. */
    fun pathOf(treeUri: Uri): String? = try {
        Paths.fromTreeDocId(DocumentsContract.getTreeDocumentId(treeUri), Environment.getExternalStorageDirectory().absolutePath)
    } catch (e: IllegalArgumentException) {
        null
    }

    /** Quick check used for display: the folder exists and is writable. */
    fun looksUsable(path: String): Boolean = path.isNotEmpty() && File(path).let { it.isDirectory && it.canWrite() }

    /** Full check before using a folder; returns an error message or null when fine. */
    fun check(ctx: Context, path: String?): String? {
        val p = path?.let { Paths.clean(it) } ?: return "That isn't a folder on this phone. Choose a folder under Internal storage."
        if (!hasAccess(ctx)) return "Arthik needs file access to use this folder. Allow it, then choose the folder again."
        val dir = File(p)
        if (!dir.exists() && !dir.mkdirs()) return "The folder $p doesn't exist and can't be created."
        if (!dir.isDirectory) return "$p is a file, not a folder."
        val probe = File(dir, ".arthik-write-test")
        return try {
            probe.writeText("ok")
            probe.delete()
            null
        } catch (e: Exception) {
            "Arthik can't write to $p (${e.message})."
        }
    }
}
