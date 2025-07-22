document.addEventListener('DOMContentLoaded', () => {
    // STATE
    let currentPath = '';
    let currentDirectoryContent = { images: [], videos: [], audios: [], others: [] };
    let currentModalIndex = -1;
    let currentMediaList = [];
    let player;

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
            mainContent.innerHTML = `<p class="error">Error loading content: ${error.message}</p>`;
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
            renderSearchResults(results);
        } catch (error) {
            mainContent.innerHTML = `<p class="error">Error searching: ${error.message}</p>`;
        }
    }

    async function triggerSync() {
        logoEl.classList.add('syncing');
        try {
            const response = await fetch('/api/sync', {
                method: 'POST'
            });
            if (!response.ok) {
                throw new Error('Sync failed');
            }
            await fetchData(currentPath);
        } catch (error) {
            console.error('Error triggering sync:', error);
        } finally {
            setTimeout(() => {
                logoEl.classList.remove('syncing');
            }, 1000);
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
                if (['zip', 'rar', '7z', 'tar', 'gz'].includes(extension)) {
                    return 'fa-file-archive';
                }
                return 'fa-file';
        }
    }

    function renderContent(data) {
        breadcrumbsEl.innerHTML = renderBreadcrumbs(data.breadcrumbs);
        mainContent.innerHTML = `
            ${(data.folders || []).length > 0 ? renderSection('Folders', renderFolderItems(data.folders), 'folders') : ''}
            ${(data.images || []).length > 0 ? renderSection('Images', renderMediaItems(data.images, 'image'), 'images') : ''}
            ${(data.videos || []).length > 0 ? renderSection('Videos', renderMediaItems(data.videos, 'video'), 'videos') : ''}
            ${(data.audios || []).length > 0 ? renderSection('Audio', renderMediaItems(data.audios, 'audio'), 'audios') : ''}
            ${(data.others || []).length > 0 ? renderSection('Other Files', renderOtherItems(data.others), 'others') : ''}
        `;
    }

    function renderBreadcrumbs(breadcrumbs) {
        return breadcrumbs.map((crumb, index) => {
            const isLast = index === breadcrumbs.length - 1;
            if (isLast) {
                return `<span>${crumb.name}</span>`;
            }
            return `<a href="#" data-path="${crumb.path}">${crumb.name}</a><span class="separator"> / </span>`;
        }).join('');
    }

    function renderSection(title, content, type) {
        return `
            <section class="section">
                <h2 class="section-title">${title}</h2>
                <div class="grid ${type}">${content}</div>
            </section>
        `;
    }

    function renderFolderItems(folders) {
        return folders.map(folder => `
            <div class="item" data-path="${folder.path}" data-type="folder">
                <div class="item-icon"><i class="fas fa-folder"></i></div>
                <p class="item-name">${folder.name}</p>
            </div>
        `).join('');
    }

    function renderMediaItems(items, type) {
        return items.map((item, index) => {
            const images = currentDirectoryContent.images || [];
            const videos = currentDirectoryContent.videos || [];
            let mediaIndex = 0;

            if (type === 'image') {
                mediaIndex = index;
            } else if (type === 'video') {
                mediaIndex = images.length + index;
            } else if (type === 'audio') {
                mediaIndex = images.length + videos.length + index;
            }

            const thumbnailSrc = item.thumbnailPath ? `/media/${item.thumbnailPath}` : (type === 'image' ? `/media/${item.path}` : null);

            let contentHtml;
            if (thumbnailSrc) {
                contentHtml = `<div class="item-thumbnail"><img src="${thumbnailSrc}" alt="${item.name}" loading="lazy"></div>`;
            } else {
                const iconClass = getFileIcon(item);
                contentHtml = `<div class="item-icon"><i class="fas ${iconClass}"></i></div>`;
            }

            return `
                <div class="item media-item" data-full-path="${item.path}" data-media-index="${mediaIndex}" data-type="${type}">
                    ${contentHtml}
                    <p class="item-name">${item.name}</p>
                </div>
            `;
        }).join('');
    }
    
    function renderOtherItems(items) {
        return items.map(item => {
            const iconClass = getFileIcon(item);
            return `
                <a href="/media/${item.path}" target="_blank" class="item">
                    <div class="item-icon"><i class="fas ${iconClass}"></i></div>
                    <p class="item-name">${item.name}</p>
                </a>
            `;
        }).join('');
    }

    function renderSearchResults(results) {
        breadcrumbsEl.innerHTML = `<span>Search Results</span>`;
        if (results.length === 0) {
            mainContent.innerHTML = `<p>No results found.</p>`;
            return;
        }
       
        const resultItems = results.map(item => {
            const parentPath = item.path.substring(0, item.path.lastIndexOf('/') || 0);
            const thumbnailSrc = item.thumbnailPath ? `/media/${item.thumbnailPath}` : null;
            const iconClass = getFileIcon(item);

            let thumbnailHtml;
            if (thumbnailSrc) {
                thumbnailHtml = `<div class="item-thumbnail"><img src="${thumbnailSrc}" alt="${item.name}" loading="lazy"></div>`;
            } else {
                thumbnailHtml = `<div class="item-icon"><i class="fas ${iconClass}"></i></div>`;
            }

            return `
                <div class="item search-result-item" data-path="${parentPath}" data-type="folder">
                    ${thumbnailHtml}
                    <p class="item-name">${item.path}</p>
                </div>
            `;
        }).join('');

        mainContent.innerHTML = `
            <section class="section">
                <h2 class="section-title">Results</h2>
                <div class="grid">${resultItems}</div>
            </section>
        `;
    }


    // MODAL LOGIC
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
            modalContentContainer.innerHTML = `<img src="${mediaUrl}" alt="${item.name}">`;
        } else if (mediaType === 'video') {
            modalContentContainer.innerHTML = `<video id="modal-video-player" playsinline controls><source src="${mediaUrl}" /></video>`;
            player = new Plyr('#modal-video-player', { autoplay: true });
        } else if (mediaType === 'audio') {
            modalContentContainer.innerHTML = `
                <div class="audio-player-container">
                    <i class="fas fa-music audio-backdrop-icon"></i>
                    <p class="audio-title">${item.name}</p>
                    <audio id="modal-audio-player" controls>
                        <source src="${mediaUrl}" />
                    </audio>
                </div>
            `;
            player = new Plyr('#modal-audio-player', { autoplay: true });
        }

        modalDownloadBtn.href = mediaUrl;
        modalDownloadBtn.setAttribute('download', item.name);
        modalRawBtn.href = mediaUrl;

        modal.style.display = 'flex';
        document.body.style.overflow = 'hidden';
        updateModalNav();
    }

    function closeModal() {
        if (player) {
            player.destroy();
            player = null;
        }
        modal.style.display = 'none';
        document.body.style.overflow = 'auto';
        modalContentContainer.innerHTML = '';
    }

    function updateModalNav() {
        modalPrevBtn.disabled = currentModalIndex <= 0;
        modalNextBtn.disabled = currentModalIndex >= currentMediaList.length - 1;
    }

    function showNextMedia() {
        if (!modalNextBtn.disabled) {
            openModal(currentModalIndex + 1);
        }
    }

    function showPrevMedia() {
        if (!modalPrevBtn.disabled) {
            openModal(currentModalIndex - 1);
        }
    }

    // EVENT LISTENERS
    mainContent.addEventListener('click', (e) => {
        const item = e.target.closest('.item');
        if (!item || item.tagName === 'A') return;

        const { path, type, mediaIndex } = item.dataset;

        if (type === 'folder') {
            fetchData(path);
        } else if (mediaIndex) {
            openModal(mediaIndex);
        }
    });

    breadcrumbsEl.addEventListener('click', (e) => {
        if (e.target.tagName === 'A') {
            e.preventDefault();
            fetchData(e.target.dataset.path);
        }
    });

    let searchTimeout;
    searchInput.addEventListener('input', () => {
        clearTimeout(searchTimeout);
        searchTimeout = setTimeout(() => {
            searchFiles(searchInput.value.trim());
        }, 300);
    });

    modalCloseBtn.addEventListener('click', closeModal);
    modal.addEventListener('click', (e) => {
        if (e.target === modal) closeModal();
    });
    modalPrevBtn.addEventListener('click', showPrevMedia);
    modalNextBtn.addEventListener('click', showNextMedia);

    document.addEventListener('keydown', (e) => {
        if (modal.style.display !== 'flex') return;
        if (e.key === 'Escape') closeModal();
        if (e.key === 'ArrowLeft') showPrevMedia();
        if (e.key === 'ArrowRight') showNextMedia();
    });
    
    logoEl.addEventListener('click', () => {
        triggerSync();
    });

    // INITIAL LOAD
    fetchData('');
});
