import Elements from './elements.js';
import state from './state.js';
import Player from './player.js';
import Escape from './escape.js';
import API from './api.js';

const UI = {
    container: document.getElementById('main-container'),
    
    init() {
        const range = document.getElementById('ep-range');
        range.addEventListener('input', (e) => {
            const percent = e.target.value;
            Player.seek(percent);
            // Update fill in real-time while dragging
            document.getElementById('ep-range-fill').style.width = `${percent}%`;
            document.getElementById('ep-range-fill-mob').style.width = `${percent}%`;
        });
        
        const rangeMob = document.getElementById('ep-range-mob');
        rangeMob.addEventListener('input', (e) => {
            const percent = e.target.value;
            Player.seek(percent);
            // Update fill in real-time while dragging
            document.getElementById('ep-range-fill').style.width = `${percent}%`;
            document.getElementById('ep-range-fill-mob').style.width = `${percent}%`;
        });
        
        // Handle queue item clicks (XSS-safe event delegation)
        const queueContainer = document.getElementById('queue-list-container');
        if (queueContainer) {
            queueContainer.addEventListener('click', (e) => {
                const item = e.target.closest('[data-queue-index]');
                if (item) {
                    const index = item.getAttribute('data-queue-index');
                    if (index !== null) {
                        window.playQueueIndex(parseInt(index, 10));
                    }
                }
            });
        }

        // Handle fullscreen change events (for escape key)
        document.addEventListener('fullscreenchange', () => this.handleFullscreenChange());
        document.addEventListener('webkitfullscreenchange', () => this.handleFullscreenChange());
        document.addEventListener('mozfullscreenchange', () => this.handleFullscreenChange());
        document.addEventListener('MSFullscreenChange', () => this.handleFullscreenChange());

        // Copy stream URL
        const copyBtn = document.getElementById('ep-copy-url');
        if (copyBtn) {
            copyBtn.addEventListener('click', async () => {
                const url = copyBtn.getAttribute('data-url');
                if (!url) return;
                try {
                    if (navigator.clipboard && navigator.clipboard.writeText) {
                        await navigator.clipboard.writeText(url);
                    } else {
                        const ta = document.createElement('textarea');
                        ta.value = url;
                        ta.style.position = 'fixed';
                        ta.style.opacity = '0';
                        document.body.appendChild(ta);
                        ta.select();
                        document.execCommand('copy');
                        document.body.removeChild(ta);
                    }
                } catch (e) {
                    console.error('Failed to copy URL', e);
                }
            });
        }
    },
    
    handlePlayerBarSeek(event) {
        const container = event.currentTarget;
        const rect = container.getBoundingClientRect();
        const percent = ((event.clientX - rect.left) / rect.width) * 100;
        Player.seek(Math.max(0, Math.min(100, percent)));
    },
    
    refreshIcons() {
        if (window.lucide) window.lucide.createIcons();
    },

    render(items) {
        this.container.innerHTML = items.map(item => 
            state.view === 'grid' 
                ? Elements.createGridItem(item) 
                : Elements.createListItem(item)
        ).join('');
        
        this.refreshIcons();
    },
    
    renderBreadcrumbs() {
        const subHeader = document.getElementById('sub-header');
        const parts = state.path.split('/').filter(p => p);
        
        let html = `
            <div class="flex items-center justify-center gap-2 text-sm text-subtext0 mask-fade w-full overflow-hidden whitespace-nowrap">
                <button data-nav-path="/" class="hover:text-mauve transition-colors font-bold"><i data-lucide="home" size="16"></i></button>
        `;
        
        let currentBuild = '';
        parts.forEach(part => {
            currentBuild += '/' + part;
            const escapedPath = Escape.attr(currentBuild);
            // part is already decoded from state.path (which was decoded from URL hash)
            const escapedPart = Escape.html(part);
            html += `
                <span class="text-surface2">/</span>
                <button data-nav-path="${escapedPath}" class="hover:text-mauve transition-colors font-semibold text-text">${escapedPart}</button>
            `;
        });
        
        html += `</div>`;
        subHeader.innerHTML = html;
        
        // Add event listeners for navigation buttons
        subHeader.querySelectorAll('[data-nav-path]').forEach(btn => {
            btn.addEventListener('click', () => {
                window.navigate(btn.getAttribute('data-nav-path'));
            });
        });
        
        this.refreshIcons();
    },
    
    showPlayerBar() {
        document.getElementById('player-bar').classList.remove('translate-y-full');
    },
    
    hidePlayerBar() {
        document.getElementById('player-bar').classList.add('translate-y-full');
        document.getElementById('expanded-player').classList.add('translate-y-full');
        // Reset progress bar to prevent tiny line showing when player is closed
        document.getElementById('pb-progress').style.width = '0%';
    },
    
    expandPlayer() {
        document.getElementById('expanded-player').classList.remove('translate-y-full');
    },
    
    toggleExpanded() {
        document.getElementById('expanded-player').classList.toggle('translate-y-full');
    },
    
    updatePlayButton(isPlaying) {
        const icon = isPlaying ? 'pause' : 'play';
        document.getElementById('pb-play-icon').setAttribute('data-lucide', icon);
        document.getElementById('ep-play-icon').setAttribute('data-lucide', icon);
        const epPlayIconDesktop = document.getElementById('ep-play-icon-desktop');
        if (epPlayIconDesktop) {
            epPlayIconDesktop.setAttribute('data-lucide', icon);
        }
        this.refreshIcons();
    },
    
    updatePlayerMeta(item, thumb) {
        const title = item.name;
        const meta = state.mode === 'music' ? 'Music' : 'File';
        
        document.getElementById('pb-title').innerText = title;
        document.getElementById('pb-meta').innerText = '';
        
        document.getElementById('ep-title').innerText = title;
        document.getElementById('ep-meta').innerText = '';

        // Image Handling with Reset
        const pbThumb = document.getElementById('pb-thumb');
        const epThumb = document.getElementById('ep-art-img');
        
        // Reset opacity to 0 initially
        pbThumb.style.opacity = '0';
        epThumb.style.opacity = '0';
        
        if (thumb) {
            // If we have a thumbnail URL, set it and when loaded, show it
            pbThumb.src = thumb;
            epThumb.src = thumb;
            pbThumb.onload = () => pbThumb.style.opacity = '1';
            epThumb.onload = () => epThumb.style.opacity = '1';
        } 
        
        // Set Fallback Icons
        const iconMap = { 'audio': 'music', 'video': 'film', 'image': 'image' };
        const icon = iconMap[item.type] || 'file';
        document.getElementById('pb-fallback-icon').setAttribute('data-lucide', icon);
        document.getElementById('ep-fallback-icon').setAttribute('data-lucide', icon);
        
        // Update VLC deep link for mobile playback
        const copyBtn = document.getElementById('ep-copy-url');
        if (copyBtn) {
            if (item.type === 'audio' || item.type === 'video') {
                const contentUrl = API.getContentUrl(item.path, state.mode);
                const origin = `${window.location.protocol}//${window.location.host}`;
                const absoluteUrl = `${origin}${contentUrl}`;
                copyBtn.setAttribute('data-url', absoluteUrl);
                copyBtn.classList.remove('hidden');
            } else {
                copyBtn.removeAttribute('data-url');
                copyBtn.classList.add('hidden');
            }
        }
        
        // Show/hide seek bar elements for images (keep controls visible)
        const seekBarContainer = document.getElementById('ep-seek-container');
        const bottomSection = seekBarContainer.closest('.bg-mantle');
        const desktopControls = document.getElementById('ep-desktop-controls');
        const mobileSeek = document.getElementById('ep-mobile-seek');
        const mobileControls = document.getElementById('ep-mobile-controls');
        const desktopPlaylistBtn = document.getElementById('ep-desktop-playlist');
        const timeCurr = document.getElementById('ep-time-curr');
        const timeTotal = document.getElementById('ep-time-total');
        const rangeWrapper = document.getElementById('ep-range-wrapper');
        
        if (item.type === 'image') {
            // Hide seek bar elements but keep controls visible
            seekBarContainer.classList.remove('hidden');
            // Hide time displays and range wrapper in desktop view
            if (timeCurr) timeCurr.classList.add('hidden');
            if (timeTotal) timeTotal.classList.add('hidden');
            if (rangeWrapper) rangeWrapper.classList.add('hidden');
            // Hide mobile seek bar completely
            if (mobileSeek) mobileSeek.classList.add('hidden');
            // Mobile controls should remain visible (they have 'flex md:hidden' in HTML)
            // Just ensure 'hidden' class is not present
            if (mobileControls) mobileControls.classList.remove('hidden');
            // Desktop controls use 'hidden md:flex' - DO NOT remove 'hidden' class
            // as that would make them visible on mobile (they'd have no display class)
            // Desktop playlist button uses 'hidden md:block' - same principle
            
            bottomSection.classList.remove('pb-6', 'pt-4');
            bottomSection.classList.add('pb-4', 'pt-2');
        } else {
            // Show all elements
            seekBarContainer.classList.remove('hidden');
            if (timeCurr) timeCurr.classList.remove('hidden');
            if (timeTotal) timeTotal.classList.remove('hidden');
            if (rangeWrapper) rangeWrapper.classList.remove('hidden');
            // Mobile seek bar has 'flex md:hidden' - just ensure 'hidden' is not present
            if (mobileSeek) mobileSeek.classList.remove('hidden');
            // Mobile controls have 'flex md:hidden' - just ensure 'hidden' is not present
            if (mobileControls) mobileControls.classList.remove('hidden');
            // Desktop controls use 'hidden md:flex' - DO NOT remove 'hidden' class
            // Desktop playlist button uses 'hidden md:block' - DO NOT remove 'hidden' class
            
            bottomSection.classList.remove('pb-4', 'pt-2');
            bottomSection.classList.add('pb-6', 'pt-4');
        }
        
        this.renderQueueList();
        this.refreshIcons();
    },
    
    renderQueueList() {
        const list = document.getElementById('queue-list-container');
        if (list) {
            list.innerHTML = Player.queue.map((item, idx) => 
                Elements.createQueueItem(item, idx === Player.currentIndex)
            ).join('');
            this.refreshIcons();
        }
    },
    
    toggleQueueDialog() {
        const dialog = document.getElementById('queue-dialog');
        if (dialog.classList.contains('hidden')) {
            dialog.classList.remove('hidden');
            this.renderQueueList();
        } else {
            dialog.classList.add('hidden');
        }
    },
    
    updateProgress(current, duration) {
        if (!duration) return;
        const percent = (current / duration) * 100;
        document.getElementById('pb-progress').style.width = `${percent}%`;
        
        // Update desktop range input and fill
        const range = document.getElementById('ep-range');
        if (document.activeElement !== range) {
            range.value = percent;
        }
        document.getElementById('ep-range-fill').style.width = `${percent}%`;
        
        // Update mobile range input and fill
        const rangeMob = document.getElementById('ep-range-mob');
        if (document.activeElement !== rangeMob) {
            rangeMob.value = percent;
        }
        document.getElementById('ep-range-fill-mob').style.width = `${percent}%`;
        
        const fmt = (t) => {
            if (isNaN(t)) return "0:00";
            const m = Math.floor(t / 60);
            const s = Math.floor(t % 60).toString().padStart(2, '0');
            return `${m}:${s}`;
        };
        
        const timeStr = fmt(current);
        const totalStr = fmt(duration);
        
        // Update desktop time displays
        document.getElementById('ep-time-curr').innerText = timeStr;
        document.getElementById('ep-time-total').innerText = totalStr;
        
        // Update mobile time displays
        document.getElementById('ep-time-curr-mob').innerText = timeStr;
        document.getElementById('ep-time-total-mob').innerText = totalStr;
    },

    toggleFullscreen() {
        if (!Player.queue.length) return;
        const item = Player.queue[Player.currentIndex];
        
        if (item.type === 'video') {
            // For videos, use native fullscreen API
            const videoEl = document.getElementById('ep-video');
            if (videoEl && !videoEl.classList.contains('hidden')) {
                if (videoEl.requestFullscreen) {
                    videoEl.requestFullscreen();
                } else if (videoEl.webkitRequestFullscreen) {
                    videoEl.webkitRequestFullscreen();
                } else if (videoEl.mozRequestFullScreen) {
                    videoEl.mozRequestFullScreen();
                } else if (videoEl.msRequestFullscreen) {
                    videoEl.msRequestFullscreen();
                }
            }
        } else if (item.type === 'image') {
            // For images, use our custom fullscreen container
            const container = document.getElementById('fullscreen-image-container');
            const fullscreenImg = document.getElementById('fullscreen-image');
            
            if (container.classList.contains('hidden')) {
                // Enter fullscreen - use the same source as the expanded image
                const src = API.getContentUrl(item.path, state.mode);
                fullscreenImg.src = src;
                container.classList.remove('hidden');
                
                // Request fullscreen on the container
                if (container.requestFullscreen) {
                    container.requestFullscreen();
                } else if (container.webkitRequestFullscreen) {
                    container.webkitRequestFullscreen();
                } else if (container.mozRequestFullScreen) {
                    container.mozRequestFullScreen();
                } else if (container.msRequestFullscreen) {
                    container.msRequestFullscreen();
                }
            } else {
                // Exit fullscreen
                this.exitFullscreen();
            }
        }
    },

    exitFullscreen() {
        const container = document.getElementById('fullscreen-image-container');
        
        if (document.fullscreenElement || document.webkitFullscreenElement || 
            document.mozFullScreenElement || document.msFullscreenElement) {
            if (document.exitFullscreen) {
                document.exitFullscreen();
            } else if (document.webkitExitFullscreen) {
                document.webkitExitFullscreen();
            } else if (document.mozCancelFullScreen) {
                document.mozCancelFullScreen();
            } else if (document.msExitFullscreen) {
                document.msExitFullscreen();
            }
        }
        
        container.classList.add('hidden');
    },

    handleFullscreenChange() {
        const container = document.getElementById('fullscreen-image-container');
        const isFullscreen = !!(document.fullscreenElement || document.webkitFullscreenElement || 
                               document.mozFullScreenElement || document.msFullscreenElement);
        
        if (!isFullscreen && !container.classList.contains('hidden')) {
            // User exited fullscreen (e.g., via escape key)
            container.classList.add('hidden');
        }
    }
};

export default UI;
