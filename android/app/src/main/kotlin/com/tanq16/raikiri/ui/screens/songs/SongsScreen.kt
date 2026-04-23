package com.tanq16.raikiri.ui.screens.songs

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tanq16.raikiri.ui.MusicViewModel
import com.tanq16.raikiri.ui.PlayerViewModel
import com.tanq16.raikiri.ui.components.SearchBar
import com.tanq16.raikiri.ui.components.TrackItem

@Composable
fun SongsScreen(
    musicVm: MusicViewModel,
    playerVm: PlayerViewModel,
    serverUrl: String
) {
    val uiState by musicVm.allSongsState.collectAsStateWithLifecycle()
    val currentTrack by playerVm.currentTrack.collectAsStateWithLifecycle()
    var query by rememberSaveable { mutableStateOf("") }

    LaunchedEffect(Unit) {
        if (uiState is MusicViewModel.UiState.Loading) {
            musicVm.loadAllSongs()
        }
    }

    Column(Modifier.fillMaxSize()) {
        SearchBar(
            query = query,
            onQueryChange = { query = it },
            placeholder = "Search songs...",
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp)
        )

        when (val state = uiState) {
            is MusicViewModel.UiState.Loading -> {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(color = MaterialTheme.colorScheme.primary)
                }
            }

            is MusicViewModel.UiState.Error -> {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    TextButton(onClick = { musicVm.loadAllSongs() }) {
                        Text("Failed to load. Tap to retry.", color = MaterialTheme.colorScheme.error)
                    }
                }
            }

            is MusicViewModel.UiState.Success -> {
                val filtered = if (query.length >= 2) {
                    val q = query.lowercase()
                    state.items.filter {
                        it.name.lowercase().contains(q) || it.path.lowercase().contains(q)
                    }
                } else {
                    state.items
                }

                LazyColumn(Modifier.fillMaxSize()) {
                    itemsIndexed(
                        items = filtered,
                        key = { _, item -> item.path }
                    ) { index, item ->
                        TrackItem(
                            item = item,
                            serverUrl = serverUrl,
                            isPlaying = currentTrack?.path == item.path,
                            onClick = { playerVm.playTracks(filtered, index, serverUrl) }
                        )
                    }
                }
            }
        }
    }
}
