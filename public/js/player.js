import API from './api.js';
import state from './state.js';
import UI from './ui.js';

const Player = {
    queue: [],
    currentIndex: -1,
    audioEl: new Audio(),
    videoEl: null, 
    imageTimer: null,
    resyncIntervalId: null,
    // Disable resync loop; native playback stays in sync once SW bypasses media
    resyncEnabled: false,
    resyncIntervalMs: 30000, // how often to nudge video decoder
    resyncRateBump: 0.01, // small playback rate bump to avoid overlay flash
    resyncRateDurationMs: 800, // how long to keep the rate bump
    resyncRateTimerId: null,
    basePlaybackRate: 1.0,
    // Periodic micro-seek to re-anchor playback without visible flash
    driftFixEnabled: true,
    driftSeekSeconds: 0.1,
    driftIntervalMs: 120000, // every 2 minutes
    driftIntervalId: null,
    isPlaying: false,

    init() {
        this.videoEl = document.getElementById('ep-video');
        
        this.audioEl.addEventListener('ended', () => this.next());
        this.audioEl.addEventListener('timeupdate', () => {
            UI.updateProgress(this.audioEl.currentTime, this.audioEl.duration);
            this.updateMediaSessionPosition();
        });
        
        this.videoEl.addEventListener('ended', () => this.next());
        this.videoEl.addEventListener('timeupdate', () => {
            UI.updateProgress(this.videoEl.currentTime, this.videoEl.duration);
            this.updateMediaSessionPosition();
        });
        
        // Update position when duration becomes available
        this.audioEl.addEventListener('loadedmetadata', () => this.updateMediaSessionPosition());
        this.videoEl.addEventListener('loadedmetadata', () => this.updateMediaSessionPosition());
        
        if ('mediaSession' in navigator) {
            navigator.mediaSession.setActionHandler('play', () => this.play());
            navigator.mediaSession.setActionHandler('pause', () => this.pause());
            navigator.mediaSession.setActionHandler('previoustrack', () => this.prev());
            navigator.mediaSession.setActionHandler('nexttrack', () => this.next());
        }
    },

    setQueue(items, startIndex = 0) {
        this.queue = items.map((item, idx) => ({ ...item, index: idx }));
        this.currentIndex = startIndex;
        this.load(this.queue[this.currentIndex]);
    },

    load(item) {
        if (!item) return;
        
        this.pause();
        clearTimeout(this.imageTimer);
        this.stopResyncLoop();
        this.stopDriftCorrectionLoop();
        
        this.videoEl.classList.add('hidden');
        this.videoEl.pause();
        document.getElementById('ep-image').classList.add('hidden');
        document.getElementById('ep-audio-art').classList.add('hidden');

        const src = API.getContentUrl(item.path, state.mode);
        const thumb = item.thumb ? API.getContentUrl(item.thumb, state.mode) : null;
        
        UI.updatePlayerMeta(item, thumb);
        this.updateMediaSession(item, thumb);

        if (item.type === 'audio') {
            this.audioEl.src = src;
            this.audioEl.play();
            this.isPlaying = true;
            document.getElementById('ep-audio-art').classList.remove('hidden');
        } else if (item.type === 'video') {
            this.videoEl.src = src;
            this.videoEl.classList.remove('hidden');
            this.videoEl.play();
            this.isPlaying = true;
            this.startResyncLoop(item);
            this.startDriftCorrectionLoop(item);
        } else if (item.type === 'image') {
            const img = document.getElementById('ep-image');
            img.src = src;
            img.classList.remove('hidden');
            
            // Update fullscreen image if it's currently showing
            const fullscreenImg = document.getElementById('fullscreen-image');
            const fullscreenContainer = document.getElementById('fullscreen-image-container');
            if (fullscreenImg && fullscreenContainer && !fullscreenContainer.classList.contains('hidden')) {
                fullscreenImg.src = src;
            }
            
            this.isPlaying = true;
            this.imageTimer = setTimeout(() => this.next(), 5000);
        }
        
        UI.showPlayerBar();
        UI.expandPlayer();
        UI.updatePlayButton(true);
    },

    toggle() {
        if (this.isPlaying) this.pause();
        else this.play();
    },

    play() {
        if (!this.queue.length) return;
        const item = this.queue[this.currentIndex];

        if (item.type === 'audio') this.audioEl.play();
        else if (item.type === 'video') {
            this.videoEl.play();
            this.startResyncLoop(item);
            this.startDriftCorrectionLoop(item);
        }
        else if (item.type === 'image') {
            clearTimeout(this.imageTimer);
            this.imageTimer = setTimeout(() => this.next(), 5000);
        }
        
        this.isPlaying = true;
        UI.updatePlayButton(true);
        this.updatePlaybackState('playing');
        this.updateMediaSessionPosition();
    },

    pause() {
        this.audioEl.pause();
        this.videoEl.pause();
        clearTimeout(this.imageTimer);
        this.isPlaying = false;
        this.stopDriftCorrectionLoop();
        UI.updatePlayButton(false);
        // Keep position accurate when pausing
        this.updatePlaybackState('paused');
        this.updateMediaSessionPosition();
    },
    
    seek(percent) {
        if (!this.queue.length) return;
        const item = this.queue[this.currentIndex];
        
        if (item.type === 'audio' && this.audioEl.duration) {
            this.audioEl.currentTime = (percent / 100) * this.audioEl.duration;
            this.updateMediaSessionPosition();
        } else if (item.type === 'video' && this.videoEl.duration) {
            this.videoEl.currentTime = (percent / 100) * this.videoEl.duration;
            this.updateMediaSessionPosition();
        }
    },

    seekBy(seconds) {
        if (!this.queue.length) return;
        const item = this.queue[this.currentIndex];
        let media = null;
        if (item.type === 'audio') media = this.audioEl;
        else if (item.type === 'video') media = this.videoEl;
        if (!media || !media.duration || Number.isNaN(media.duration)) return;
        const next = Math.min(Math.max(0, media.currentTime + seconds), Math.max(media.duration - 0.01, 0));
        media.currentTime = next;
        this.updateMediaSessionPosition();
    },

    next() {
        if (this.currentIndex < this.queue.length - 1) {
            this.currentIndex++;
            this.load(this.queue[this.currentIndex]);
        } else {
            this.stop();
        }
    },

    prev() {
        if (this.currentIndex > 0) {
            this.currentIndex--;
            this.load(this.queue[this.currentIndex]);
        }
    },
    
    playIndex(idx) {
        if (idx >= 0 && idx < this.queue.length) {
            this.currentIndex = parseInt(idx);
            this.load(this.queue[this.currentIndex]);
        }
    },

    stop() {
        this.pause();
        this.queue = [];
        this.currentIndex = -1;
        UI.hidePlayerBar();
        this.stopResyncLoop();
        this.stopDriftCorrectionLoop();
        
        // Clear Media Session position state
        this.updatePlaybackState('none');
        if ('mediaSession' in navigator && navigator.mediaSession.setPositionState) {
            try {
                navigator.mediaSession.setPositionState(null);
            } catch (e) {
                // Ignore errors when clearing position state
            }
        }
    },
    
    updateMediaSession(item, thumb) {
        if ('mediaSession' in navigator) {
            navigator.mediaSession.metadata = new MediaMetadata({
                title: item.name,
                artist: state.mode === 'music' ? 'Raikiri Music' : 'Media',
                album: state.path,
                artwork: thumb ? [{ src: thumb, sizes: '512x512', type: 'image/jpeg' }] : []
            });
            // Update position state after metadata is set
            this.updateMediaSessionPosition();
        }
    },
    
    updateMediaSessionPosition() {
        if (!('mediaSession' in navigator) || !this.queue.length) return;
        
        const item = this.queue[this.currentIndex];
        if (!item || item.type === 'image') return;
        
        let currentTime = 0;
        let duration = 0;
        let playbackRate = this.isPlaying ? 1.0 : 0;
        
        if (item.type === 'audio' && this.audioEl.duration) {
            currentTime = this.audioEl.currentTime;
            duration = this.audioEl.duration;
            playbackRate = this.isPlaying ? (this.audioEl.playbackRate || 1.0) : 0;
        } else if (item.type === 'video' && this.videoEl.duration) {
            currentTime = this.videoEl.currentTime;
            duration = this.videoEl.duration;
            playbackRate = this.isPlaying ? (this.videoEl.playbackRate || 1.0) : 0;
        }
        
        if (duration > 0) {
            try {
                navigator.mediaSession.setPositionState({
                    duration: duration,
                    playbackRate: playbackRate,
                    position: currentTime
                });
            } catch (e) {
                // setPositionState may not be supported in all browsers
                console.debug('MediaSession setPositionState not supported:', e);
            }
        }
    },
    
    updatePlaybackState(state) {
        if (!('mediaSession' in navigator)) return;
        const nextState = state || (this.isPlaying ? 'playing' : 'paused');
        try {
            navigator.mediaSession.playbackState = nextState;
        } catch (e) {
            // Ignore if not supported
        }
    },

    startResyncLoop(item) {
        if (!this.resyncEnabled || !item || item.type !== 'video') return;
        this.stopResyncLoop(); // ensure only one loop is running
        this.resyncIntervalId = setInterval(() => {
            if (!this.isPlaying) return;
            if (!this.videoEl || this.videoEl.paused || this.videoEl.readyState < 2) return;
            if (!this.videoEl.duration || Number.isNaN(this.videoEl.duration)) return;
            // Prefer rate bump everywhere to avoid overlay flash.
            this.nudgePlaybackRate();
        }, this.resyncIntervalMs);
    },

    stopResyncLoop() {
        if (this.resyncIntervalId) {
            clearInterval(this.resyncIntervalId);
            this.resyncIntervalId = null;
        }
        this.clearResyncRate();
    },

    nudgePlaybackRate() {
        if (this.resyncRateTimerId) return true; // already applied
        const video = this.videoEl;
        if (!video) return false;
        this.basePlaybackRate = video.playbackRate || 1.0;
        const bumped = this.basePlaybackRate + this.resyncRateBump;
        try {
            video.playbackRate = bumped;
        } catch (e) {
            return false; // if browser disallows, skip
        }
        this.resyncRateTimerId = setTimeout(() => {
            this.clearResyncRate();
        }, this.resyncRateDurationMs);
        return true;
    },

    clearResyncRate() {
        if (this.resyncRateTimerId) {
            clearTimeout(this.resyncRateTimerId);
            this.resyncRateTimerId = null;
        }
        if (this.videoEl) {
            try {
                this.videoEl.playbackRate = this.basePlaybackRate || 1.0;
            } catch (e) {
                // ignore
            }
        }
    },

    startDriftCorrectionLoop(item) {
        if (!this.driftFixEnabled || !item || item.type !== 'video') return;
        this.stopDriftCorrectionLoop();
        this.driftIntervalId = setInterval(() => {
            if (!this.isPlaying) return;
            const video = this.videoEl;
            if (!video || video.paused || video.readyState < 2) return;
            const target = video.currentTime + this.driftSeekSeconds;
            try {
                if (typeof video.fastSeek === 'function') {
                    video.fastSeek(target);
                } else {
                    video.currentTime = target;
                }
            } catch (e) {
                // swallow to avoid disrupting playback
            }
        }, this.driftIntervalMs);
    },

    stopDriftCorrectionLoop() {
        if (this.driftIntervalId) {
            clearInterval(this.driftIntervalId);
            this.driftIntervalId = null;
        }
    }
};

window.playQueueIndex = (idx) => Player.playIndex(idx); 

export default Player;
