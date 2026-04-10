@file:OptIn(UnstableApi::class)

package dev.tanq16.raikiri

import android.content.Intent
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.os.PowerManager
import androidx.media3.common.util.UnstableApi
import androidx.media3.session.DefaultMediaNotificationProvider
import androidx.media3.session.MediaSession
import androidx.media3.session.MediaSessionService

class PlaybackService : MediaSessionService() {
    private var mediaSession: MediaSession? = null
    private lateinit var audioManager: AudioManager
    private var audioFocusRequest: AudioFocusRequest? = null
    private var wakeLock: PowerManager.WakeLock? = null
    private var pausedByFocusLoss = false

    companion object {
        var instance: PlaybackService? = null
        const val WAKELOCK_TIMEOUT_MS = 4L * 60 * 60 * 1000
    }

    private val audioFocusListener = AudioManager.OnAudioFocusChangeListener { focusChange ->
        val player = mediaSession?.player as? WebViewPlayer ?: return@OnAudioFocusChangeListener
        when (focusChange) {
            AudioManager.AUDIOFOCUS_LOSS -> {
                pausedByFocusLoss = false
                player.handleSetPlayWhenReady(false)
            }
            AudioManager.AUDIOFOCUS_LOSS_TRANSIENT -> {
                if (player.playWhenReady) {
                    pausedByFocusLoss = true
                    player.handleSetPlayWhenReady(false)
                }
            }
            AudioManager.AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK -> {
                sendJsCommand("document.querySelector('audio').volume=0.2")
            }
            AudioManager.AUDIOFOCUS_GAIN -> {
                sendJsCommand("document.querySelector('audio').volume=1.0")
                if (pausedByFocusLoss) {
                    pausedByFocusLoss = false
                    player.handleSetPlayWhenReady(true)
                }
            }
        }
    }

    override fun onCreate() {
        super.onCreate()
        instance = this

        val player = WebViewPlayer(this)
        mediaSession = MediaSession.Builder(this, player).build()

        setMediaNotificationProvider(
            DefaultMediaNotificationProvider.Builder(this)
                .setChannelId("raikiri_media")
                .setChannelName(R.string.media_channel_name)
                .build()
                .also { it.setSmallIcon(R.drawable.ic_launcher_monochrome) }
        )

        audioManager = getSystemService(AUDIO_SERVICE) as AudioManager
        requestAudioFocus()
        acquireWakeLock()
    }

    override fun onGetSession(controllerInfo: MediaSession.ControllerInfo): MediaSession? {
        return mediaSession
    }

    override fun onTaskRemoved(rootIntent: Intent?) {
        pauseAllPlayersAndStopSelf()
    }

    override fun onDestroy() {
        instance = null
        mediaSession?.run {
            player.release()
            release()
        }
        mediaSession = null
        audioFocusRequest?.let { audioManager.abandonAudioFocusRequest(it) }
        releaseWakeLock()
        super.onDestroy()
    }

    fun getPlayer(): WebViewPlayer? = mediaSession?.player as? WebViewPlayer

    private fun sendJsCommand(command: String) {
        val intent = Intent("dev.tanq16.raikiri.JS_COMMAND")
        intent.putExtra("command", command)
        intent.setPackage(packageName)
        sendBroadcast(intent)
    }

    private fun requestAudioFocus() {
        audioFocusRequest = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
            .setAudioAttributes(
                AudioAttributes.Builder()
                    .setUsage(AudioAttributes.USAGE_MEDIA)
                    .setContentType(AudioAttributes.CONTENT_TYPE_MUSIC)
                    .build()
            )
            .setOnAudioFocusChangeListener(audioFocusListener)
            .build()
        audioManager.requestAudioFocus(audioFocusRequest!!)
    }

    private fun acquireWakeLock() {
        if (wakeLock == null || !wakeLock!!.isHeld) {
            wakeLock = (getSystemService(POWER_SERVICE) as PowerManager)
                .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "raikiri::media")
                .apply {
                    setReferenceCounted(false)
                    acquire(WAKELOCK_TIMEOUT_MS)
                }
        }
    }

    private fun releaseWakeLock() {
        wakeLock?.let { if (it.isHeld) it.release() }
        wakeLock = null
    }
}
