package com.tanq16.raikiri.ui.screens.artists

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.tanq16.raikiri.data.api.FileEntry
import com.tanq16.raikiri.ui.MusicViewModel
import com.tanq16.raikiri.ui.PlayerViewModel
import com.tanq16.raikiri.ui.components.FolderItem
import com.tanq16.raikiri.ui.components.TrackItem
import com.tanq16.raikiri.ui.navigation.AlbumDetailRoute

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ArtistDetailScreen(
    path: String,
    name: String,
    musicVm: MusicViewModel,
    playerVm: PlayerViewModel,
    serverUrl: String,
    navController: NavController
) {
    var items by remember { mutableStateOf<List<FileEntry>?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    val currentTrack by playerVm.currentTrack.collectAsStateWithLifecycle()

    LaunchedEffect(path) {
        musicVm.repository.listFolder(path)
            .onSuccess { items = it }
            .onFailure { error = it.message }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(name) },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        },
        containerColor = MaterialTheme.colorScheme.background
    ) { padding ->
        when {
            error != null -> {
                Box(
                    Modifier.fillMaxSize().padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    TextButton(onClick = {
                        error = null
                        musicVm.loadFolder(path)
                    }) {
                        Text("Failed to load. Tap to retry.", color = MaterialTheme.colorScheme.error)
                    }
                }
            }

            items == null -> {
                Box(
                    Modifier.fillMaxSize().padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    CircularProgressIndicator(color = MaterialTheme.colorScheme.primary)
                }
            }

            else -> {
                val folders = items!!.filter { it.type == "folder" }
                val tracks = items!!.filter { it.type == "audio" }

                LazyColumn(Modifier.fillMaxSize().padding(padding)) {
                    items(folders, key = { it.path }) { item ->
                        FolderItem(
                            item = item,
                            label = "Album",
                            serverUrl = serverUrl,
                            onClick = {
                                navController.navigate(
                                    AlbumDetailRoute(path = item.path, name = item.name, artist = name)
                                )
                            }
                        )
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
