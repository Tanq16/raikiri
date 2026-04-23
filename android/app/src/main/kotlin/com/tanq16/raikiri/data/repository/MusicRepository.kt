package com.tanq16.raikiri.data.repository

import com.tanq16.raikiri.data.api.FileEntry
import com.tanq16.raikiri.data.api.RaikiriApi
import java.net.URLEncoder

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

    suspend fun getAllSongs(): Result<List<FileEntry>> = runCatching {
        val api = api ?: throw IllegalStateException("Server not configured")
        allSongs?.let { return@runCatching it }
        val songs = api.list(path = "", mode = "music", recursive = true)
            .filter { it.type == "audio" }
            .sortedBy { it.name.lowercase() }
        allSongs = songs
        songs
    }

    fun getCachedSongs(): List<FileEntry> = allSongs ?: emptyList()

    fun clearCache() {
        allSongs = null
    }

    fun search(query: String): List<FileEntry> {
        val q = query.lowercase()
        return getCachedSongs().filter { song ->
            song.name.lowercase().contains(q) ||
            song.path.lowercase().contains(q)
        }
    }

    companion object {
        fun contentUrl(serverUrl: String, path: String): String {
            val encoded = path.split("/").joinToString("/") {
                URLEncoder.encode(it, "UTF-8")
            }
            return "${serverUrl.trimEnd('/')}/content/$encoded?mode=music"
        }

        fun thumbUrl(serverUrl: String, thumbPath: String): String {
            if (thumbPath.isBlank()) return ""
            return contentUrl(serverUrl, thumbPath)
        }
    }
}
