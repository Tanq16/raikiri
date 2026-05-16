package com.tanq16.raikiri.data.repository

import android.net.Uri
import com.tanq16.raikiri.data.api.FileEntry
import com.tanq16.raikiri.data.api.RaikiriApi

class MusicRepository(
    private val api: RaikiriApi?,
    val serverUrl: String
) {
    private var allSongs: List<FileEntry>? = null

    suspend fun listFolder(path: String): Result<List<FileEntry>> = runCatching {
        val api = api ?: throw IllegalStateException("Server not configured")
        api.list(path = path, mode = "music", recursive = false)
            .filter { it.type == "folder" || it.type == "audio" }
    }

    suspend fun listSongs(path: String, recursive: Boolean): Result<List<FileEntry>> = runCatching {
        fetchSongs(path, recursive)
    }

    suspend fun getAllSongs(): Result<List<FileEntry>> = runCatching {
        allSongs?.let { return@runCatching it }
        val songs = fetchSongs(path = "", recursive = true)
        allSongs = songs
        songs
    }

    fun clearCache() {
        allSongs = null
    }

    private suspend fun fetchSongs(path: String, recursive: Boolean): List<FileEntry> {
        val api = api ?: throw IllegalStateException("Server not configured")
        return api.list(path = path, mode = "music", recursive = recursive)
            .filter { it.type == "audio" }
            .sortedBy { it.name.lowercase() }
    }

    companion object {
        fun contentUrl(serverUrl: String, path: String): String {
            val encoded = path.split("/").joinToString("/") {
                Uri.encode(it)
            }
            return "${serverUrl.trimEnd('/')}/content/$encoded?mode=music"
        }

        fun thumbUrl(serverUrl: String, thumbPath: String): String {
            if (thumbPath.isBlank()) return ""
            return contentUrl(serverUrl, thumbPath)
        }
    }
}
