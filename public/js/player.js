import API from './api.js';
import state from './state.js';
import UI from './ui.js';

const Player = {
    queue: [],
    currentIndex: -1,
    audioEl: new Audio(),
    videoEl: null,
    imageTimer: null,
    hls: null,
    currentSessionId: null,
    videoDuration: null,
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
            const duration = this.videoDuration || this.videoEl.duration;
            UI.updateProgress(this.videoEl.currentTime, duration);
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
        this.queue = items.map((item) => ({ ...item }));
        this.currentIndex = startIndex;
        this.load(this.queue[this.currentIndex]);
    },

    removeFromQueue(idx) {
        if (idx < 0 || idx >= this.queue.length) return;
        const removingCurrent = idx === this.currentIndex;
        this.queue.splice(idx, 1);

        if (!this.queue.length) {
            this.stop();
            UI.renderQueueList();
            return;
        }

        if (idx < this.currentIndex) {
            this.currentIndex -= 1;
        } else if (removingCurrent) {
            if (this.currentIndex >= this.queue.length) {
                this.currentIndex = this.queue.length - 1;
            }
            this.load(this.queue[this.currentIndex]);
            return;
        }

        UI.renderQueueList();
    },

    cleanupHLS() {
        if (this.hls) {
            this.hls.destroy();
            this.hls = null;
        }
        
        if (this.currentSessionId) {
            fetch(`/api/stop-stream?session=${this.currentSessionId}`);
            this.currentSessionId = null;
        }
        
        this.videoDuration = null;
    },

    async load(item) {
        if (!item) return;
        
        this.pause();
        clearTimeout(this.imageTimer);
        this.cleanupHLS();
        this.videoEl.classList.add('hidden');
        this.videoEl.pause();
        document.getElementById('ep-image').classList.add('hidden');
        document.getElementById('ep-audio-art').classList.add('hidden');

        const thumb = item.thumb ? API.getContentUrl(item.thumb, state.mode) : null;
        
        UI.updatePlayerMeta(item, thumb);
        this.updateMediaSession(item, thumb);

        if (item.type === 'audio') {
            const src = API.getContentUrl(item.path, state.mode);
            this.audioEl.src = src;
            this.audioEl.play();
            this.isPlaying = true;
            document.getElementById('ep-audio-art').classList.remove('hidden');
        } else if (item.type === 'video') {
            try {
                const res = await fetch(`/api/stream?file=${encodeURIComponent(item.path)}&mode=${state.mode}`);
                const data = await res.json();
                
                this.currentSessionId = data.sessionId;
                this.videoDuration = data.duration || null;
                const hlsUrl = data.url;
                this.videoEl.classList.remove('hidden');
                
                if (Hls.isSupported()) {
                    this.hls = new Hls();
                    this.hls.loadSource(hlsUrl);
                    this.hls.attachMedia(this.videoEl);
                    this.hls.on(Hls.Events.MANIFEST_PARSED, () => {
                        this.videoEl.play();
                    });
                } else if (this.videoEl.canPlayType('application/vnd.apple.mpegurl')) {
                    this.videoEl.src = hlsUrl;
                    this.videoEl.play();
                }
                
                this.isPlaying = true;
            } catch (e) {
                console.error("Stream failed", e);
            }
        } else if (item.type === 'image') {
            const img = document.getElementById('ep-image');
            const src = API.getContentUrl(item.path, state.mode);
            img.src = src;
            img.classList.remove('hidden');
            
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
        else if (item.type === 'video') this.videoEl.play();
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
        UI.updatePlayButton(false);
        this.updatePlaybackState('paused');
        this.updateMediaSessionPosition();
    },
    
    seek(percent) {
        if (!this.queue.length) return;
        const item = this.queue[this.currentIndex];
        
        if (item.type === 'audio' && this.audioEl.duration) {
            this.audioEl.currentTime = (percent / 100) * this.audioEl.duration;
            this.updateMediaSessionPosition();
        } else if (item.type === 'video') {
            const duration = this.videoDuration || this.videoEl.duration;
            if (duration) {
                this.videoEl.currentTime = (percent / 100) * duration;
                this.updateMediaSessionPosition();
            }
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
        this.cleanupHLS();
        this.queue = [];
        this.currentIndex = -1;
        UI.hidePlayerBar();
        
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
        } else if (item.type === 'video') {
            currentTime = this.videoEl.currentTime;
            duration = this.videoDuration || this.videoEl.duration;
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
    }
};

window.playQueueIndex = (idx) => Player.playIndex(idx); 

export default Player;
