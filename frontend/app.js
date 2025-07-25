document.addEventListener('DOMContentLoaded', () => {
    // STATE
    let currentPath = '';
    let currentDirectoryContent = { images: [], videos: [], audios: [], others: [] };
    let currentModalIndex = -1;
    let currentMediaList = [];
    let player;
    let navAutoHideTimer = null;

    // DOM ELEMENTS
    const mainContent = document.getElementById('main-content');
    const breadcrumbsEl = document.getElementById('breadcrumbs');
    const searchInput = document.getElementById('search-input');
    const modal = document.getElementById('media-modal');
    const modalContentContainer = document.getElementById('modal-content-container');
    const modalCloseBtn = document.getElementById('modal-close-btn');
    const modalPrevBtn = document.getElementById('modal-prev-btn');
    const modalNextBtn = document.getElementById('modal-next-btn');
    const modalDownloadBtn = document.getElementById('modal-download-btn');
    const modalRawBtn = document.getElementById('modal-raw-btn');
    const logoEl = document.querySelector('.logo');
    const modalHeader = document.getElementById('modal-header');
    const modalNavControls = [modalPrevBtn, modalNextBtn, modalHeader];


    // API FUNCTIONS
    async function fetchData(path) {
        try {
            const response = await fetch(`/api/browse/${path}`);
            if (!response.ok) throw new Error('Network response was not ok');
            const data = await response.json();
            currentPath = path;
            currentDirectoryContent = data;
            renderContent(data);
        } catch (error) {
            mainContent.innerHTML = `<p class="text-red text-center">Error loading content: ${error.message}</p>`;
        }
    }

    async function searchFiles(query) {
        if (!query) {
            fetchData(currentPath);
            return;
        }
        try {
            const response = await fetch(`/api/search?q=${encodeURIComponent(query)}`);
            if (!response.ok) throw new Error('Network response was not ok');
            const results = await response.json();
            renderSearchResults(results || []);
        } catch (error) {
            mainContent.innerHTML = `<p class="text-red text-center">Error searching: ${error.message}</p>`;
        }
    }

    async function triggerSync() {
        logoEl.classList.add('syncing');
        try {
            const response = await fetch('/api/sync', { method: 'POST' });
            if (!response.ok) throw new Error('Sync failed');
            await fetchData(currentPath);
        } catch (error) {
            console.error('Error triggering sync:', error);
        } finally {
            setTimeout(() => logoEl.classList.remove('syncing'), 1000);
        }
    }

    // RENDER FUNCTIONS
    function getFileIcon(item) {
        switch (item.type) {
            case 'image': return 'fa-file-image';
            case 'video': return 'fa-file-video';
            case 'audio': return 'fa-file-audio';
            case 'pdf': return 'fa-file-pdf';
            case 'text': return 'fa-file-alt';
            default:
                const extension = item.name.split('.').pop().toLowerCase();
                if (['zip', 'rar', '7z', 'tar', 'gz'].includes(extension)) return 'fa-file-archive';
                return 'fa-file';
        }
    }

    function renderContent(data) {
        breadcrumbsEl.innerHTML = renderBreadcrumbs(data.breadcrumbs);
        mainContent.innerHTML = `
            ${(data.folders || []).length > 0 ? renderSection('Folders', renderFolderItems(data.folders)) : ''}
            ${(data.images || []).length > 0 ? renderSection('Images', renderMediaItems(data.images, 'image')) : ''}
            ${(data.videos || []).length > 0 ? renderSection('Videos', renderMediaItems(data.videos, 'video')) : ''}
            ${(data.audios || []).length > 0 ? renderSection('Audio', renderMediaItems(data.audios, 'audio')) : ''}
            ${(data.others || []).length > 0 ? renderSection('Other Files', renderOtherItems(data.others)) : ''}
        `;
    }

    function renderBreadcrumbs(breadcrumbs) {
        const homeIcon = `<a href="#" data-path="" class="hover:text-mauve transition-colors"><i class="fas fa-home"></i></a>`;
        if (breadcrumbs.length <= 1) {
            return homeIcon;
        }
        const separator = `<span class="separator text-overlay0 mx-2"><i class="fas fa-chevron-right text-xs"></i></span>`;
        const pathLinks = breadcrumbs.slice(1).map((crumb, index) => {
            const isLast = index === breadcrumbs.length - 2;
            return isLast 
                ? `<span class="text-text font-semibold">${crumb.name}</span>`
                : `<a href="#" data-path="${crumb.path}" class="hover:text-mauve transition-colors">${crumb.name}</a>`;
        }).join(separator);
        return `${homeIcon}${separator}${pathLinks}`;
    }

    function renderSection(title, content) {
        return `
            <section class="mb-12">
                <div class="text-center mb-6">
                    <h2 class="text-sm font-bold uppercase tracking-widest text-subtext1">${title}</h2>
                </div>
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">${content}</div>
            </section>
        `;
    }

    function renderFolderItems(folders) {
        return folders.map(folder => `
            <div class="item flex items-center gap-4 bg-base p-2 rounded-lg hover:bg-surface0 transition-colors cursor-pointer" data-path="${folder.path}" data-type="folder">
                <div class="flex-shrink-0 w-12 h-12 flex items-center justify-center bg-surface1 rounded-md text-mauve">
                    <i class="fas fa-folder fa-lg"></i>
                </div>
                <p class="flex-grow font-medium truncate" title="${folder.name}">${folder.name}</p>
            </div>
        `).join('');
    }

    function renderMediaItems(items, type) {
        return items.map((item, index) => {
            const images = currentDirectoryContent.images || [];
            const videos = currentDirectoryContent.videos || [];
            let mediaIndex = type === 'image' ? index : (type === 'video' ? images.length + index : images.length + videos.length + index);

            const thumbnailSrc = item.thumbnailPath ? `/media/${item.thumbnailPath}` : (type === 'image' ? `/media/${item.path}` : null);
            const iconClass = getFileIcon(item);

            let contentHtml = thumbnailSrc
                ? `<img src="${thumbnailSrc}" alt="${item.name}" loading="lazy" class="w-full h-full object-cover">`
                : `<div class="w-full h-full flex items-center justify-center text-mauve"><i class="fas ${iconClass} fa-lg"></i></div>`;

            return `
                <div class="item media-item flex items-center gap-4 bg-base p-2 rounded-lg hover:bg-surface0 transition-colors cursor-pointer" data-media-index="${mediaIndex}" data-type="${type}">
                    <div class="flex-shrink-0 w-16 h-16 bg-surface1 rounded-md overflow-hidden">${contentHtml}</div>
                    <p class="flex-grow font-medium truncate" title="${item.name}">${item.name}</p>
                </div>
            `;
        }).join('');
    }
    
    function renderOtherItems(items) {
        return items.map(item => {
            const iconClass = getFileIcon(item);
            return `
                <a href="/media/${item.path}" target="_blank" class="item flex items-center gap-4 bg-base p-2 rounded-lg hover:bg-surface0 transition-colors">
                    <div class="flex-shrink-0 w-12 h-12 flex items-center justify-center bg-surface1 rounded-md text-mauve">
                        <i class="fas ${iconClass} fa-lg"></i>
                    </div>
                    <p class="flex-grow font-medium truncate" title="${item.name}">${item.name}</p>
                </a>
            `;
        }).join('');
    }

    function renderSearchResults(results) {
        breadcrumbsEl.innerHTML = `<span class="text-text font-semibold">Search Results</span>`;
        if (results.length === 0) {
            mainContent.innerHTML = `<p class="text-subtext0 text-center">No results found.</p>`;
            return;
        }
       
        const resultItems = results.map(item => {
            const parentPath = item.path.substring(0, item.path.lastIndexOf('/') || 0);
            const thumbnailSrc = item.thumbnailPath ? `/media/${item.thumbnailPath}` : null;
            const iconClass = getFileIcon(item);

            let thumbnailHtml = thumbnailSrc
                ? `<img src="${thumbnailSrc}" alt="${item.name}" loading="lazy" class="w-full h-full object-cover">`
                : `<div class="w-full h-full flex items-center justify-center text-mauve"><i class="fas ${iconClass} fa-lg"></i></div>`;

            return `
                <div class="item search-result-item flex items-center gap-4 bg-base p-2 rounded-lg hover:bg-surface0 transition-colors cursor-pointer" data-path="${parentPath}" data-type="folder">
                    <div class="flex-shrink-0 w-16 h-16 bg-surface1 rounded-md overflow-hidden">${thumbnailHtml}</div>
                    <p class="flex-grow font-medium text-sm text-subtext1 truncate" title="${item.path}">${item.path}</p>
                </div>
            `;
        }).join('');

        mainContent.innerHTML = `
            <section>
                <div class="text-center mb-6">
                    <h2 class="text-sm font-bold uppercase tracking-widest text-subtext1">Results</h2>
                </div>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">${resultItems}</div>
            </section>
        `;
    }

    // MODAL LOGIC
    function showModalControls() {
        modalNavControls.forEach(el => el.classList.remove('opacity-0'));
        clearTimeout(navAutoHideTimer);
        navAutoHideTimer = setTimeout(() => {
            modalNavControls.forEach(el => el.classList.add('opacity-0'));
        }, 3000);
    }

    function openModal(mediaIndex) {
        currentMediaList = [
            ...(currentDirectoryContent.images || []),
            ...(currentDirectoryContent.videos || []),
            ...(currentDirectoryContent.audios || [])
        ];
        currentModalIndex = parseInt(mediaIndex, 10);
        if (currentModalIndex < 0 || currentModalIndex >= currentMediaList.length) return;
        const item = currentMediaList[currentModalIndex];
        const mediaType = item.type;
        const mediaUrl = `/media/${item.path}`;
        modalContentContainer.innerHTML = '';
        if (player) {
            player.destroy();
            player = null;
        }

        if (mediaType === 'image') {
            modalContentContainer.innerHTML = `<img src="${mediaUrl}" alt="${item.name}" class="max-w-full max-h-full object-contain">`;
        } else if (mediaType === 'video') {
            modalContentContainer.innerHTML = `<video id="modal-video-player" playsinline controls class="w-full h-full"><source src="${mediaUrl}" /></video>`;
            player = new Plyr('#modal-video-player', { autoplay: true });
        } else if (mediaType === 'audio') {
            modalContentContainer.innerHTML = `
                <div class="flex flex-col items-center justify-center text-text w-full max-w-md mx-auto p-4">
                    <i class="fas fa-music text-9xl text-overlay0 mb-6"></i>
                    <p class="text-xl font-semibold mb-4 text-center">${item.name}</p>
                    <audio id="modal-audio-player" controls class="w-full"><source src="${mediaUrl}" /></audio>
                </div>
            `;
            player = new Plyr('#modal-audio-player', { autoplay: true });
        }

        modalDownloadBtn.href = mediaUrl;
        modalDownloadBtn.setAttribute('download', item.name);
        modalRawBtn.href = mediaUrl;
        modal.classList.remove('hidden');
        document.body.style.overflow = 'hidden';
        updateModalNav();
        modal.addEventListener('mousemove', showModalControls);
        modal.addEventListener('click', showModalControls);
        showModalControls();
    }

    function closeModal() {
        if (player) {
            player.destroy();
            player = null;
        }
        modal.classList.add('hidden');
        document.body.style.overflow = 'auto';
        modalContentContainer.innerHTML = '';
        clearTimeout(navAutoHideTimer);
        modal.removeEventListener('mousemove', showModalControls);
        modal.removeEventListener('click', showModalControls);
    }

    function updateModalNav() {
        modalPrevBtn.disabled = currentModalIndex <= 0;
        modalNextBtn.disabled = currentModalIndex >= currentMediaList.length - 1;
    }

    function showNextMedia(e) {
        e.stopPropagation();
        if (!modalNextBtn.disabled) openModal(currentModalIndex + 1);
    }

    function showPrevMedia(e) {
        e.stopPropagation();
        if (!modalPrevBtn.disabled) openModal(currentModalIndex - 1);
    }

    // EVENT LISTENERS
    mainContent.addEventListener('click', (e) => {
        const item = e.target.closest('.item');
        if (!item || item.tagName === 'A') return;
        const { path, type, mediaIndex } = item.dataset;
        if (type === 'folder') fetchData(path);
        else if (mediaIndex) openModal(mediaIndex);
    });

    breadcrumbsEl.addEventListener('click', (e) => {
        const link = e.target.closest('a');
        if (link) {
            e.preventDefault();
            fetchData(link.dataset.path);
        }
    });

    let searchTimeout;
    searchInput.addEventListener('input', () => {
        clearTimeout(searchTimeout);
        searchTimeout = setTimeout(() => searchFiles(searchInput.value.trim()), 300);
    });

    modalCloseBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        closeModal();
    });
    modalPrevBtn.addEventListener('click', showPrevMedia);
    modalNextBtn.addEventListener('click', showNextMedia);

    document.addEventListener('keydown', (e) => {
        if (modal.classList.contains('hidden')) return;
        if (e.key === 'Escape') closeModal();
        if (e.key === 'ArrowLeft') {
             if (!modalPrevBtn.disabled) openModal(currentModalIndex - 1);
        }
        if (e.key === 'ArrowRight') {
            if (!modalNextBtn.disabled) openModal(currentModalIndex + 1);
        }
    });
    logoEl.addEventListener('click', triggerSync);

    // INITIAL LOAD
    fetchData('');
});
