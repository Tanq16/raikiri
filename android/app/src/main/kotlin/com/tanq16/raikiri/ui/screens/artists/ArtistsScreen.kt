package com.tanq16.raikiri.ui.screens.artists

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.filled.GridView
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
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
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.tanq16.raikiri.ui.MusicViewModel
import com.tanq16.raikiri.ui.components.FolderGridItem
import com.tanq16.raikiri.ui.components.FolderListItem
import com.tanq16.raikiri.ui.navigation.ArtistDetailRoute

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ArtistsScreen(
    musicVm: MusicViewModel,
    serverUrl: String,
    navController: NavController
) {
    val uiState by musicVm.folderState.collectAsStateWithLifecycle()
    var isGrid by rememberSaveable { mutableStateOf(true) }

    LaunchedEffect(Unit) {
        musicVm.loadFolder("")
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Artists") },
                actions = {
                    IconButton(onClick = { isGrid = !isGrid }) {
                        Icon(
                            imageVector = if (isGrid) Icons.AutoMirrored.Filled.List else Icons.Default.GridView,
                            contentDescription = if (isGrid) "List view" else "Grid view",
                            tint = MaterialTheme.colorScheme.onBackground
                        )
                    }
                },
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
                        Text("Failed to load. Tap to retry.", color = MaterialTheme.colorScheme.error)
                    }
                }
            }

            is MusicViewModel.UiState.Success -> {
                val folders = state.items.filter { it.type == "folder" }

                if (isGrid) {
                    LazyVerticalGrid(
                        columns = GridCells.Adaptive(minSize = 150.dp),
                        modifier = Modifier.fillMaxSize().padding(padding),
                        contentPadding = PaddingValues(16.dp),
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                        verticalArrangement = Arrangement.spacedBy(16.dp)
                    ) {
                        items(folders, key = { it.path }) { item ->
                            FolderGridItem(
                                item = item,
                                serverUrl = serverUrl,
                                onClick = {
                                    navController.navigate(
                                        ArtistDetailRoute(path = item.path, name = item.name)
                                    )
                                }
                            )
                        }
                    }
                } else {
                    LazyColumn(
                        Modifier.fillMaxSize().padding(padding)
                    ) {
                        items(folders, key = { it.path }) { item ->
                            FolderListItem(
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
}
