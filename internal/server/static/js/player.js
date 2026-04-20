import API from './api.js';
import state from './state.js';
import UI from './ui.js';

const Player = {
    queue: [],
    currentIndex: -1,
    audioEl: null,
    videoEl: null,
    imageTimer: null,
    hls: null,
    audioHls: null,
    currentSessionId: null,
    videoDuration: null,
    audioDuration: null,
    isPlaying: false,
    availableSubtitles: [],
    activeSubtitleIndex: null,
    _advancing: false,
    _directMode: false,
    _currentSource: null,
    _availableSources: [],

    init() {
        this.audioEl = document.getElementById('ep-audio');
        this.videoEl = document.getElementById('ep-video');

        this.audioEl.addEventListener('ended', () => this.next());
        this.audioEl.addEventListener('timeupdate', () => {
            const duration = this.audioDuration || this.audioEl.duration;
            UI.updateProgress(this.audioEl.currentTime, duration);
            this.updateMediaSessionPosition();
        });

        this.videoEl.addEventListener('ended', () => this.next());
        this.videoEl.addEventListener('timeupdate', () => {
            const duration = this.videoDuration || this.videoEl.duration;
            UI.updateProgress(this.videoEl.currentTime, duration);
            this.updateMediaSessionPosition();
            // Safety net: if within 1s of known duration, advance once
            if (this.videoDuration && this.videoEl.currentTime > 0 && !this._advancing) {
                const remaining = this.videoDuration - this.videoEl.currentTime;
                if (remaining < 1 && remaining >= 0) {
                    this.next();
                }
            }
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

        // Clean up sessions when tab is closed
        window.addEventListener('beforeunload', () => {
            if (this.currentSessionId) {
                navigator.sendBeacon(`/api/stop-stream?session=${this.currentSessionId}`);
            }
        });
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
        if (this.audioHls) {
            this.audioHls.destroy();
            this.audioHls = null;
        }

        if (this._directMode) {
            this.videoEl.removeAttribute('src');
            this.videoEl.load();
            this._directMode = false;
        }

        if (this.currentSessionId) {
            fetch(`/api/stop-stream?session=${this.currentSessionId}`);
            this.currentSessionId = null;
        }

        this.videoDuration = null;
        this.audioDuration = null;
    },

    _loadVideoHLS(url) {
        if (Hls.isSupported()) {
            this.hls = new Hls({
                enableWorker: true,
                lowLatencyMode: false,
                startPosition: 0,
                stretchShortVideoTrack: true,
                backBufferLength: 60,
                maxMaxBufferLength: 120,
                nudgeMaxRetry: 5,
                manifestLoadingMaxRetry: 2,
            });
            this.hls.loadSource(url);
            this.hls.attachMedia(this.videoEl);
            this.hls.on(Hls.Events.MANIFEST_PARSED, () => {
                this.videoEl.play().catch(() => {});
            });
            this.hls.on(Hls.Events.ERROR, (event, data) => {
                if (this._advancing) return;
                // Near end of video: treat fatal/frag errors as "ended"
                if (this.videoDuration) {
                    const remaining = this.videoDuration - this.videoEl.currentTime;
                    if (remaining < 5) {
                        if (data.fatal || (data.type === Hls.ErrorTypes.NETWORK_ERROR && data.details === Hls.ErrorDetails.FRAG_LOAD_ERROR)) {
                            this.next();
                            return;
                        }
                    }
                }
                // Mid-playback recovery for fatal errors
                if (data.fatal) {
                    if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
                        this.hls.recoverMediaError();
                    } else if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
                        this.hls.startLoad();
                    }
                }
            });
        } else if (this.videoEl.canPlayType('application/vnd.apple.mpegurl')) {
            this.videoEl.src = url;
            this.videoEl.play().catch(() => {});
        }
    },

    _loadAudioHLS(url) {
        if (Hls.isSupported()) {
            this.audioHls = new Hls({
                enableWorker: true,
                backBufferLength: 60,
                maxMaxBufferLength: 120,
            });
            this.audioHls.loadSource(url);
            this.audioHls.attachMedia(this.audioEl);
            this.audioHls.on(Hls.Events.MANIFEST_PARSED, () => {
                this.audioEl.play().catch(() => {});
            });
            this.audioHls.on(Hls.Events.ERROR, (event, data) => {
                if (this._advancing) return;
                if (this.audioDuration) {
                    const remaining = this.audioDuration - this.audioEl.currentTime;
                    if (remaining < 5) {
                        if (data.fatal || (data.type === Hls.ErrorTypes.NETWORK_ERROR && data.details === Hls.ErrorDetails.FRAG_LOAD_ERROR)) {
                            this.next();
                            return;
                        }
                    }
                }
                if (data.fatal) {
                    if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
                        this.audioHls.recoverMediaError();
                    } else if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
                        this.audioHls.startLoad();
                    }
                }
            });
        } else if (this.audioEl.canPlayType('application/vnd.apple.mpegurl')) {
            // Safari native HLS
            this.audioEl.src = url;
            this.audioEl.play().catch(() => {});
        }
    },

    async load(item) {
        if (!item) return;

        this._advancing = false;
        this.pause();
        clearTimeout(this.imageTimer);
        this.cleanupHLS();
        this.videoEl.classList.add('hidden');
        document.getElementById('ep-image').classList.add('hidden');
        document.getElementById('ep-audio-art').classList.add('hidden');

        this.availableSubtitles = [];
        this.activeSubtitleIndex = null;
        this._currentSource = null;
        this._availableSources = [];
        UI.updateSubtitleButton(false);
        UI.updateSourceButton(null, false);

        const thumb = item.thumb ? API.getContentUrl(item.thumb, state.mode) : null;

        UI.updatePlayerMeta(item, thumb);
        this.updateMediaSession(item, thumb);

        let loaded = false;

        if (item.type === 'audio') {
            document.getElementById('ep-audio-art').classList.remove('hidden');
            try {
                const data = await this._requestSource(item);
                this.currentSessionId = data.sessionId;
                this.audioDuration = data.duration || null;
                this._currentSource = data.source;
                this._availableSources = data.availableSources || [];

                this._loadAudioHLS(data.url);

                UI.updateSourceButton(this._currentSource, this._availableSources.length > 1);
                this.isPlaying = true;
                loaded = true;
            } catch (e) {
                console.error("Audio stream failed", e);
            }
        } else if (item.type === 'video') {
            // Store video path in history before starting playback
            const fullPath = item.path;
            try {
                const history = JSON.parse(localStorage.getItem('raikiri_history') || '[]');
                const filtered = history.filter(p => p !== fullPath);
                filtered.unshift(fullPath);
                const capped = filtered.slice(0, 50);
                localStorage.setItem('raikiri_history', JSON.stringify(capped));
            } catch (e) {
                console.error('Failed to save history', e);
            }

            try {
                const data = await this._requestSource(item);

                this.currentSessionId = data.sessionId;
                this.videoDuration = data.duration || null;
                this.availableSubtitles = data.subtitles || [];
                this._availableSources = data.availableSources || [];
                this._currentSource = data.source;
                this.videoEl.classList.remove('hidden');

                while (this.videoEl.firstChild) {
                    this.videoEl.removeChild(this.videoEl.firstChild);
                }

                if (data.mode === 'direct') {
                    this._directMode = true;
                    this.videoEl.src = data.url;
                    try {
                        await this.videoEl.play();
                    } catch (e) {
                        console.warn('Direct playback failed, cycling to next source', e);
                        this._directMode = false;
                        this.videoEl.removeAttribute('src');
                        this.videoEl.load();
                        await this._autoFallback(item);
                    }
                } else {
                    this._directMode = false;
                    this._loadVideoHLS(data.url);
                }

                UI.updateSubtitleButton(this.availableSubtitles.length > 0);
                UI.updateSourceButton(this._currentSource, true);
                this.isPlaying = true;
                loaded = true;
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
            loaded = true;
        }

        if (loaded) {
            UI.showPlayerBar();
            UI.expandPlayer();
            UI.updatePlayButton(this.isPlaying);
            if (this.isPlaying) this.updatePlaybackState('playing');
        }
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

        if (item.type === 'audio') {
            const duration = this.audioDuration || this.audioEl.duration;
            if (duration) {
                this.audioEl.currentTime = (percent / 100) * duration;
                this.updateMediaSessionPosition();
            }
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
        let duration = 0;
        if (item.type === 'audio') { media = this.audioEl; duration = this.audioDuration || this.audioEl.duration; }
        else if (item.type === 'video') { media = this.videoEl; duration = this.videoDuration || this.videoEl.duration; }
        if (!media || !duration || Number.isNaN(duration)) return;
        const next = Math.min(Math.max(0, media.currentTime + seconds), Math.max(duration - 0.01, 0));
        media.currentTime = next;
        this.updateMediaSessionPosition();
    },

    next() {
        if (this._advancing) return;
        this._advancing = true;
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
        this._advancing = false;
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
            const artist = state.mode === 'music' ? 'Raikiri Music' : 'Media';
            navigator.mediaSession.metadata = new MediaMetadata({
                title: item.name,
                artist: artist,
                album: state.path,
                artwork: thumb ? [{ src: thumb, sizes: '512x512', type: 'image/jpeg' }] : []
            });
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

        if (item.type === 'audio') {
            currentTime = this.audioEl.currentTime;
            duration = this.audioDuration || this.audioEl.duration;
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
                    position: Math.min(currentTime, duration)
                });
            } catch (e) {
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

    async _requestSource(item, source) {
        const params = new URLSearchParams({
            file: item.path,
            mode: state.mode,
        });
        if (source) params.set('source', source);
        const res = await fetch(`/api/stream?${params}`);
        if (!res.ok) throw new Error(`Stream request failed: ${res.status}`);
        return res.json();
    },

    async _autoFallback(item) {
        const currentIdx = this._availableSources.indexOf(this._currentSource);
        if (currentIdx < 0 || this._availableSources.length <= 1) return;

        const nextIdx = (currentIdx + 1) % this._availableSources.length;
        const nextSource = this._availableSources[nextIdx];
        if (nextSource === this._currentSource) return;

        try {
            this.cleanupHLS();
            const data = await this._requestSource(item, nextSource);
            this.currentSessionId = data.sessionId;
            this.videoDuration = data.duration || null;
            this._currentSource = data.source;

            while (this.videoEl.firstChild) {
                this.videoEl.removeChild(this.videoEl.firstChild);
            }

            if (data.mode === 'direct') {
                this._directMode = true;
                this.videoEl.src = data.url;
                await this.videoEl.play();
            } else {
                this._directMode = false;
                this._loadVideoHLS(data.url);
            }
            UI.updateSourceButton(this._currentSource, true);
        } catch (e) {
            console.error('Auto-fallback failed', e);
        }
    },

    async cycleSource() {
        if (!this.queue.length || !this._availableSources.length) return;
        const item = this.queue[this.currentIndex];
        if (!item || (item.type !== 'video' && item.type !== 'audio')) return;

        const currentIdx = this._availableSources.indexOf(this._currentSource);
        const nextIdx = (currentIdx + 1) % this._availableSources.length;
        const nextSource = this._availableSources[nextIdx];

        const isAudio = item.type === 'audio';
        const mediaEl = isAudio ? this.audioEl : this.videoEl;
        const savedTime = mediaEl.currentTime;

        this.cleanupHLS();
        if (!isAudio) {
            this.videoEl.removeAttribute('src');
            this.videoEl.load();
        }

        try {
            const data = await this._requestSource(item, nextSource);
            this.currentSessionId = data.sessionId;
            this._currentSource = data.source;

            if (isAudio) {
                this.audioDuration = data.duration || null;
                // Seek to saved position once loaded
                if (savedTime > 0) {
                    const seekOnce = () => {
                        this.audioEl.currentTime = savedTime;
                        this.audioEl.removeEventListener('loadeddata', seekOnce);
                    };
                    this.audioEl.addEventListener('loadeddata', seekOnce);
                }
                this._loadAudioHLS(data.url);
            } else {
                this.videoDuration = data.duration || null;
                this.availableSubtitles = data.subtitles || [];

                while (this.videoEl.firstChild) {
                    this.videoEl.removeChild(this.videoEl.firstChild);
                }

                if (data.mode === 'direct') {
                    this._directMode = true;
                    this.videoEl.src = data.url;
                    if (savedTime > 0) {
                        const seekOnce = () => {
                            this.videoEl.currentTime = savedTime;
                            this.videoEl.removeEventListener('loadeddata', seekOnce);
                        };
                        this.videoEl.addEventListener('loadeddata', seekOnce);
                    }
                    await this.videoEl.play();
                } else {
                    this._directMode = false;
                    if (savedTime > 0) {
                        const seekOnce = () => {
                            this.videoEl.currentTime = savedTime;
                            this.videoEl.removeEventListener('loadeddata', seekOnce);
                        };
                        this.videoEl.addEventListener('loadeddata', seekOnce);
                    }
                    this._loadVideoHLS(data.url);
                }

                UI.updateSubtitleButton(this.availableSubtitles.length > 0);

                // Re-apply active subtitle
                if (this.activeSubtitleIndex !== null) {
                    this.setSubtitle(this.activeSubtitleIndex);
                }
            }

            UI.updateSourceButton(this._currentSource, true);
        } catch (e) {
            console.error('Source cycle failed', e);
        }
    },

    setSubtitle(index) {
        if (!this.videoEl || !this.currentSessionId) return;

        const existingTracks = this.videoEl.querySelectorAll('track');
        existingTracks.forEach(track => track.remove());

        if (index !== null && index > 0) {
            const track = document.createElement('track');
            track.kind = 'subtitles';
            track.label = `Sub ${index}`;
            track.srclang = 'en';
            track.src = `/api/hls/${this.currentSessionId}/sub_${index}.vtt`;
            track.default = true;
            this.videoEl.appendChild(track);
            track.track.mode = 'showing';
        }

        this.activeSubtitleIndex = index;
    }
};

window.playQueueIndex = (idx) => Player.playIndex(idx);

export default Player;
