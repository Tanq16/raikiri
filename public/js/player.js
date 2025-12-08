import API from './api.js';
import state from './state.js';
import UI from './ui.js';

const Player = {
    queue: [],
    currentIndex: -1,
    audioEl: new Audio(),
    videoEl: null, 
    imageTimer: null,
    isPlaying: false,

    init() {
        this.videoEl = document.getElementById('ep-video');
        
        this.audioEl.addEventListener('ended', () => this.next());
        this.audioEl.addEventListener('timeupdate', () => UI.updateProgress(this.audioEl.currentTime, this.audioEl.duration));
        
        this.videoEl.addEventListener('ended', () => this.next());
        this.videoEl.addEventListener('timeupdate', () => UI.updateProgress(this.videoEl.currentTime, this.videoEl.duration));
        
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
        else if (item.type === 'video') this.videoEl.play();
        else if (item.type === 'image') {
            clearTimeout(this.imageTimer);
            this.imageTimer = setTimeout(() => this.next(), 5000);
        }
        
        this.isPlaying = true;
        UI.updatePlayButton(true);
    },

    pause() {
        this.audioEl.pause();
        this.videoEl.pause();
        clearTimeout(this.imageTimer);
        this.isPlaying = false;
        UI.updatePlayButton(false);
    },
    
    seek(percent) {
        if (!this.queue.length) return;
        const item = this.queue[this.currentIndex];
        
        if (item.type === 'audio' && this.audioEl.duration) {
            this.audioEl.currentTime = (percent / 100) * this.audioEl.duration;
        } else if (item.type === 'video' && this.videoEl.duration) {
            this.videoEl.currentTime = (percent / 100) * this.videoEl.duration;
        }
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
    },
    
    updateMediaSession(item, thumb) {
        if ('mediaSession' in navigator) {
            navigator.mediaSession.metadata = new MediaMetadata({
                title: item.name,
                artist: state.mode === 'music' ? 'Raikiri Music' : 'Media',
                album: state.path,
                artwork: thumb ? [{ src: thumb, sizes: '512x512', type: 'image/jpeg' }] : []
            });
        }
    }
};

window.playQueueIndex = (idx) => Player.playIndex(idx); 

export default Player;
