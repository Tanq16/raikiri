package com.tanq16.raikiri

import android.content.Context

object Prefs {
    private const val FILE = "raikiri_prefs"
    private const val KEY_SERVER_URL = "server_url"

    fun getServerUrl(context: Context): String =
        context.getSharedPreferences(FILE, Context.MODE_PRIVATE)
            .getString(KEY_SERVER_URL, "") ?: ""

    fun setServerUrl(context: Context, url: String) {
        context.getSharedPreferences(FILE, Context.MODE_PRIVATE)
            .edit().putString(KEY_SERVER_URL, url).apply()
    }
}
