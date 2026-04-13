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
    currentSessionId: null,
    videoDuration: null,
    isPlaying: false,
    availableSubtitles: [],
    activeSubtitleIndex: null,
    _advancing: false,
    _directMode: false,
    _currentSource: null,
    _availableSources: [],
    _preloadAudio: null,
    _preloadedIndex: -1,
    // Audio queue-manifest state (Phase 3)
    _audioHls: null,
    _audioHlsAttached: false,
    _audioQueueMode: false,
    _trackStartOffsets: [],
    _pendingAudioSeek: -1,

    init() {
        this.audioEl = document.getElementById('ep-audio');
        this.videoEl = document.getElementById('ep-video');
        this._preloadAudio = new Audio();
        this._preloadAudio.preload = 'auto';

        // Persistent hls.js instance for audio queue playback.
        // One MediaSource for the life of the page — never changes src between tracks,
        // which is what Chrome 124+ requires for background-tab audio on Android.
        if (window.Hls && Hls.isSupported()) {
            this._audioHls = new Hls({
                enableWorker: true,
                backBufferLength: 60,
                maxMaxBufferLength: 120,
            });
            this._audioHls.on(Hls.Events.MANIFEST_PARSED, () => {
                if (this._pendingAudioSeek >= 0) {
                    try { this.audioEl.currentTime = this._pendingAudioSeek; } catch (e) {}
                    this._pendingAudioSeek = -1;
                }
                this.audioEl.play().catch(() => {});
            });
            this._audioHls.on(Hls.Events.ERROR, (event, data) => {
                if (!data.fatal) return;
                if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
                    this._audioHls.recoverMediaError();
                } else if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
                    this._audioHls.startLoad();
                }
            });
        }

        this.audioEl.addEventListener('ended', () => {
            // Queue mode: whole manifest finished — stop. Track transitions happen mid-stream.
            if (this._audioQueueMode) {
                this.stop();
                return;
            }
            this.next();
        });
        this.audioEl.addEventListener('timeupdate', () => {
            if (this._audioQueueMode) {
                this._checkAudioTrackBoundary();
                const { trackTime, trackDur } = this._currentTrackTime();
                UI.updateProgress(trackTime, trackDur);
            } else {
                UI.updateProgress(this.audioEl.currentTime, this.audioEl.duration);
            }
            this.updateMediaSessionPosition();
            if (!this._audioQueueMode) {
                this._maybePreloadNext();
            }
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

    // Returns true if the whole queue is HLS-passthrough audio (today: MP3).
    _queueIsPassthroughAudio(items) {
        if (!items.length) return false;
        return items.every(it => it.type === 'audio' && /\.mp3$/i.test(it.path));
    },

    async _loadAudioQueue(items, startIndex) {
        const canMSE = !!this._audioHls;
        const canNativeHLS = this.audioEl.canPlayType('application/vnd.apple.mpegurl') !== '';
        if (!canMSE && !canNativeHLS) {
            // Neither MSE nor native HLS — fall back to per-track legacy path
            this.currentIndex = startIndex;
            this.load(this.queue[startIndex]);
            return;
        }

        // Tear down any video playback and clear visuals
        this._advancing = false;
        clearTimeout(this.imageTimer);
        this.cleanupHLS();
        this.videoEl.classList.add('hidden');
        document.getElementById('ep-image').classList.add('hidden');

        this.availableSubtitles = [];
        this.activeSubtitleIndex = null;
        this._currentSource = null;
        this._availableSources = [];
        UI.updateSubtitleButton(false);
        UI.updateSourceButton(null, false);
        document.getElementById('ep-audio-art').classList.remove('hidden');

        const tracks = items.map(it => it.path);
        const meta = await API.getQueueMeta(tracks, state.mode);
        this._trackStartOffsets = meta.tracks.map(t => t.startOffset);
        this._audioQueueMode = true;
        this.currentIndex = startIndex;

        const item = this.queue[startIndex];
        const thumb = item.thumb ? API.getContentUrl(item.thumb, state.mode) : null;
        UI.updatePlayerMeta(item, thumb);
        this.updateMediaSession(item, thumb);

        this._pendingAudioSeek = this._trackStartOffsets[startIndex] || 0;

        const manifestUrl = API.getQueueManifestUrl(tracks, state.mode);
        if (canMSE) {
            if (!this._audioHlsAttached) {
                this._audioHls.attachMedia(this.audioEl);
                this._audioHlsAttached = true;
            }
            this._audioHls.loadSource(manifestUrl);
        } else {
            // Safari / native HLS path — set src once, seek on loadedmetadata
            this.audioEl.src = manifestUrl;
            const onMeta = () => {
                this.audioEl.removeEventListener('loadedmetadata', onMeta);
                if (this._pendingAudioSeek >= 0) {
                    try { this.audioEl.currentTime = this._pendingAudioSeek; } catch (e) {}
                    this._pendingAudioSeek = -1;
                }
                this.audioEl.play().catch(() => {});
            };
            this.audioEl.addEventListener('loadedmetadata', onMeta);
            this.audioEl.load();
        }

        this.isPlaying = true;
        UI.showPlayerBar();
        UI.expandPlayer();
        UI.updatePlayButton(this.isPlaying);
        this.updatePlaybackState('playing');
    },

    _currentTrackTime() {
        const offsets = this._trackStartOffsets;
        const idx = this.currentIndex;
        const trackStart = offsets[idx] || 0;
        const nextStart = offsets[idx + 1];
        const total = this.audioEl.duration || 0;
        const trackDur = (nextStart !== undefined ? nextStart : total) - trackStart;
        const trackTime = Math.max(0, this.audioEl.currentTime - trackStart);
        return { trackStart, trackDur, trackTime };
    },

    _checkAudioTrackBoundary() {
        const offsets = this._trackStartOffsets;
        if (!offsets.length) return;
        const t = this.audioEl.currentTime + 0.05;
        let newIdx = 0;
        for (let i = 0; i < offsets.length; i++) {
            if (offsets[i] <= t) newIdx = i;
            else break;
        }
        if (newIdx !== this.currentIndex) {
            this.currentIndex = newIdx;
            const item = this.queue[newIdx];
            if (item) {
                const thumb = item.thumb ? API.getContentUrl(item.thumb, state.mode) : null;
                UI.updatePlayerMeta(item, thumb);
                this.updateMediaSession(item, thumb);
                UI.renderQueueList();
            }
        }
    },

    async _reloadAudioQueue(preserveIndex, preserveTrackTime) {
        if (!this.queue.length) return;
        const tracks = this.queue.map(it => it.path);
        const meta = await API.getQueueMeta(tracks, state.mode);
        this._trackStartOffsets = meta.tracks.map(t => t.startOffset);
        this.currentIndex = Math.min(preserveIndex, this.queue.length - 1);
        this._pendingAudioSeek = (this._trackStartOffsets[this.currentIndex] || 0) + (preserveTrackTime || 0);
        const manifestUrl = API.getQueueManifestUrl(tracks, state.mode);
        if (this._audioHls && this._audioHlsAttached) {
            this._audioHls.loadSource(manifestUrl);
        } else {
            this.audioEl.src = manifestUrl;
            const onMeta = () => {
                this.audioEl.removeEventListener('loadedmetadata', onMeta);
                if (this._pendingAudioSeek >= 0) {
                    try { this.audioEl.currentTime = this._pendingAudioSeek; } catch (e) {}
                    this._pendingAudioSeek = -1;
                }
                this.audioEl.play().catch(() => {});
            };
            this.audioEl.addEventListener('loadedmetadata', onMeta);
            this.audioEl.load();
        }
        const item = this.queue[this.currentIndex];
        if (item) {
            const thumb = item.thumb ? API.getContentUrl(item.thumb, state.mode) : null;
            UI.updatePlayerMeta(item, thumb);
            this.updateMediaSession(item, thumb);
        }
    },

    _teardownAudioQueue() {
        if (!this._audioQueueMode) return;
        this._audioQueueMode = false;
        this._trackStartOffsets = [];
        this._pendingAudioSeek = -1;
        if (this._audioHls && this._audioHlsAttached) {
            this._audioHls.stopLoad();
            this._audioHls.detachMedia();
            this._audioHlsAttached = false;
        }
        try { this.audioEl.removeAttribute('src'); this.audioEl.load(); } catch (e) {}
    },

    setQueue(items, startIndex = 0) {
        this.queue = items.map((item) => ({ ...item }));
        this._preloadedIndex = -1;

        if (this._queueIsPassthroughAudio(this.queue)) {
            this._loadAudioQueue(this.queue, startIndex).catch(e => {
                console.warn('queue manifest failed, falling back to per-track', e);
                this._teardownAudioQueue();
                this.currentIndex = startIndex;
                this.load(this.queue[this.currentIndex]);
            });
            return;
        }

        this._teardownAudioQueue();
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

        if (this._audioQueueMode) {
            let targetIndex = this.currentIndex;
            let preserveTime = 0;
            if (idx < this.currentIndex) {
                targetIndex -= 1;
            } else if (removingCurrent) {
                if (targetIndex >= this.queue.length) targetIndex = this.queue.length - 1;
                preserveTime = 0;
            } else {
                const { trackTime } = this._currentTrackTime();
                preserveTime = trackTime;
            }
            this._reloadAudioQueue(targetIndex, preserveTime).catch(e => {
                console.warn('queue manifest reload failed', e);
            });
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
    },

    _loadHLS(url) {
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

    async load(item) {
        if (!item) return;

        // Legacy per-item load path. Audio-queue-mode playback uses _loadAudioQueue instead.
        this._teardownAudioQueue();

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
            const src = API.getContentUrl(item.path, state.mode);
            this.audioEl.src = src;
            this.audioEl.play();
            this.isPlaying = true;
            loaded = true;
            document.getElementById('ep-audio-art').classList.remove('hidden');
        } else if (item.type === 'video') {
            // Store video path in history before starting playback
            const fullPath = item.path;
            try {
                const history = JSON.parse(localStorage.getItem('raikiri_history') || '[]');
                // Remove if already exists (to move to top)
                const filtered = history.filter(p => p !== fullPath);
                filtered.unshift(fullPath);
                // Cap at 50 entries
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
                        // Direct play failed, auto-fallback to next source
                        console.warn('Direct playback failed, cycling to next source', e);
                        this._directMode = false;
                        this.videoEl.removeAttribute('src');
                        this.videoEl.load();
                        await this._autoFallback(item);
                    }
                } else {
                    this._directMode = false;
                    this._loadHLS(data.url);
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
            if (this._audioQueueMode) {
                const { trackStart, trackDur } = this._currentTrackTime();
                if (trackDur > 0) {
                    this.audioEl.currentTime = trackStart + (percent / 100) * trackDur;
                    this.updateMediaSessionPosition();
                }
            } else if (this.audioEl.duration) {
                this.audioEl.currentTime = (percent / 100) * this.audioEl.duration;
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
        if (item.type === 'audio' && this._audioQueueMode) {
            const { trackStart, trackDur, trackTime } = this._currentTrackTime();
            if (!trackDur) return;
            const nextTrackTime = Math.min(Math.max(0, trackTime + seconds), Math.max(trackDur - 0.01, 0));
            this.audioEl.currentTime = trackStart + nextTrackTime;
            this.updateMediaSessionPosition();
            return;
        }
        let media = null;
        if (item.type === 'audio') media = this.audioEl;
        else if (item.type === 'video') media = this.videoEl;
        if (!media || !media.duration || Number.isNaN(media.duration)) return;
        const next = Math.min(Math.max(0, media.currentTime + seconds), Math.max(media.duration - 0.01, 0));
        media.currentTime = next;
        this.updateMediaSessionPosition();
    },

    next() {
        if (this._audioQueueMode) {
            const nextIdx = this.currentIndex + 1;
            if (nextIdx < this.queue.length) {
                this.audioEl.currentTime = this._trackStartOffsets[nextIdx];
                // timeupdate handler will detect boundary and update UI/metadata.
                if (!this.isPlaying) this.play();
            } else {
                this.stop();
            }
            return;
        }
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
        if (this._audioQueueMode) {
            if (this.currentIndex > 0) {
                const prevIdx = this.currentIndex - 1;
                this.audioEl.currentTime = this._trackStartOffsets[prevIdx];
                if (!this.isPlaying) this.play();
            }
            return;
        }
        if (this.currentIndex > 0) {
            this.currentIndex--;
            this.load(this.queue[this.currentIndex]);
        }
    },

    playIndex(idx) {
        if (idx < 0 || idx >= this.queue.length) return;
        idx = parseInt(idx);
        if (this._audioQueueMode) {
            this.audioEl.currentTime = this._trackStartOffsets[idx] || 0;
            if (!this.isPlaying) this.play();
            return;
        }
        this.currentIndex = idx;
        this.load(this.queue[this.currentIndex]);
    },

    stop() {
        this._advancing = false;
        this._preloadedIndex = -1;
        this.pause();
        this.cleanupHLS();
        this._teardownAudioQueue();
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
            if (this._audioQueueMode) {
                const { trackDur, trackTime } = this._currentTrackTime();
                currentTime = trackTime;
                duration = trackDur;
            } else if (this.audioEl.duration) {
                currentTime = this.audioEl.currentTime;
                duration = this.audioEl.duration;
            }
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

    _maybePreloadNext() {
        if (!this.audioEl.duration || this.currentIndex >= this.queue.length - 1) return;
        const remaining = this.audioEl.duration - this.audioEl.currentTime;
        if (remaining > 15 || remaining <= 0) return;
        const nextIndex = this.currentIndex + 1;
        if (this._preloadedIndex === nextIndex) return;
        const nextItem = this.queue[nextIndex];
        if (!nextItem || nextItem.type !== 'audio') return;
        const src = API.getContentUrl(nextItem.path, state.mode);
        this._preloadAudio.src = src;
        this._preloadedIndex = nextIndex;
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
                this._loadHLS(data.url);
            }
            UI.updateSourceButton(this._currentSource, true);
        } catch (e) {
            console.error('Auto-fallback failed', e);
        }
    },

    async cycleSource() {
        if (!this.queue.length || !this._availableSources.length) return;
        const item = this.queue[this.currentIndex];
        if (!item || item.type !== 'video') return;

        const currentIdx = this._availableSources.indexOf(this._currentSource);
        const nextIdx = (currentIdx + 1) % this._availableSources.length;
        const nextSource = this._availableSources[nextIdx];

        // Save playback position
        const savedTime = this.videoEl.currentTime;

        this.cleanupHLS();
        this.videoEl.removeAttribute('src');
        this.videoEl.load();

        try {
            const data = await this._requestSource(item, nextSource);
            this.currentSessionId = data.sessionId;
            this.videoDuration = data.duration || null;
            this.availableSubtitles = data.subtitles || [];
            this._currentSource = data.source;

            while (this.videoEl.firstChild) {
                this.videoEl.removeChild(this.videoEl.firstChild);
            }

            if (data.mode === 'direct') {
                this._directMode = true;
                this.videoEl.src = data.url;
                // Seek to saved position once metadata loads
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
                // Seek to saved position once HLS manifest is parsed
                if (savedTime > 0) {
                    const seekOnce = () => {
                        this.videoEl.currentTime = savedTime;
                        this.videoEl.removeEventListener('loadeddata', seekOnce);
                    };
                    this.videoEl.addEventListener('loadeddata', seekOnce);
                }
                this._loadHLS(data.url);
            }

            UI.updateSubtitleButton(this.availableSubtitles.length > 0);
            UI.updateSourceButton(this._currentSource, true);

            // Re-apply active subtitle
            if (this.activeSubtitleIndex !== null) {
                this.setSubtitle(this.activeSubtitleIndex);
            }
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
