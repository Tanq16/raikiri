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
    // MSE audio pipeline state
    _mseActive: false,
    _mediaSource: null,
    _sourceBuffer: null,
    _trackStartOffsets: [],   // cumulative start time per track
    _trackDurations: [],      // duration per track
    _prefetchedIndex: -1,     // highest track index appended to SourceBuffer
    _prefetching: false,

    init() {
        this.audioEl = document.getElementById('ep-audio');
        this.videoEl = document.getElementById('ep-video');

        this.audioEl.addEventListener('ended', () => {
            if (this._mseActive) {
                // All buffered data consumed. If there are unfetched tracks,
                // they'll be appended when JS resumes and 'waiting' fires.
                // If we're past the last track, stop.
                if (this._prefetchedIndex >= this.queue.length - 1) {
                    this.stop();
                }
                return;
            }
            this.next();
        });

        this.audioEl.addEventListener('timeupdate', () => {
            if (this._mseActive) {
                this._checkMSETrackBoundary();
                const { trackTime, trackDur } = this._currentMSETrackTime();
                UI.updateProgress(trackTime, trackDur);
                this.updateMediaSessionPosition();
                this._maybePrefetchNext();
            } else {
                UI.updateProgress(this.audioEl.currentTime, this.audioEl.duration);
                this.updateMediaSessionPosition();
            }
        });

        // If MSE buffer runs dry mid-playback (JS was frozen), resume prefetch
        this.audioEl.addEventListener('waiting', () => {
            if (this._mseActive && !this._prefetching && this._prefetchedIndex < this.queue.length - 1) {
                this._appendTrackData(this._prefetchedIndex + 1).catch(e => console.warn('MSE prefetch on waiting:', e));
            }
        });

        this.videoEl.addEventListener('ended', () => this.next());
        this.videoEl.addEventListener('timeupdate', () => {
            const duration = this.videoDuration || this.videoEl.duration;
            UI.updateProgress(this.videoEl.currentTime, duration);
            this.updateMediaSessionPosition();
            if (this.videoDuration && this.videoEl.currentTime > 0 && !this._advancing) {
                const remaining = this.videoDuration - this.videoEl.currentTime;
                if (remaining < 1 && remaining >= 0) {
                    this.next();
                }
            }
        });

        this.audioEl.addEventListener('loadedmetadata', () => this.updateMediaSessionPosition());
        this.videoEl.addEventListener('loadedmetadata', () => this.updateMediaSessionPosition());

        if ('mediaSession' in navigator) {
            navigator.mediaSession.setActionHandler('play', () => this.play());
            navigator.mediaSession.setActionHandler('pause', () => this.pause());
            navigator.mediaSession.setActionHandler('previoustrack', () => this.prev());
            navigator.mediaSession.setActionHandler('nexttrack', () => this.next());
        }

        window.addEventListener('beforeunload', () => {
            if (this.currentSessionId) {
                navigator.sendBeacon(`/api/stop-stream?session=${this.currentSessionId}`);
            }
        });
    },

    // ── MSE Audio Pipeline ──────────────────────────────────────────────

    async _startMSEQueue(startIndex) {
        this._teardownMSE();
        this._cleanupVideo();
        clearTimeout(this.imageTimer);
        this.videoEl.classList.add('hidden');
        document.getElementById('ep-image').classList.add('hidden');
        document.getElementById('ep-audio-art').classList.remove('hidden');

        this.availableSubtitles = [];
        this.activeSubtitleIndex = null;
        this._currentSource = null;
        this._availableSources = [];
        UI.updateSubtitleButton(false);
        UI.updateSourceButton(null, false);

        this.currentIndex = startIndex;
        this._trackStartOffsets = [];
        this._trackDurations = [];
        this._prefetchedIndex = -1;
        this._prefetching = false;

        this._mediaSource = new MediaSource();
        this.audioEl.src = URL.createObjectURL(this._mediaSource);

        // play() must be called before any await to preserve user gesture token
        this.audioEl.play().catch(e => console.warn('MSE play():', e));
        this.isPlaying = true;

        await new Promise((resolve, reject) => {
            this._mediaSource.addEventListener('sourceopen', resolve, { once: true });
            this._mediaSource.addEventListener('error', reject, { once: true });
        });

        this._sourceBuffer = this._mediaSource.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
        this._sourceBuffer.mode = 'sequence';
        this._mseActive = true;

        await this._appendTrackData(startIndex);

        if (this._trackStartOffsets[startIndex]) {
            this.audioEl.currentTime = this._trackStartOffsets[startIndex];
        }

        const item = this.queue[startIndex];
        const thumb = item.thumb ? API.getContentUrl(item.thumb, state.mode) : null;
        UI.updatePlayerMeta(item, thumb);
        this.updateMediaSession(item, thumb);
        UI.showPlayerBar();
        UI.expandPlayer();
        UI.updatePlayButton(true);
        this.updatePlaybackState('playing');

        this._maybePrefetchNext();
    },

    async _appendTrackData(trackIndex) {
        if (trackIndex < 0 || trackIndex >= this.queue.length) return;
        if (this._prefetching) return;

        this._prefetching = true;
        try {
            const item = this.queue[trackIndex];
            const url = API.getAudioFMP4Url(item.path, state.mode);
            const response = await fetch(url);
            if (!response.ok) throw new Error(`audio-fmp4 failed: ${response.status}`);

            const duration = parseFloat(response.headers.get('X-Audio-Duration') || '0');
            const data = await response.arrayBuffer();

            if (!this._mseActive || !this._sourceBuffer) return;

            // Wait for any pending SourceBuffer operation
            await this._waitForSB();
            this._sourceBuffer.appendBuffer(data);
            await this._waitForSB();

            // Record track metadata
            const startOffset = this._trackDurations.reduce((a, b) => a + b, 0);
            this._trackDurations[trackIndex] = duration;
            this._trackStartOffsets[trackIndex] = startOffset;
            this._prefetchedIndex = trackIndex;

            // If this is the last track, signal end of stream
            if (trackIndex >= this.queue.length - 1 && this._mediaSource.readyState === 'open') {
                this._mediaSource.endOfStream();
            }
        } finally {
            this._prefetching = false;
        }
    },

    _waitForSB() {
        return new Promise((resolve, reject) => {
            if (!this._sourceBuffer || !this._sourceBuffer.updating) {
                resolve();
                return;
            }
            this._sourceBuffer.addEventListener('updateend', resolve, { once: true });
            this._sourceBuffer.addEventListener('error', () => reject(new Error('SourceBuffer error')), { once: true });
        });
    },

    _maybePrefetchNext() {
        if (!this._mseActive || this._prefetching) return;
        if (this._prefetchedIndex >= this.queue.length - 1) return;

        const nextIdx = this._prefetchedIndex + 1;

        // Start pre-fetching when we're past 50% of the current track,
        // or if there's less than 30 seconds of buffered audio remaining
        const trackStart = this._trackStartOffsets[this.currentIndex] || 0;
        const trackDur = this._trackDurations[this.currentIndex] || 0;
        const trackTime = this.audioEl.currentTime - trackStart;
        const totalBuffered = this._trackStartOffsets[this._prefetchedIndex] + (this._trackDurations[this._prefetchedIndex] || 0);
        const bufferedAhead = totalBuffered - this.audioEl.currentTime;

        if (trackDur > 0 && (trackTime > trackDur * 0.5 || bufferedAhead < 30)) {
            this._appendTrackData(nextIdx).catch(e => {
                console.warn('Prefetch failed:', e);
            });
        }
    },

    _checkMSETrackBoundary() {
        if (!this._trackStartOffsets.length) return;
        const t = this.audioEl.currentTime;
        let newIdx = this.currentIndex;

        // Find which track we're in
        for (let i = 0; i <= this._prefetchedIndex && i < this.queue.length; i++) {
            const start = this._trackStartOffsets[i] || 0;
            const dur = this._trackDurations[i] || 0;
            if (t >= start - 0.05 && t < start + dur + 0.05) {
                newIdx = i;
                break;
            }
            if (i === this._prefetchedIndex) {
                newIdx = i;
            }
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

    _currentMSETrackTime() {
        const idx = this.currentIndex;
        const trackStart = this._trackStartOffsets[idx] || 0;
        const trackDur = this._trackDurations[idx] || 0;
        const trackTime = Math.max(0, this.audioEl.currentTime - trackStart);
        return { trackStart, trackDur, trackTime };
    },

    _teardownMSE() {
        if (!this._mseActive) return;
        this._mseActive = false;
        this._prefetchedIndex = -1;
        this._prefetching = false;
        this._trackStartOffsets = [];
        this._trackDurations = [];

        if (this._mediaSource && this._mediaSource.readyState === 'open') {
            try {
                if (this._sourceBuffer) {
                    this._mediaSource.removeSourceBuffer(this._sourceBuffer);
                }
                this._mediaSource.endOfStream();
            } catch (e) {}
        }
        this._sourceBuffer = null;
        this._mediaSource = null;

        try {
            this.audioEl.removeAttribute('src');
            this.audioEl.load();
        } catch (e) {}
    },

    // ── Queue Management ────────────────────────────────────────────────

    // MSE SourceBuffer only works with AAC fMP4. M4A (already AAC) and MP3
    // (fast transcode) are fine. WAV/FLAC/OGG need heavy transcoding — play
    // those directly via the browser's native audio decoder instead.
    _canUseMSE(items) {
        if (!items.length) return false;
        const mseExts = /\.(m4a|mp3|aac)$/i;
        return items.every(it => it.type === 'audio' && mseExts.test(it.path));
    },

    setQueue(items, startIndex = 0) {
        this.queue = items.map((item) => ({ ...item }));

        if (this._canUseMSE(this.queue)) {
            this._startMSEQueue(startIndex).catch(e => {
                console.warn('MSE queue failed, falling back to direct:', e);
                this._teardownMSE();
                this.currentIndex = startIndex;
                this.load(this.queue[this.currentIndex]);
            });
        } else {
            this._teardownMSE();
            this.currentIndex = startIndex;
            this.load(this.queue[this.currentIndex]);
        }
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

        if (this._mseActive) {
            // Rebuild MSE from current position
            let targetIndex = this.currentIndex;
            if (idx < this.currentIndex) targetIndex--;
            if (targetIndex >= this.queue.length) targetIndex = this.queue.length - 1;
            this._startMSEQueue(targetIndex).catch(() => this.stop());
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

    // ── Video Helpers ───────────────────────────────────────────────────

    _cleanupVideo() {
        if (this.hls) {
            this.hls.destroy();
            this.hls = null;
        }
        if (this._directMode) {
            this.videoEl.removeAttribute('src');
            this.videoEl.load();
            this._directMode = false;
        }
        this.videoEl.pause();
        this.videoEl.classList.add('hidden');

        if (this.currentSessionId) {
            fetch(`/api/stop-stream?session=${this.currentSessionId}`);
            this.currentSessionId = null;
        }
        this.videoDuration = null;
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
                if (this.videoDuration) {
                    const remaining = this.videoDuration - this.videoEl.currentTime;
                    if (remaining < 5) {
                        if (data.fatal || (data.type === Hls.ErrorTypes.NETWORK_ERROR && data.details === Hls.ErrorDetails.FRAG_LOAD_ERROR)) {
                            this.next();
                            return;
                        }
                    }
                }
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

    // ── Per-Item Load (video, image, non-MSE fallback) ──────────────────

    async load(item) {
        if (!item) return;

        this._advancing = false;
        clearTimeout(this.imageTimer);
        this._teardownMSE();
        this._cleanupVideo();
        document.getElementById('ep-image').classList.add('hidden');
        document.getElementById('ep-audio-art').classList.add('hidden');

        this.audioEl.pause();
        this.audioEl.removeAttribute('src');
        this.audioEl.load();

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
            const src = API.getContentUrl(item.path, state.mode);
            this.audioEl.src = src;
            this.audioEl.play().catch(() => {});
            this.isPlaying = true;
            loaded = true;
        } else if (item.type === 'video') {
            const fullPath = item.path;
            try {
                const history = JSON.parse(localStorage.getItem('raikiri_history') || '[]');
                const filtered = history.filter(p => p !== fullPath);
                filtered.unshift(fullPath);
                localStorage.setItem('raikiri_history', JSON.stringify(filtered.slice(0, 50)));
            } catch (e) {}

            try {
                const data = await this._requestSource(item);
                this.currentSessionId = data.sessionId;
                this.videoDuration = data.duration || null;
                this.availableSubtitles = data.subtitles || [];
                this._availableSources = data.availableSources || [];
                this._currentSource = data.source;
                this.videoEl.classList.remove('hidden');

                while (this.videoEl.firstChild) this.videoEl.removeChild(this.videoEl.firstChild);

                if (data.mode === 'direct') {
                    this._directMode = true;
                    this.videoEl.src = data.url;
                    try {
                        await this.videoEl.play();
                    } catch (e) {
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
            this.updatePlaybackState('playing');
        }
    },

    // ── Playback Controls ───────────────────────────────────────────────

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
            if (this._mseActive) {
                const { trackStart, trackDur } = this._currentMSETrackTime();
                if (trackDur > 0) {
                    this.audioEl.currentTime = trackStart + (percent / 100) * trackDur;
                }
            } else if (this.audioEl.duration) {
                this.audioEl.currentTime = (percent / 100) * this.audioEl.duration;
            }
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
        if (item.type === 'audio' && this._mseActive) {
            const { trackStart, trackDur, trackTime } = this._currentMSETrackTime();
            if (!trackDur) return;
            const next = Math.min(Math.max(0, trackTime + seconds), Math.max(trackDur - 0.01, 0));
            this.audioEl.currentTime = trackStart + next;
            this.updateMediaSessionPosition();
            return;
        }
        let media = null;
        if (item.type === 'audio') media = this.audioEl;
        else if (item.type === 'video') media = this.videoEl;
        if (!media || !media.duration || Number.isNaN(media.duration)) return;
        media.currentTime = Math.min(Math.max(0, media.currentTime + seconds), Math.max(media.duration - 0.01, 0));
        this.updateMediaSessionPosition();
    },

    next() {
        if (this._mseActive) {
            const nextIdx = this.currentIndex + 1;
            if (nextIdx < this.queue.length) {
                if (nextIdx <= this._prefetchedIndex) {
                    // Already buffered — just seek
                    this.audioEl.currentTime = this._trackStartOffsets[nextIdx] || 0;
                    if (!this.isPlaying) this.play();
                } else {
                    // Not yet buffered — append and seek
                    this._appendTrackData(nextIdx).then(() => {
                        this.audioEl.currentTime = this._trackStartOffsets[nextIdx] || 0;
                        if (!this.isPlaying) this.play();
                    }).catch(() => this.stop());
                }
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
        if (this._mseActive) {
            if (this.currentIndex > 0) {
                const prevIdx = this.currentIndex - 1;
                if (prevIdx >= 0 && this._trackStartOffsets[prevIdx] !== undefined) {
                    this.audioEl.currentTime = this._trackStartOffsets[prevIdx];
                    if (!this.isPlaying) this.play();
                }
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
        if (this._mseActive) {
            if (idx <= this._prefetchedIndex && this._trackStartOffsets[idx] !== undefined) {
                this.audioEl.currentTime = this._trackStartOffsets[idx];
                if (!this.isPlaying) this.play();
            } else {
                // Target not in buffer — rebuild from that track
                this._startMSEQueue(idx).catch(() => this.stop());
            }
            return;
        }
        this.currentIndex = idx;
        this.load(this.queue[this.currentIndex]);
    },

    stop() {
        this._advancing = false;
        this._teardownMSE();
        this.audioEl.pause();
        try { this.audioEl.removeAttribute('src'); this.audioEl.load(); } catch (e) {}
        this._cleanupVideo();
        clearTimeout(this.imageTimer);
        this.queue = [];
        this.currentIndex = -1;
        this.isPlaying = false;
        UI.updatePlayButton(false);
        UI.hidePlayerBar();
        this.updatePlaybackState('none');
        if ('mediaSession' in navigator && navigator.mediaSession.setPositionState) {
            try { navigator.mediaSession.setPositionState(null); } catch (e) {}
        }
    },

    // ── Media Session ───────────────────────────────────────────────────

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
            if (this._mseActive) {
                const { trackDur, trackTime } = this._currentMSETrackTime();
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
                    duration, playbackRate,
                    position: Math.min(currentTime, duration)
                });
            } catch (e) {}
        }
    },

    updatePlaybackState(state) {
        if (!('mediaSession' in navigator)) return;
        try {
            navigator.mediaSession.playbackState = state || (this.isPlaying ? 'playing' : 'paused');
        } catch (e) {}
    },

    // ── Video Source Management ──────────────────────────────────────────

    async _requestSource(item, source) {
        const params = new URLSearchParams({ file: item.path, mode: state.mode });
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
            this._cleanupVideo();
            const data = await this._requestSource(item, nextSource);
            this.currentSessionId = data.sessionId;
            this.videoDuration = data.duration || null;
            this._currentSource = data.source;
            this.videoEl.classList.remove('hidden');
            while (this.videoEl.firstChild) this.videoEl.removeChild(this.videoEl.firstChild);
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
        if (!item || item.type !== 'video') return;
        const currentIdx = this._availableSources.indexOf(this._currentSource);
        const nextIdx = (currentIdx + 1) % this._availableSources.length;
        const nextSource = this._availableSources[nextIdx];
        const savedTime = this.videoEl.currentTime;

        this._cleanupVideo();
        this.videoEl.removeAttribute('src');
        this.videoEl.load();

        try {
            const data = await this._requestSource(item, nextSource);
            this.currentSessionId = data.sessionId;
            this.videoDuration = data.duration || null;
            this.availableSubtitles = data.subtitles || [];
            this._currentSource = data.source;
            this.videoEl.classList.remove('hidden');
            while (this.videoEl.firstChild) this.videoEl.removeChild(this.videoEl.firstChild);

            const seekOnce = () => { this.videoEl.currentTime = savedTime; this.videoEl.removeEventListener('loadeddata', seekOnce); };
            if (savedTime > 0) this.videoEl.addEventListener('loadeddata', seekOnce);

            if (data.mode === 'direct') {
                this._directMode = true;
                this.videoEl.src = data.url;
                await this.videoEl.play();
            } else {
                this._directMode = false;
                this._loadVideoHLS(data.url);
            }

            UI.updateSubtitleButton(this.availableSubtitles.length > 0);
            UI.updateSourceButton(this._currentSource, true);
            if (this.activeSubtitleIndex !== null) this.setSubtitle(this.activeSubtitleIndex);
        } catch (e) {
            console.error('Source cycle failed', e);
        }
    },

    setSubtitle(index) {
        if (!this.videoEl || !this.currentSessionId) return;
        this.videoEl.querySelectorAll('track').forEach(t => t.remove());
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
