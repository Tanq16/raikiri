import state from './state.js';
import UI from './ui.js';
import API from './api.js';
import Player from './player.js';

const App = {
    async init() {
        UI.init(); // Bind seek events
        Player.init();
        
        this.handleHashChange();
        window.addEventListener('hashchange', () => this.handleHashChange());

        // Search Listener
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
        
        // Clear search
        document.getElementById('search-input').value = '';
    },
    
    handleSearch(query) {
        if (!query) {
            UI.render(state.items);
            return;
        }
        const lower = query.toLowerCase();
        const filtered = state.items.filter(item => item.name.toLowerCase().includes(lower));
        UI.render(filtered);
    },
    
    switchTab(mode) {
        state.setMode(mode);
    },
    
    toggleView() {
        state.view = state.view === 'grid' ? 'list' : 'grid';
        document.getElementById('view-toggle-icon').setAttribute('data-lucide', state.view === 'grid' ? 'layout-grid' : 'list');
        UI.render(state.items);
    },
    
    async handleItemClick(path, type) {
        if (type === 'folder') {
            // Path is already relative from root, just append root slash
            const newPath = `/${path}`; 
            state.setPath(newPath.replace(/\/+/g, '/'));
        } else if (['audio', 'video', 'image'].includes(type)) {
            // Filter to only media files in current directory
            const mediaItems = state.items.filter(i => ['audio', 'video', 'image'].includes(i.type));
            // Find the index of the clicked file
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
        if (!allItems.length) return;
        
        if (shuffle) {
            for (let i = allItems.length - 1; i > 0; i--) {
                const j = Math.floor(Math.random() * (i + 1));
                [allItems[i], allItems[j]] = [allItems[j], allItems[i]];
            }
        }
        
        Player.setQueue(allItems, 0);
    },
    
    async handleUpload(files) {
        if (!files.length) return;
        const success = await API.upload(files, state.path, state.mode);
        if (success) {
            this.loadDirectory();
        } else {
            alert('Upload failed');
        }
    }
};

window.app = App;
window.player = Player;
window.ui = UI;
window.queue = {
    shuffleCurrentPath: () => App.playRecursive(true),
    playAll: () => App.playRecursive(false)
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
fileInput.addEventListener('change', (e) => App.handleUpload(e.target.files));

App.init();
