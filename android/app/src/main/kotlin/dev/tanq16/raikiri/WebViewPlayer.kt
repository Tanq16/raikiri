@file:OptIn(UnstableApi::class)

package dev.tanq16.raikiri

import android.app.Service
import android.content.Intent
import android.os.Looper
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.Player
import androidx.media3.common.SimpleBasePlayer
import androidx.media3.common.util.UnstableApi
import com.google.common.util.concurrent.Futures
import com.google.common.util.concurrent.ListenableFuture

class WebViewPlayer(
    private val service: Service,
    looper: Looper = Looper.getMainLooper()
) : SimpleBasePlayer(looper) {

    private var playing = false
    private var playbackState = Player.STATE_IDLE
    private var title = "Raikiri"
    private var artist = "Media"
    private var album = ""

    override fun getState(): State {
        val commands = Player.Commands.Builder()
            .addAll(
                Player.COMMAND_PLAY_PAUSE,
                Player.COMMAND_SEEK_TO_NEXT,
                Player.COMMAND_SEEK_TO_PREVIOUS,
                Player.COMMAND_STOP
            )
            .build()

        val playlist = if (playbackState != Player.STATE_IDLE) {
            listOf(
                MediaItemData.Builder("current")
                    .setMediaItem(
                        MediaItem.Builder()
                            .setMediaId("current")
                            .setMediaMetadata(
                                MediaMetadata.Builder()
                                    .setTitle(title)
                                    .setArtist(artist)
                                    .setAlbumTitle(album)
                                    .build()
                            )
                            .build()
                    )
                    .build()
            )
        } else emptyList()

        return State.Builder()
            .setAvailableCommands(commands)
            .setPlayWhenReady(playing, Player.PLAY_WHEN_READY_CHANGE_REASON_USER_REQUEST)
            .setPlaybackState(playbackState)
            .setPlaylist(playlist)
            .build()
    }

    override fun handleSetPlayWhenReady(playWhenReady: Boolean): ListenableFuture<*> {
        sendJsCommand(if (playWhenReady) "Player.play()" else "Player.pause()")
        return Futures.immediateVoidFuture()
    }

    override fun handleSeek(
        mediaItemIndex: Int,
        positionMs: Long,
        seekCommand: Int
    ): ListenableFuture<*> {
        when (seekCommand) {
            Player.COMMAND_SEEK_TO_NEXT, Player.COMMAND_SEEK_TO_NEXT_MEDIA_ITEM ->
                sendJsCommand("Player.next()")
            Player.COMMAND_SEEK_TO_PREVIOUS, Player.COMMAND_SEEK_TO_PREVIOUS_MEDIA_ITEM ->
                sendJsCommand("Player.prev()")
        }
        return Futures.immediateVoidFuture()
    }

    override fun handleStop(): ListenableFuture<*> {
        playing = false
        playbackState = Player.STATE_IDLE
        return Futures.immediateVoidFuture()
    }

    override fun handleRelease(): ListenableFuture<*> {
        playing = false
        playbackState = Player.STATE_IDLE
        return Futures.immediateVoidFuture()
    }

    fun onJsPlaying() {
        playing = true
        playbackState = Player.STATE_READY
        invalidateState()
    }

    fun onJsPaused() {
        playing = false
        invalidateState()
    }

    fun onJsMetadata(newTitle: String, newArtist: String, newAlbum: String) {
        title = newTitle
        artist = newArtist
        album = newAlbum
        playbackState = Player.STATE_READY
        invalidateState()
    }

    fun onJsStopped() {
        playing = false
        playbackState = Player.STATE_IDLE
        title = "Raikiri"
        artist = "Media"
        album = ""
        invalidateState()
    }

    private fun sendJsCommand(command: String) {
        val intent = Intent("dev.tanq16.raikiri.JS_COMMAND")
        intent.putExtra("command", command)
        intent.setPackage(service.packageName)
        service.sendBroadcast(intent)
    }
}
