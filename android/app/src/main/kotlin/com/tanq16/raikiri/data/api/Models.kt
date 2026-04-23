package com.tanq16.raikiri.data.api

import kotlinx.serialization.Serializable

@Serializable
data class FileEntry(
    val name: String,
    val path: String,
    val type: String,
    val size: String = "",
    val thumb: String = "",
    val modified: String = ""
)
