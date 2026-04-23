package com.tanq16.raikiri.data.api

import retrofit2.http.GET
import retrofit2.http.Query

interface RaikiriApi {
    @GET("api/list")
    suspend fun list(
        @Query("path") path: String,
        @Query("mode") mode: String = "music",
        @Query("recursive") recursive: Boolean = false
    ): List<FileEntry>
}
