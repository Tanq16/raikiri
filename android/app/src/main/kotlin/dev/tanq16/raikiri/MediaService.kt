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
import android.os.Handler
import android.os.IBinder
import android.os.Looper
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
        const val ACTION_PLAY_PAUSE = "dev.tanq16.raikiri.PLAY_PAUSE"
        const val ACTION_NEXT = "dev.tanq16.raikiri.NEXT"
        const val ACTION_PREV = "dev.tanq16.raikiri.PREV"
        const val IDLE_TIMEOUT_MS = 15L * 60 * 1000
        const val WAKELOCK_TIMEOUT_MS = 4L * 60 * 60 * 1000
    }

    private lateinit var mediaSession: MediaSessionCompat
    private lateinit var audioManager: AudioManager
    private var audioFocusRequest: AudioFocusRequest? = null
    private var wakeLock: PowerManager.WakeLock? = null
    private val binder = LocalBinder()
    private val handler = Handler(Looper.getMainLooper())
    private val idleRunnable = Runnable { goIdle() }
    private var isIdle = false
    private var pausedByFocusLoss = false

    private var currentTitle = "Raikiri"
    private var currentArtist = "Media"
    private var isPlaying = false

    private val audioFocusListener = AudioManager.OnAudioFocusChangeListener { focusChange ->
        when (focusChange) {
            AudioManager.AUDIOFOCUS_LOSS -> {
                pausedByFocusLoss = false
                sendJsCommand("Player.pause()")
            }
            AudioManager.AUDIOFOCUS_LOSS_TRANSIENT -> {
                if (isPlaying) {
                    pausedByFocusLoss = true
                    sendJsCommand("Player.pause()")
                }
            }
            AudioManager.AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK -> {
                sendJsCommand("document.querySelector('audio').volume=0.2")
            }
            AudioManager.AUDIOFOCUS_GAIN -> {
                sendJsCommand("document.querySelector('audio').volume=1.0")
                if (pausedByFocusLoss) {
                    pausedByFocusLoss = false
                    sendJsCommand("Player.play()")
                }
            }
        }
    }

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

        mediaSession.setPlaybackState(
            PlaybackStateCompat.Builder()
                .setState(PlaybackStateCompat.STATE_PAUSED, PlaybackStateCompat.PLAYBACK_POSITION_UNKNOWN, 0f)
                .setActions(MEDIA_ACTIONS)
                .build()
        )

        audioManager = getSystemService(AUDIO_SERVICE) as AudioManager
        requestAudioFocus()
        acquireWakeLock()

        startForeground(NOTIFICATION_ID, buildNotification())
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_PLAY_PAUSE -> {
                if (isPlaying) sendJsCommand("Player.pause()")
                else sendJsCommand("Player.play()")
            }
            ACTION_NEXT -> sendJsCommand("Player.next()")
            ACTION_PREV -> sendJsCommand("Player.prev()")
        }
        return START_NOT_STICKY
    }

    fun isServiceIdle(): Boolean = isIdle

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
        if (!isIdle) refreshNotification()
    }

    fun updatePlaybackState(playing: Boolean) {
        isPlaying = playing
        handler.removeCallbacks(idleRunnable)

        if (playing && isIdle) {
            isIdle = false
            mediaSession.isActive = true
            acquireWakeLock()
            requestAudioFocus()
            startForeground(NOTIFICATION_ID, buildNotification())
        }

        if (playing) {
            acquireWakeLock()
        } else {
            releaseWakeLock()
        }

        val state = if (playing) PlaybackStateCompat.STATE_PLAYING else PlaybackStateCompat.STATE_PAUSED
        mediaSession.setPlaybackState(
            PlaybackStateCompat.Builder()
                .setState(state, PlaybackStateCompat.PLAYBACK_POSITION_UNKNOWN, if (playing) 1f else 0f)
                .setActions(MEDIA_ACTIONS)
                .build()
        )

        if (!isIdle) refreshNotification()
    }

    fun clearMedia() {
        isPlaying = false
        releaseWakeLock()
        handler.removeCallbacks(idleRunnable)
        handler.postDelayed(idleRunnable, IDLE_TIMEOUT_MS)
        mediaSession.setPlaybackState(
            PlaybackStateCompat.Builder()
                .setState(PlaybackStateCompat.STATE_STOPPED, PlaybackStateCompat.PLAYBACK_POSITION_UNKNOWN, 0f)
                .setActions(MEDIA_ACTIONS)
                .build()
        )
        refreshNotification()
    }

    private fun goIdle() {
        isIdle = true
        isPlaying = false
        releaseWakeLock()
        audioFocusRequest?.let { audioManager.abandonAudioFocusRequest(it) }
        mediaSession.isActive = false
        stopForeground(STOP_FOREGROUND_REMOVE)
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
        val contentPi = PendingIntent.getActivity(
            this, 0, launchIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val prevPi = PendingIntent.getService(this, 1,
            Intent(this, MediaService::class.java).apply { action = ACTION_PREV },
            PendingIntent.FLAG_IMMUTABLE)
        val playPausePi = PendingIntent.getService(this, 2,
            Intent(this, MediaService::class.java).apply { action = ACTION_PLAY_PAUSE },
            PendingIntent.FLAG_IMMUTABLE)
        val nextPi = PendingIntent.getService(this, 3,
            Intent(this, MediaService::class.java).apply { action = ACTION_NEXT },
            PendingIntent.FLAG_IMMUTABLE)

        val playPauseIcon = if (isPlaying) R.drawable.ic_media_pause else R.drawable.ic_media_play

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(currentTitle)
            .setContentText(currentArtist)
            .setSmallIcon(R.drawable.ic_launcher_monochrome)
            .setContentIntent(contentPi)
            .addAction(R.drawable.ic_media_previous, "Previous", prevPi)
            .addAction(playPauseIcon, if (isPlaying) "Pause" else "Play", playPausePi)
            .addAction(R.drawable.ic_media_next, "Next", nextPi)
            .setStyle(
                MediaNotificationCompat.MediaStyle()
                    .setMediaSession(mediaSession.sessionToken)
                    .setShowActionsInCompactView(0, 1, 2)
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
            .setOnAudioFocusChangeListener(audioFocusListener)
            .build()
        audioManager.requestAudioFocus(audioFocusRequest!!)
    }

    private fun acquireWakeLock() {
        if (wakeLock == null || !wakeLock!!.isHeld) {
            wakeLock = (getSystemService(POWER_SERVICE) as PowerManager)
                .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "raikiri::media")
                .apply { acquire(WAKELOCK_TIMEOUT_MS) }
        }
    }

    private fun releaseWakeLock() {
        wakeLock?.let { if (it.isHeld) it.release() }
        wakeLock = null
    }

    override fun onDestroy() {
        handler.removeCallbacksAndMessages(null)
        mediaSession.release()
        audioFocusRequest?.let { audioManager.abandonAudioFocusRequest(it) }
        releaseWakeLock()
        super.onDestroy()
    }
}

private const val MEDIA_ACTIONS =
    PlaybackStateCompat.ACTION_PLAY or
    PlaybackStateCompat.ACTION_PAUSE or
    PlaybackStateCompat.ACTION_PLAY_PAUSE or
    PlaybackStateCompat.ACTION_SKIP_TO_NEXT or
    PlaybackStateCompat.ACTION_SKIP_TO_PREVIOUS
