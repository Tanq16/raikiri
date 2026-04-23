package com.tanq16.raikiri.ui.screens.artists

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tanq16.raikiri.data.api.FileEntry
import com.tanq16.raikiri.ui.MusicViewModel
import com.tanq16.raikiri.ui.PlayerViewModel
import com.tanq16.raikiri.ui.components.AlbumArtImage
import com.tanq16.raikiri.ui.components.TrackItem

@Composable
fun AlbumDetailScreen(
    path: String,
    name: String,
    artist: String,
    musicVm: MusicViewModel,
    playerVm: PlayerViewModel,
    serverUrl: String
) {
    var items by remember { mutableStateOf<List<FileEntry>?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    val currentTrack by playerVm.currentTrack.collectAsStateWithLifecycle()

    LaunchedEffect(path) {
        musicVm.repository.listFolder(path)
            .onSuccess { items = it.filter { e -> e.type == "audio" } }
            .onFailure { error = it.message }
    }

    Column(Modifier.fillMaxSize()) {
        when {
            error != null -> {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    TextButton(onClick = { error = null }) {
                        Text("Failed to load. Tap to retry.", color = MaterialTheme.colorScheme.error)
                    }
                }
            }

            items == null -> {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(color = MaterialTheme.colorScheme.primary)
                }
            }

            else -> {
                val tracks = items!!
                val thumbPath = tracks.firstOrNull()?.thumb ?: ""

                LazyColumn(Modifier.fillMaxSize()) {
                    item {
                        Column(
                            modifier = Modifier.fillMaxWidth().padding(16.dp),
                            horizontalAlignment = Alignment.CenterHorizontally
                        ) {
                            AlbumArtImage(
                                thumbPath = thumbPath,
                                serverUrl = serverUrl,
                                size = 200.dp
                            )
                            Spacer(Modifier.height(12.dp))
                            Text(
                                text = name,
                                style = MaterialTheme.typography.headlineMedium,
                                color = MaterialTheme.colorScheme.onBackground
                            )
                            Text(
                                text = artist,
                                style = MaterialTheme.typography.bodyLarge,
                                color = MaterialTheme.colorScheme.primary
                            )
                            Spacer(Modifier.height(8.dp))
                        }
                    }
                    itemsIndexed(tracks, key = { _, item -> item.path }) { index, item ->
                        TrackItem(
                            item = item,
                            serverUrl = serverUrl,
                            isPlaying = currentTrack?.path == item.path,
                            onClick = { playerVm.playTracks(tracks, index, serverUrl) }
                        )
                    }
                }
            }
        }
    }
}
