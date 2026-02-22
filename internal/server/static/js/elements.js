import API from './api.js';
import state from './state.js';
import Escape from './escape.js';

const Elements = {
    getIconName(type) {
        const map = {
            'folder': 'folder',
            'audio': 'music',
            'video': 'film',
            'image': 'image',
            'pdf': 'file-text',
            'text': 'align-left'
        };
        return map[type] || 'file';
    },

    getDisplayType(item) {
        if (state.mode === 'music') {
            const depth = state.path.split('/').filter(p => p).length;
            if (item.type === 'audio') return 'Track';
            if (item.type === 'folder') {
                if (depth === 0) return 'Artist';
                if (depth === 1) return 'Album';
                return 'Folder';
            }
        }
        const map = {
            'folder': 'Folder',
            'audio': 'Audio',
            'video': 'Video',
            'image': 'Image',
            'pdf': 'PDF',
            'text': 'Document'
        };
        return map[item.type] || 'File';
    },

    createGridItem(item) {
        const isMedia = ['image', 'video', 'audio'].includes(item.type);
        const iconName = this.getIconName(item.type);
        const thumbUrl = item.thumb ? API.getContentUrl(item.thumb, state.mode) : null;
        const origUrl = API.getContentUrl(item.path, state.mode);
        
        // Fallback Logic:
        // 1. If thumb exists, try it.
        // 2. On error:
        //    - If it's an image file, switch src to the original file (data-try-orig flag).
        //    - If that fails or it's not an image, hide img to show the background icon.
        
        let visual = `
            <div class="w-full h-full flex items-center justify-center text-overlay0 bg-surface0/50 absolute inset-0 z-0">
                <i data-lucide="${iconName}" size="32"></i>
            </div>
        `;

        // If thumbUrl provided, or it's an image (so we can try original), render img tag
        if (thumbUrl || item.type === 'image') {
            const src = thumbUrl || origUrl;
            // Use data attributes and event delegation instead of inline onerror to prevent XSS
            const escapedOrigUrl = Escape.attr(origUrl);
            const escapedType = Escape.attr(item.type);
            visual += `
                <img src="${Escape.attr(src)}" 
                     class="w-full h-full object-cover transition-transform duration-500 group-hover:scale-110 z-10 relative bg-surface0" 
                     loading="lazy" 
                     data-try-orig="${!thumbUrl && item.type === 'image'}"
                     data-orig-url="${escapedOrigUrl}"
                     data-item-type="${escapedType}">
            `;
        }

        if (item.type === 'folder' && !item.thumb) {
             visual = `<div class="w-full h-full bg-surface0/50 flex items-center justify-center text-blue relative z-10"><i data-lucide="${iconName}" size="40"></i></div>`;
        }

        return `
            <div class="flex flex-col gap-2 group w-full select-none"
                data-id="${Escape.attr(item.path)}" data-type="${Escape.attr(item.type)}" data-name="${Escape.attr(item.name.toLowerCase())}">
                
                <div class="aspect-square w-full rounded-xl overflow-hidden bg-surface0/20 shadow-sm border border-transparent group-hover:border-surface1 group-hover:bg-surface0 transition-all cursor-pointer relative isolate">
                    ${visual}
                    ${isMedia ? `<div class="absolute inset-0 bg-black/20 hidden group-hover:flex items-center justify-center z-20"><i data-lucide="play" class="fill-white text-white drop-shadow-lg" size="24"></i></div>` : ''}
                </div>

                <div class="px-1 min-w-0">
                    <p class="text-xs font-bold text-text truncate group-hover:text-mauve transition-colors">${Escape.html(item.name)}</p>
                    <p class="text-[10px] text-subtext0 truncate mt-0.5">${Escape.html(item.size || '')}</p>
                </div>
            </div>
        `;
    },

    createListItem(item) {
        const iconName = this.getIconName(item.type);
        const typeLabel = this.getDisplayType(item);
        const size = item.type === 'folder' ? '—' : (item.size || '—');
        const modified = item.modified || '—';
        
        return `
            <div class="group w-full"
                data-id="${Escape.attr(item.path)}" data-type="${Escape.attr(item.type)}" data-name="${Escape.attr(item.name.toLowerCase())}">
                <div class="grid grid-cols-[auto,1fr] md:grid-cols-[auto,1fr,120px,100px,160px] items-center gap-3 px-3 py-3 rounded-lg hover:bg-surface0/80 transition-colors cursor-pointer border border-transparent hover:border-surface0/40 select-none">
                    <div class="w-6 h-6 flex items-center justify-center text-subtext0 group-hover:text-mauve shrink-0">
                        <i data-lucide="${iconName}" size="20"></i>
                    </div>
                    <div class="flex flex-col min-w-0">
                        <p class="text-sm font-semibold text-text truncate group-hover:text-mauve transition-colors">${Escape.html(item.name)}</p>
                        <div class="flex items-center gap-2 text-[11px] text-subtext0 md:hidden">
                            <span>${Escape.html(typeLabel)}</span>
                            ${item.type !== 'folder' ? `<span class="text-overlay0">•</span><span>${Escape.html(size)}</span>` : ''}
                        </div>
                    </div>
                    <span class="hidden md:block text-xs text-subtext0 font-semibold uppercase tracking-wide">${Escape.html(typeLabel)}</span>
                    <span class="hidden md:block text-xs text-subtext0">${Escape.html(size)}</span>
                    <span class="hidden md:block text-xs text-subtext0">${Escape.html(modified)}</span>
                </div>
            </div>
        `;
    },
    
    createQueueItem(item, isActive, idx) {
        return `
            <div class="flex items-center gap-3 p-2 rounded hover:bg-surface0/50 cursor-pointer ${isActive ? 'bg-surface0 text-mauve' : 'text-subtext1'}" data-queue-index="${Escape.attr(idx)}">
                <i data-lucide="${isActive ? 'bar-chart-2' : 'play'}" size="14"></i>
                <div class="flex-1 truncate text-sm">${Escape.html(item.name)}</div>
                <button class="p-1 rounded hover:bg-surface0/70 text-red hover:text-red shrink-0" data-queue-remove="${Escape.attr(idx)}" aria-label="Remove from queue">
                    <i data-lucide="x" size="14"></i>
                </button>
            </div>
        `;
    },
    
    createHistoryItem(path, idx) {
        return `
            <div class="flex items-center gap-3 p-2 rounded hover:bg-surface0/50 text-subtext1">
                <i data-lucide="film" size="14"></i>
                <div class="flex-1 truncate text-sm">${Escape.html(path)}</div>
            </div>
        `;
    }
};

export default Elements;
