package com.tanq16.raikiri.ui.screens.songs

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.tanq16.raikiri.ui.MusicViewModel
import com.tanq16.raikiri.ui.PlayerViewModel
import com.tanq16.raikiri.ui.components.TrackItem

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SongsScreen(
    musicVm: MusicViewModel,
    playerVm: PlayerViewModel,
    serverUrl: String
) {
    val uiState by musicVm.allSongsState.collectAsStateWithLifecycle()
    val currentTrack by playerVm.currentTrack.collectAsStateWithLifecycle()

    LaunchedEffect(Unit) {
        if (uiState is MusicViewModel.UiState.Loading) {
            musicVm.loadAllSongs()
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Songs") },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        },
        containerColor = MaterialTheme.colorScheme.background
    ) { padding ->
        when (val state = uiState) {
            is MusicViewModel.UiState.Loading -> {
                Box(
                    Modifier.fillMaxSize().padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    CircularProgressIndicator(color = MaterialTheme.colorScheme.primary)
                }
            }

            is MusicViewModel.UiState.Error -> {
                Box(
                    Modifier.fillMaxSize().padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    TextButton(onClick = { musicVm.loadAllSongs() }) {
                        Text(
                            text = "Failed to load. Tap to retry.",
                            color = MaterialTheme.colorScheme.error
                        )
                    }
                }
            }

            is MusicViewModel.UiState.Success -> {
                val isRefreshing = false
                PullToRefreshBox(
                    isRefreshing = isRefreshing,
                    onRefresh = { musicVm.refresh() },
                    modifier = Modifier.fillMaxSize().padding(padding)
                ) {
                    LazyColumn(Modifier.fillMaxSize()) {
                        itemsIndexed(
                            items = state.items,
                            key = { _, item -> item.path }
                        ) { index, item ->
                            TrackItem(
                                item = item,
                                serverUrl = serverUrl,
                                isPlaying = currentTrack?.path == item.path,
                                onClick = {
                                    playerVm.playTracks(state.items, index, serverUrl)
                                }
                            )
                        }
                    }
                }
            }
        }
    }
}
