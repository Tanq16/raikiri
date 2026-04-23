package com.tanq16.raikiri

import android.app.Application
import com.tanq16.raikiri.data.api.RaikiriApi
import com.tanq16.raikiri.data.repository.MusicRepository
import com.tanq16.raikiri.playback.PlaybackConnection
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory
import java.util.concurrent.TimeUnit

class RaikiriApp : Application() {
    lateinit var playbackConnection: PlaybackConnection
    lateinit var repository: MusicRepository

    private var api: RaikiriApi? = null
    private var currentBaseUrl: String = ""

    override fun onCreate() {
        super.onCreate()
        playbackConnection = PlaybackConnection(this)
        val serverUrl = Prefs.getServerUrl(this)
        if (serverUrl.isNotBlank()) {
            repository = MusicRepository(getOrCreateApi(serverUrl), serverUrl)
        } else {
            repository = MusicRepository(null, "")
        }
    }

    fun getOrCreateApi(baseUrl: String): RaikiriApi {
        if (api != null && currentBaseUrl == baseUrl) return api!!

        val okhttp = OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .build()

        val json = Json { ignoreUnknownKeys = true }
        val contentType = "application/json".toMediaType()

        val retrofit = Retrofit.Builder()
            .baseUrl(baseUrl.trimEnd('/') + "/")
            .client(okhttp)
            .addConverterFactory(json.asConverterFactory(contentType))
            .build()

        api = retrofit.create(RaikiriApi::class.java)
        currentBaseUrl = baseUrl
        return api!!
    }

    fun updateServerUrl(url: String) {
        Prefs.setServerUrl(this, url)
        val newApi = if (url.isNotBlank()) getOrCreateApi(url) else null
        repository = MusicRepository(newApi, url)
    }
}
