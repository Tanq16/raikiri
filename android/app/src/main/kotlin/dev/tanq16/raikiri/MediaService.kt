package dev.tanq16.raikiri

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.os.Binder
import android.os.IBinder
import android.os.PowerManager
import android.support.v4.media.MediaMetadataCompat
import android.support.v4.media.session.MediaSessionCompat
import android.support.v4.media.session.PlaybackStateCompat
import androidx.core.app.NotificationCompat
import androidx.media.app.NotificationCompat as MediaNotificationCompat

class MediaService : Service() {
    companion object {
        const val CHANNEL_ID = "raikiri_media"
        const val NOTIFICATION_ID = 1
    }

    private lateinit var mediaSession: MediaSessionCompat
    private lateinit var audioManager: AudioManager
    private var audioFocusRequest: AudioFocusRequest? = null
    private var wakeLock: PowerManager.WakeLock? = null
    private val binder = LocalBinder()

    private var currentTitle = "Raikiri"
    private var currentArtist = "Media"
    private var isPlaying = false

    inner class LocalBinder : Binder() {
        fun getService(): MediaService = this@MediaService
    }

    override fun onBind(intent: Intent?): IBinder = binder

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()

        mediaSession = MediaSessionCompat(this, "RaikiriMedia").apply {
            setCallback(object : MediaSessionCompat.Callback() {
                override fun onPlay() { sendJsCommand("Player.play()") }
                override fun onPause() { sendJsCommand("Player.pause()") }
                override fun onSkipToNext() { sendJsCommand("Player.next()") }
                override fun onSkipToPrevious() { sendJsCommand("Player.prev()") }
            })
            isActive = true
        }

        audioManager = getSystemService(AUDIO_SERVICE) as AudioManager
        requestAudioFocus()

        wakeLock = (getSystemService(POWER_SERVICE) as PowerManager)
            .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "raikiri::media")
            .apply { acquire() }

        startForeground(NOTIFICATION_ID, buildNotification())
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        return START_STICKY
    }

    fun updateMetadata(title: String, artist: String, album: String) {
        currentTitle = title
        currentArtist = artist
        mediaSession.setMetadata(
            MediaMetadataCompat.Builder()
                .putString(MediaMetadataCompat.METADATA_KEY_TITLE, title)
                .putString(MediaMetadataCompat.METADATA_KEY_ARTIST, artist)
                .putString(MediaMetadataCompat.METADATA_KEY_ALBUM, album)
                .build()
        )
        refreshNotification()
    }

    fun updatePlaybackState(playing: Boolean) {
        isPlaying = playing
        val state = if (playing) PlaybackStateCompat.STATE_PLAYING else PlaybackStateCompat.STATE_PAUSED
        mediaSession.setPlaybackState(
            PlaybackStateCompat.Builder()
                .setState(state, PlaybackStateCompat.PLAYBACK_POSITION_UNKNOWN, if (playing) 1f else 0f)
                .setActions(
                    PlaybackStateCompat.ACTION_PLAY or
                    PlaybackStateCompat.ACTION_PAUSE or
                    PlaybackStateCompat.ACTION_SKIP_TO_NEXT or
                    PlaybackStateCompat.ACTION_SKIP_TO_PREVIOUS
                )
                .build()
        )
        refreshNotification()
    }

    private fun sendJsCommand(command: String) {
        val intent = Intent("dev.tanq16.raikiri.JS_COMMAND")
        intent.putExtra("command", command)
        intent.setPackage(packageName)
        sendBroadcast(intent)
    }

    private fun refreshNotification() {
        val nm = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        nm.notify(NOTIFICATION_ID, buildNotification())
    }

    private fun buildNotification(): Notification {
        val launchIntent = packageManager.getLaunchIntentForPackage(packageName)
        val pendingIntent = PendingIntent.getActivity(
            this, 0, launchIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(currentTitle)
            .setContentText(currentArtist)
            .setSmallIcon(android.R.drawable.ic_media_play)
            .setContentIntent(pendingIntent)
            .setStyle(
                MediaNotificationCompat.MediaStyle()
                    .setMediaSession(mediaSession.sessionToken)
            )
            .setOngoing(isPlaying)
            .setSilent(true)
            .build()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID, "Media Playback",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Raikiri media playback controls"
            setShowBadge(false)
        }
        (getSystemService(NOTIFICATION_SERVICE) as NotificationManager)
            .createNotificationChannel(channel)
    }

    private fun requestAudioFocus() {
        audioFocusRequest = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
            .setAudioAttributes(
                AudioAttributes.Builder()
                    .setUsage(AudioAttributes.USAGE_MEDIA)
                    .setContentType(AudioAttributes.CONTENT_TYPE_MUSIC)
                    .build()
            )
            .build()
        audioManager.requestAudioFocus(audioFocusRequest!!)
    }

    override fun onDestroy() {
        mediaSession.release()
        audioFocusRequest?.let { audioManager.abandonAudioFocusRequest(it) }
        wakeLock?.let { if (it.isHeld) it.release() }
        super.onDestroy()
    }
}
