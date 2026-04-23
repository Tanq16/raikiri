package com.tanq16.raikiri.ui.navigation

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.tanq16.raikiri.ui.MusicViewModel
import com.tanq16.raikiri.ui.PlayerViewModel
import com.tanq16.raikiri.ui.screens.artists.AlbumDetailScreen
import com.tanq16.raikiri.ui.screens.artists.ArtistDetailScreen
import com.tanq16.raikiri.ui.screens.artists.ArtistsScreen
import com.tanq16.raikiri.ui.screens.player.NowPlayingScreen
import com.tanq16.raikiri.ui.screens.settings.SettingsScreen
import com.tanq16.raikiri.ui.screens.songs.SongsScreen
import kotlinx.serialization.Serializable

@Serializable data object SongsRoute
@Serializable data object ArtistsRoute
@Serializable data class ArtistDetailRoute(val path: String, val name: String)
@Serializable data class AlbumDetailRoute(val path: String, val name: String, val artist: String)
@Serializable data object NowPlayingRoute
@Serializable data object SettingsRoute

@Composable
fun AppNavigation(
    navController: NavHostController,
    musicVm: MusicViewModel,
    playerVm: PlayerViewModel,
    serverUrl: String,
    modifier: Modifier = Modifier
) {
    NavHost(
        navController = navController,
        startDestination = SongsRoute,
        modifier = modifier
    ) {
        composable<SongsRoute> {
            SongsScreen(musicVm, playerVm, serverUrl)
        }
        composable<ArtistsRoute> {
            ArtistsScreen(musicVm, serverUrl, navController)
        }
        composable<ArtistDetailRoute> { backStackEntry ->
            val route = backStackEntry.toRoute<ArtistDetailRoute>()
            ArtistDetailScreen(route.path, route.name, musicVm, playerVm, serverUrl, navController)
        }
        composable<AlbumDetailRoute> { backStackEntry ->
            val route = backStackEntry.toRoute<AlbumDetailRoute>()
            AlbumDetailScreen(route.path, route.name, route.artist, musicVm, playerVm, serverUrl)
        }
        composable<NowPlayingRoute> {
            NowPlayingScreen(playerVm, serverUrl)
        }
        composable<SettingsRoute> {
            SettingsScreen(musicVm)
        }
    }
}
