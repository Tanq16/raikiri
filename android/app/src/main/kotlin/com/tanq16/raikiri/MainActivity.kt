package com.tanq16.raikiri

import android.Manifest
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.LibraryMusic
import androidx.compose.material.icons.filled.MusicNote
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.LibraryMusic
import androidx.compose.material.icons.outlined.MusicNote
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.tanq16.raikiri.ui.MusicViewModel
import com.tanq16.raikiri.ui.PlayerViewModel
import com.tanq16.raikiri.ui.components.MiniPlayer
import com.tanq16.raikiri.ui.navigation.AppNavigation
import com.tanq16.raikiri.ui.navigation.ArtistsRoute
import com.tanq16.raikiri.ui.navigation.NowPlayingRoute
import com.tanq16.raikiri.ui.navigation.SettingsRoute
import com.tanq16.raikiri.ui.navigation.SongsRoute
import com.tanq16.raikiri.ui.theme.RaikiriTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val app = application as RaikiriApp

        val musicVm: MusicViewModel by viewModels { MusicViewModel.Factory(app.repository) }
        val playerVm: PlayerViewModel by viewModels { PlayerViewModel.Factory(app.playbackConnection) }

        if (Build.VERSION.SDK_INT >= 33) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 0)
        }

        setContent {
            RaikiriTheme {
                val navController = rememberNavController()
                val serverUrl = Prefs.getServerUrl(this@MainActivity)
                val currentTrack by playerVm.currentTrack.collectAsStateWithLifecycle()
                val controller by playerVm.controller.collectAsStateWithLifecycle()

                LaunchedEffect(controller) {
                    if (controller != null) playerVm.attachListener()
                }

                Scaffold(
                    bottomBar = {
                        Column {
                            if (currentTrack != null) {
                                MiniPlayer(
                                    playerVm = playerVm,
                                    serverUrl = serverUrl,
                                    onTap = { navController.navigate(NowPlayingRoute) }
                                )
                            }
                            BottomNavBar(navController)
                        }
                    },
                    containerColor = MaterialTheme.colorScheme.background
                ) { padding ->
                    Box(Modifier.padding(padding)) {
                        AppNavigation(
                            navController = navController,
                            musicVm = musicVm,
                            playerVm = playerVm,
                            serverUrl = serverUrl
                        )
                    }
                }
            }
        }
    }
}

private data class NavItem(
    val route: Any,
    val label: String,
    val selectedIcon: ImageVector,
    val unselectedIcon: ImageVector
)

private val navItems = listOf(
    NavItem(SongsRoute, "Songs", Icons.Filled.MusicNote, Icons.Outlined.MusicNote),
    NavItem(ArtistsRoute, "Artists", Icons.Filled.LibraryMusic, Icons.Outlined.LibraryMusic),
    NavItem(SettingsRoute, "Settings", Icons.Filled.Settings, Icons.Outlined.Settings),
)

@Composable
private fun BottomNavBar(navController: NavController) {
    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val currentDestination = navBackStackEntry?.destination

    NavigationBar(
        containerColor = MaterialTheme.colorScheme.surface,
        contentColor = MaterialTheme.colorScheme.onSurface
    ) {
        navItems.forEach { item ->
            val selected = currentDestination?.hierarchy?.any {
                it.hasRoute(item.route::class)
            } == true

            NavigationBarItem(
                selected = selected,
                onClick = {
                    navController.navigate(item.route) {
                        popUpTo(navController.graph.findStartDestination().id) {
                            saveState = true
                        }
                        launchSingleTop = true
                        restoreState = true
                    }
                },
                icon = {
                    Icon(
                        imageVector = if (selected) item.selectedIcon else item.unselectedIcon,
                        contentDescription = item.label
                    )
                },
                label = { Text(item.label) },
                colors = NavigationBarItemDefaults.colors(
                    selectedIconColor = MaterialTheme.colorScheme.primary,
                    selectedTextColor = MaterialTheme.colorScheme.primary,
                    indicatorColor = MaterialTheme.colorScheme.primaryContainer,
                    unselectedIconColor = MaterialTheme.colorScheme.onSurfaceVariant,
                    unselectedTextColor = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            )
        }
    }
}
