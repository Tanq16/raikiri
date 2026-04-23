package com.tanq16.raikiri.ui.screens.artists

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.tanq16.raikiri.ui.MusicViewModel
import com.tanq16.raikiri.ui.components.FolderItem
import com.tanq16.raikiri.ui.navigation.ArtistDetailRoute

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ArtistsScreen(
    musicVm: MusicViewModel,
    serverUrl: String,
    navController: NavController
) {
    val uiState by musicVm.folderState.collectAsStateWithLifecycle()

    LaunchedEffect(Unit) {
        musicVm.loadFolder("")
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Artists") },
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
                    TextButton(onClick = { musicVm.loadFolder("") }) {
                        Text(
                            text = "Failed to load. Tap to retry.",
                            color = MaterialTheme.colorScheme.error
                        )
                    }
                }
            }

            is MusicViewModel.UiState.Success -> {
                val folders = state.items.filter { it.type == "folder" }
                LazyColumn(
                    Modifier.fillMaxSize().padding(padding)
                ) {
                    items(folders, key = { it.path }) { item ->
                        FolderItem(
                            item = item,
                            label = "Artist",
                            serverUrl = serverUrl,
                            onClick = {
                                navController.navigate(
                                    ArtistDetailRoute(path = item.path, name = item.name)
                                )
                            }
                        )
                    }
                }
            }
        }
    }
}
