import state from './state.js';
import UI from './ui.js';
import API from './api.js';
import Player from './player.js';

const App = {
    _recursiveCache: null,
    _searchResults: null,
    _searchDebounce: null,

    async init() {
        UI.init(); // Bind seek events
        Player.init();
        
        this.handleHashChange();
        window.addEventListener('hashchange', () => this.handleHashChange());

        document.getElementById('search-input').addEventListener('input', (e) => this.handleSearch(e.target.value));
        
    },
    
    async handleHashChange() {
        const hash = window.location.hash.slice(1);
        
        if (!hash) {
            window.location.hash = '#/files/';
            return;
        }

        const parts = hash.split('/').filter(p => p).map(p => decodeURIComponent(p));
        if (parts.length < 1) return;
        
        const mode = parts[0];
        const path = '/' + parts.slice(1).join('/'); 
        
        state.mode = mode;
        state.path = path;
        
        this.updateTabs(mode);
        await this.loadDirectory();
    },
    
    updateTabs(mode) {
        const desktopActive = "w-20 py-1.5 rounded-full text-xs font-bold transition-all bg-mauve text-base shadow-sm";
        const desktopInactive = "w-20 py-1.5 rounded-full text-xs font-bold transition-all text-subtext0 hover:text-text";
        const mobileActive = "w-16 py-1 rounded-full text-[10px] font-bold transition-all bg-mauve text-base shadow-sm";
        const mobileInactive = "w-16 py-1 rounded-full text-[10px] font-bold transition-all text-subtext0 hover:text-text";

        // Desktop
        document.getElementById('tab-files').className = mode === 'files' ? desktopActive : desktopInactive;
        document.getElementById('tab-music').className = mode === 'music' ? desktopActive : desktopInactive;
        
        // Mobile
        document.getElementById('tab-files-mob').className = mode === 'files' ? mobileActive : mobileInactive;
        document.getElementById('tab-music-mob').className = mode === 'music' ? mobileActive : mobileInactive;
    },

    async loadDirectory() {
        let items = [];
        
        const isMusic = state.mode === 'music';
        if (isMusic) {
            const res = await API.list(state.path, state.mode);
            items = res.filter(i => i.type === 'folder' || i.type === 'audio');
        } else {
            items = await API.list(state.path, state.mode);
        }
        
        state.items = items;

        UI.renderBreadcrumbs();
        UI.render(items);

        // Reset search state so cache/results don't bleed across directories
        this._recursiveCache = null;
        this._searchResults = null;
        clearTimeout(this._searchDebounce);
        document.getElementById('search-input').value = '';
    },
    
    handleSearch(query) {
        const trimmed = query.trim();
        if (!trimmed) {
            clearTimeout(this._searchDebounce);
            this._searchResults = null;
            UI.render(state.items);
            return;
        }
        clearTimeout(this._searchDebounce);
        this._searchDebounce = setTimeout(() => this._runSearch(trimmed), 200);
    },

    async _runSearch(query) {
        if (this._recursiveCache === null) {
            const path = state.path;
            const mode = state.mode;
            let pool = await API.list(path, mode, true);
            // Don't poison the cache if the user navigated or switched mode during the fetch
            if (state.path !== path || state.mode !== mode) return;
            if (mode === 'music') {
                pool = pool.filter(i => i.type === 'folder' || i.type === 'audio');
            }
            this._recursiveCache = pool;
        }

        // Stale guard: bail if the search box changed during the in-flight fetch
        if (document.getElementById('search-input').value.trim() !== query) return;

        const lower = query.toLowerCase();
        const filtered = this._recursiveCache.filter(item => item.name.toLowerCase().includes(lower));
        this._searchResults = filtered;
        UI.render(filtered, { showPath: true });
    },
    
    switchTab(mode) {
        state.setMode(mode);
    },
    
    toggleView() {
        state.view = state.view === 'grid' ? 'list' : 'grid';
        document.getElementById('view-toggle-icon').setAttribute('data-lucide', state.view === 'grid' ? 'layout-grid' : 'list');
        // Keep the active search results visible when toggling view mid-search
        if (this._searchResults !== null) {
            UI.render(this._searchResults, { showPath: true });
        } else {
            UI.render(state.items);
        }
    },
    
    async handleItemClick(path, type) {
        const source = this._searchResults !== null ? this._searchResults : state.items;
        if (type === 'folder') {
            // Path is already relative from root, just append root slash
            const newPath = `/${path}`;
            state.setPath(newPath.replace(/\/+/g, '/'));
        } else if (['audio', 'video', 'image'].includes(type)) {
            const mediaItems = source.filter(i => ['audio', 'video', 'image'].includes(i.type));
            const clickedIndex = mediaItems.findIndex(i => i.path === path);
            if (clickedIndex !== -1) {
                Player.setQueue(mediaItems, clickedIndex);
            }
        } else {
            window.open(API.getContentUrl(path, state.mode), '_blank');
        }
    },
    
    async playRecursive(shuffle) {
        const allItems = await API.list(state.path, state.mode, true);
        const mediaItems = allItems.filter(item => ['audio', 'video', 'image'].includes(item.type));
        if (!mediaItems.length) return;
        
        if (shuffle) {
            for (let i = mediaItems.length - 1; i > 0; i--) {
                const j = Math.floor(Math.random() * (i + 1));
                [mediaItems[i], mediaItems[j]] = [mediaItems[j], mediaItems[i]];
            }
        }
        
        Player.setQueue(mediaItems, 0);
    },
    
    async handleUpload(files) {
        if (!files.length) return;
        const success = await API.upload(files, state.path, state.mode);
        if (success) {
            this.loadDirectory();
        } else {
            UI.showError('Upload failed');
        }
    }
};

window.app = App;
window.player = Player;
window.ui = UI;
window.queue = {
    shuffleCurrentPath: () => App.playRecursive(true)
};
window.navigate = (path) => state.setPath(path);

document.getElementById('main-container').addEventListener('click', (e) => {
    const el = e.target.closest('[data-id]');
    if (el) {
        App.handleItemClick(el.dataset.id, el.dataset.type);
    }
});

// Handle image error events for fallback logic (XSS-safe event delegation)
document.getElementById('main-container').addEventListener('error', (e) => {
    const img = e.target;
    if (img.tagName === 'IMG' && img.hasAttribute('data-try-orig')) {
        const tryOrig = img.getAttribute('data-try-orig') === 'true';
        const itemType = img.getAttribute('data-item-type');
        const origUrl = img.getAttribute('data-orig-url');
        
        if (!tryOrig && itemType === 'image' && origUrl) {
            img.setAttribute('data-try-orig', 'true');
            img.src = origUrl;
        } else {
            img.style.display = 'none';
        }
    }
}, true); // Use capture phase to catch errors

const fileInput = document.getElementById('file-upload');
fileInput.addEventListener('change', async (e) => {
    const files = e.target.files;
    await App.handleUpload(files);
    // Reset so selecting the same file again still fires a change event
    e.target.value = '';
});

App.init();
