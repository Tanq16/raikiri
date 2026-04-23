package com.tanq16.raikiri.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.Player
import androidx.media3.session.MediaController
import com.tanq16.raikiri.data.api.FileEntry
import com.tanq16.raikiri.data.repository.MusicRepository
import com.tanq16.raikiri.playback.PlaybackConnection
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

class PlayerViewModel(private val connection: PlaybackConnection) : ViewModel() {

    val controller: StateFlow<MediaController?> = connection.controller

    private val _currentTrack = MutableStateFlow<FileEntry?>(null)
    val currentTrack: StateFlow<FileEntry?> = _currentTrack.asStateFlow()

    private val _isPlaying = MutableStateFlow(false)
    val isPlaying: StateFlow<Boolean> = _isPlaying.asStateFlow()

    private val _positionMs = MutableStateFlow(0L)
    val positionMs: StateFlow<Long> = _positionMs.asStateFlow()

    private val _durationMs = MutableStateFlow(0L)
    val durationMs: StateFlow<Long> = _durationMs.asStateFlow()

    private val _queue = MutableStateFlow<List<FileEntry>>(emptyList())
    val queue: StateFlow<List<FileEntry>> = _queue.asStateFlow()

    private val _currentIndex = MutableStateFlow(-1)
    val currentIndex: StateFlow<Int> = _currentIndex.asStateFlow()

    private var positionJob: Job? = null
    private var listenerAttached = false

    fun attachListener() {
        val ctrl = controller.value ?: return
        if (listenerAttached) return
        listenerAttached = true

        ctrl.addListener(object : Player.Listener {
            override fun onIsPlayingChanged(playing: Boolean) {
                _isPlaying.value = playing
            }

            override fun onMediaItemTransition(mediaItem: MediaItem?, reason: Int) {
                _currentIndex.value = ctrl.currentMediaItemIndex
                updateCurrentTrack()
                _durationMs.value = ctrl.duration.coerceAtLeast(0)
            }

            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_READY) {
                    _durationMs.value = ctrl.duration.coerceAtLeast(0)
                }
            }
        })
        startPositionPolling()
    }

    private fun startPositionPolling() {
        positionJob?.cancel()
        positionJob = viewModelScope.launch {
            while (isActive) {
                controller.value?.let {
                    _positionMs.value = it.currentPosition
                }
                delay(250)
            }
        }
    }

    fun playTracks(tracks: List<FileEntry>, startIndex: Int, serverUrl: String) {
        val ctrl = controller.value ?: return
        _queue.value = tracks

        val mediaItems = tracks.map { track ->
            val uri = MusicRepository.contentUrl(serverUrl, track.path)
            val displayName = track.name.substringBeforeLast('.')
            val artistName = extractArtist(track.path)

            MediaItem.Builder()
                .setUri(uri)
                .setMediaId(track.path)
                .setMediaMetadata(
                    MediaMetadata.Builder()
                        .setTitle(displayName)
                        .setArtist(artistName)
                        .build()
                )
                .build()
        }

        ctrl.setMediaItems(mediaItems, startIndex, 0L)
        ctrl.prepare()
        ctrl.play()
        _currentIndex.value = startIndex
        updateCurrentTrack()
    }

    fun togglePlayPause() {
        controller.value?.let {
            if (it.isPlaying) it.pause() else it.play()
        }
    }

    fun next() {
        controller.value?.let {
            if (it.hasNextMediaItem()) it.seekToNextMediaItem()
        }
    }

    fun prev() {
        controller.value?.let {
            if (it.currentPosition > 3000) {
                it.seekTo(0)
            } else {
                it.seekToPreviousMediaItem()
            }
        }
    }

    fun seekTo(posMs: Long) {
        controller.value?.seekTo(posMs)
    }

    fun playIndex(index: Int) {
        controller.value?.seekToDefaultPosition(index)
    }

    private fun updateCurrentTrack() {
        val idx = _currentIndex.value
        _currentTrack.value = _queue.value.getOrNull(idx)
    }

    private fun extractArtist(path: String): String {
        // Path structure: Artist/Album/track.mp3 or Artist/track.mp3
        val parts = path.split("/")
        return if (parts.size >= 2) parts[0] else "Unknown Artist"
    }

    class Factory(private val connection: PlaybackConnection) : ViewModelProvider.Factory {
        @Suppress("UNCHECKED_CAST")
        override fun <T : ViewModel> create(modelClass: Class<T>): T =
            PlayerViewModel(connection) as T
    }
}
