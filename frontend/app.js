document.addEventListener('DOMContentLoaded', () => {
    // STATE
    let currentPath = '';
    let currentDirectoryContent = { images: [], videos: [], audios: [], others: [] };
    let currentModalIndex = -1;
    let currentMediaList = [];
    let isGalleryView = false;
    let player;
    let resyncInterval = null;
    let navAutoHideTimer = null;
    let slideshowTimer = null;
    let isSlideshowPlaying = false;
    let isSlideshowShuffle = false;
    let shuffleList = [];
    let currentShuffleIndex = -1;

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
    const modalPlayBtn = document.getElementById('modal-play-btn');
    const modalShuffleBtn = document.getElementById('modal-shuffle-btn');
    const logoEl = document.querySelector('.logo');
    const modalHeader = document.getElementById('modal-header');
    const modalNavControls = [modalPrevBtn, modalNextBtn, modalHeader];
    const galleryToggleBtn = document.getElementById('gallery-toggle-btn');

    // Upload Modal Elements
    const uploadBtn = document.getElementById('upload-btn');
    const uploadModal = document.getElementById('upload-modal');
    const uploadModalCloseBtn = document.getElementById('upload-modal-close-btn');
    const uploadForm = document.getElementById('upload-form');
    const fileInput = document.getElementById('file-input');
    const pathInput = document.getElementById('path-input');
    const filenameInput = document.getElementById('filename-input');
    const uploadSubmitBtn = document.getElementById('upload-submit-btn');
    const uploadSpinner = document.getElementById('upload-spinner');
    const uploadSubmitBtnText = uploadSubmitBtn.querySelector('span');
    const fileDropZone = document.getElementById('file-drop-zone');
    const fileDropZoneContent = document.getElementById('file-drop-zone-content');
    const fileList = document.getElementById('file-list');
    const uploadProgressContainer = document.getElementById('upload-progress-container');
    const uploadProgressBar = document.getElementById('upload-progress-bar');
    const uploadProgressText = document.getElementById('upload-progress-text');
    const uploadError = document.getElementById('upload-error');
    const pathError = document.getElementById('path-error');
    const notificationToast = document.getElementById('notification-toast');
    const notificationContent = document.getElementById('notification-content');
    const notificationIcon = document.getElementById('notification-icon');
    const notificationMessage = document.getElementById('notification-message');
    const notificationClose = document.getElementById('notification-close');


    // API FUNCTIONS
    async function fetchData(path, itemToHighlight = null) {
        try {
            const response = await fetch(`/api/browse/${path}`);
            if (!response.ok) throw new Error('Network response was not ok');
            const data = await response.json();
            currentPath = path;
            currentDirectoryContent = data;
            renderContent(data, itemToHighlight);
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
            case 'image': return 'fas fa-file-image';
            case 'video': return 'fas fa-file-video';
            case 'audio': return 'fas fa-file-audio';
            case 'pdf': return 'fas fa-file-pdf';
            case 'markdown': return 'fab fa-markdown';
            case 'text': return 'fas fa-file-lines';
            default:
                const extension = item.name.split('.').pop().toLowerCase();
                if (['zip', 'rar', '7z', 'tar', 'gz'].includes(extension)) return 'fas fa-file-archive';
                return 'fas fa-file';
        }
    }

    function highlightItem(itemName) {
        const escapedItemName = itemName.replace(/"/g, '\\"');
        const itemElement = mainContent.querySelector(`.item[data-name="${escapedItemName}"]`);
        if (itemElement) {
            itemElement.scrollIntoView({ behavior: 'smooth', block: 'center' });
            itemElement.classList.add('highlight-glow');
            setTimeout(() => {
                itemElement.classList.remove('highlight-glow');
            }, 2000);
        }
    }

    function renderContent(data, itemToHighlight = null) {
        breadcrumbsEl.innerHTML = renderBreadcrumbs(data.breadcrumbs);
        mainContent.innerHTML = `
            ${(data.folders || []).length > 0 ? renderSection('Folders', renderFolderItems(data.folders)) : ''}
            ${(data.images || []).length > 0 ? renderSection('Images', renderMediaItems(data.images, 'image')) : ''}
            ${(data.videos || []).length > 0 ? renderSection('Videos', renderMediaItems(data.videos, 'video')) : ''}
            ${(data.audios || []).length > 0 ? renderSection('Audio', renderOtherItems(data.audios)) : ''}
            ${(data.others || []).length > 0 ? renderSection('Other Files', renderOtherItems(data.others)) : ''}
        `;
        if (itemToHighlight) {
            // short timeout so DOM is fully updated
            setTimeout(() => highlightItem(itemToHighlight), 100);
        }
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
        const isMediaSection = title === 'Images' || title === 'Videos';
        const gridClasses = "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4";
        let containerClasses = isMediaSection ? `media-grid ${gridClasses}` : gridClasses;
        if (isMediaSection && isGalleryView) {
            containerClasses += ' gallery-view';
        }

        return `
            <section class="mb-12">
                <div class="text-center mb-6 flex items-center justify-center">
                    <h2 class="text-sm font-bold uppercase tracking-widest text-subtext1">${title}</h2>
                </div>
                <div class="${containerClasses}">${content}</div>
            </section>
        `;
    }

    function renderFolderItems(folders) {
        return folders.map(folder => `
            <div class="item flex items-center gap-4 bg-base p-2 rounded-lg hover:bg-surface0 transition-colors cursor-pointer" data-path="${folder.path}" data-name="${folder.name}" data-type="folder">
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
            const iconClass = getFileIcon({ type });
            const escapedName = item.name.replace(/"/g, '&quot;').replace(/'/g, '&#39;');

            let contentHtml = thumbnailSrc
                ? `<img src="${thumbnailSrc}" alt="${escapedName}" loading="lazy" class="w-full h-full object-cover" onerror="this.onerror=null; const parent=this.parentElement; parent.innerHTML='<div class=\\'w-full h-full flex items-center justify-center text-mauve\\'><i class=\\'${iconClass} fa-lg\\'></i></div>';" onload="this.style.opacity='1';" style="opacity:0;transition:opacity 0.3s;">`
                : `<div class="w-full h-full flex items-center justify-center text-mauve"><i class="${iconClass} fa-lg"></i></div>`;

            return `
                <div class="item media-item bg-base rounded-lg hover:bg-surface0 transition-colors cursor-pointer" data-media-index="${mediaIndex}" data-name="${escapedName}" data-type="${type}" role="button" tabindex="0" aria-label="View ${escapedName}">
                    <div class="media-thumbnail bg-surface1 rounded-md overflow-hidden">${contentHtml}</div>
                    <p class="media-name font-medium truncate" title="${escapedName}">${escapedName}</p>
                </div>
            `;
        }).join('');
    }
    
    function renderOtherItems(items) {
        return items.map(item => {
            const iconClasses = getFileIcon(item);
            const isMarkdown = item.type === 'markdown';
            // link to the markdown renderer if md otherwise link to the file
            const targetUrl = isMarkdown 
                ? `/render.html?path=${encodeURIComponent(item.path)}` 
                : `/media/${encodeURIComponent(item.path)}`;

            return `
                <a href="${targetUrl}" target="_blank" class="item flex items-center gap-4 bg-base p-2 rounded-lg hover:bg-surface0 transition-colors" data-name="${item.name}">
                    <div class="flex-shrink-0 w-12 h-12 flex items-center justify-center bg-surface1 rounded-md text-mauve">
                        <i class="${iconClasses} fa-lg"></i>
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

            const escapedSearchName = item.name.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
            let thumbnailHtml = thumbnailSrc
                ? `<img src="${thumbnailSrc}" alt="${escapedSearchName}" loading="lazy" class="w-full h-full object-cover" onerror="this.onerror=null; const parent=this.parentElement; parent.innerHTML='<div class=\\'w-full h-full flex items-center justify-center text-mauve\\'><i class=\\'${iconClass} fa-lg\\'></i></div>';" onload="this.style.opacity='1';" style="opacity:0;transition:opacity 0.3s;">`
                : `<div class="w-full h-full flex items-center justify-center text-mauve"><i class="${iconClass} fa-lg"></i></div>`;

            return `
                <div class="item search-result-item flex items-center gap-4 bg-base p-2 rounded-lg hover:bg-surface0 transition-colors cursor-pointer" data-path="${parentPath}" data-filename="${item.name}" data-type="folder">
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

    function startAutoResync(videoElementId) {
        if (resyncInterval) {
            clearInterval(resyncInterval);
            resyncInterval = null;
        }
        resyncInterval = setInterval(() => {
            if (!player || !player.playing) return;
            const videoElement = document.querySelector(videoElementId);
            if (!videoElement) return;
            const currentTime = videoElement.currentTime;
            videoElement.currentTime = currentTime;
            console.log('Auto-resync triggered at', currentTime);
        }, 120000);
    }

    function stopAutoResync() {
        if (resyncInterval) {
            clearInterval(resyncInterval);
            resyncInterval = null;
        }
    }

    function openModal(mediaIndex, skipShowControls = false) {
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
        stopAutoResync();

        if (mediaType === 'image') {
            modalContentContainer.innerHTML = `<img src="${mediaUrl}" alt="${item.name}" class="max-w-full max-h-full object-contain">`;
            modalPlayBtn.classList.remove('hidden');
            modalShuffleBtn.classList.remove('hidden');
        } else {
            modalPlayBtn.classList.add('hidden');
            modalShuffleBtn.classList.add('hidden');
            if (mediaType === 'video') {
                modalContentContainer.innerHTML = `<video id="modal-video-player" playsinline controls class="w-full h-full"><source src="${mediaUrl}" /></video>`;
                player = new Plyr('#modal-video-player', { autoplay: true });
                startAutoResync('#modal-video-player');
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
        }

        modalDownloadBtn.href = mediaUrl;
        modalDownloadBtn.setAttribute('download', item.name);
        modalRawBtn.href = mediaUrl;
        modal.classList.remove('hidden');
        document.body.style.overflow = 'hidden';
        updateModalNav();
        if (!skipShowControls) {
            modal.addEventListener('mousemove', showModalControls);
            modal.addEventListener('click', showModalControls);
            showModalControls();
        }
    }

    function closeModal() {
        stopSlideshow();
        stopAutoResync();
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
        stopSlideshow();
        if (!modalNextBtn.disabled) openModal(currentModalIndex + 1);
    }

    function showPrevMedia(e) {
        e.stopPropagation();
        stopSlideshow();
        if (!modalPrevBtn.disabled) openModal(currentModalIndex - 1);
    }

    function stopSlideshow() {
        if (slideshowTimer) {
            clearInterval(slideshowTimer);
            slideshowTimer = null;
        }
        isSlideshowPlaying = false;
        isSlideshowShuffle = false;
        shuffleList = [];
        currentShuffleIndex = -1;
        const playIcon = modalPlayBtn.querySelector('i');
        playIcon.classList.remove('fa-pause');
        playIcon.classList.add('fa-play');
        modalPlayBtn.title = 'Play Slideshow';
        const shuffleIcon = modalShuffleBtn.querySelector('i');
        shuffleIcon.classList.remove('fa-pause');
        shuffleIcon.classList.add('fa-random');
        modalShuffleBtn.title = 'Shuffle Slideshow';
    }

    function startSlideshow(shuffle = false) {
        const images = currentDirectoryContent.images || [];
        if (images.length === 0) return;

        stopSlideshow();
        isSlideshowPlaying = true;
        isSlideshowShuffle = shuffle;

        if (shuffle) {
            shuffleList = images.map((_, index) => index);
            for (let i = shuffleList.length - 1; i > 0; i--) {
                const j = Math.floor(Math.random() * (i + 1));
                [shuffleList[i], shuffleList[j]] = [shuffleList[j], shuffleList[i]];
            }
            const currentIndexInShuffle = shuffleList.indexOf(currentModalIndex);
            currentShuffleIndex = currentIndexInShuffle !== -1 ? currentIndexInShuffle : 0;
            const shuffleIcon = modalShuffleBtn.querySelector('i');
            shuffleIcon.classList.remove('fa-random');
            shuffleIcon.classList.add('fa-pause');
            modalShuffleBtn.title = 'Pause Slideshow';
        } else {
            const playIcon = modalPlayBtn.querySelector('i');
            playIcon.classList.remove('fa-play');
            playIcon.classList.add('fa-pause');
            modalPlayBtn.title = 'Pause Slideshow';
        }

        slideshowTimer = setInterval(() => {
            if (shuffle) {
                if (currentShuffleIndex >= shuffleList.length - 1) {
                    currentShuffleIndex = 0;
                } else {
                    currentShuffleIndex++;
                }
                openModal(shuffleList[currentShuffleIndex], true);
            } else {
                const nextIndex = (currentModalIndex + 1) % images.length;
                openModal(nextIndex, true);
            }
        }, 4000);
    }

    function toggleSlideshow() {
        if (isSlideshowPlaying) {
            stopSlideshow();
        } else {
            startSlideshow(false);
        }
    }

    function startShuffleSlideshow() {
        if (isSlideshowPlaying && isSlideshowShuffle) {
            stopSlideshow();
        } else {
            startSlideshow(true);
        }
    }

    // NOTIFICATION SYSTEM
    function showNotification(message, type = 'success') {
        const icons = {
            success: 'fas fa-check-circle text-green',
            error: 'fas fa-exclamation-circle text-red',
            info: 'fas fa-info-circle text-blue'
        };
        const colors = {
            success: 'border-green',
            error: 'border-red',
            info: 'border-blue'
        };
        notificationIcon.className = `text-xl ${icons[type] || icons.info}`;
        notificationMessage.textContent = message;
        notificationContent.className = `bg-base rounded-lg shadow-xl p-4 flex items-center gap-3 min-w-[300px] max-w-md border ${colors[type] || colors.info}`;
        notificationToast.classList.remove('hidden');
        setTimeout(() => {
            hideNotification();
        }, 5000);
    }

    function hideNotification() {
        notificationToast.classList.add('hidden');
    }

    // FILE VALIDATION
    function validateFile(file) {
        const maxSize = 500 * 1024 * 1024; // 500 MB
        if (file.size > maxSize) {
            return { valid: false, error: `File "${file.name}" exceeds maximum size of 500 MB` };
        }
        return { valid: true };
    }

    function validatePath(path) {
        if (!path || path.trim() === '') return { valid: true };
        // Check for invalid characters
        const invalidChars = /[<>:"|?*\x00-\x1f]/;
        if (invalidChars.test(path)) {
            return { valid: false, error: 'Path contains invalid characters' };
        }
        // Check for path traversal attempts
        if (path.includes('..')) {
            return { valid: false, error: 'Path cannot contain ".."' };
        }
        return { valid: true };
    }

    function formatFileSize(bytes) {
        if (bytes === 0) return '0 Bytes';
        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
    }

    function updateFileList() {
        if (!fileInput.files || fileInput.files.length === 0) {
            fileList.classList.add('hidden');
            fileDropZoneContent.classList.remove('hidden');
            return;
        }
        fileList.classList.remove('hidden');
        fileDropZoneContent.classList.add('hidden');
        fileList.innerHTML = Array.from(fileInput.files).map((file, index) => {
            const validation = validateFile(file);
            const escapedName = file.name.replace(/"/g, '&quot;');
            const escapedError = validation.error ? validation.error.replace(/"/g, '&quot;') : '';
            return `
                <div class="flex items-center justify-between p-2 bg-surface0 rounded ${!validation.valid ? 'border border-red' : ''}" role="listitem">
                    <div class="flex-grow min-w-0">
                        <p class="text-sm text-text truncate" title="${escapedName}">${escapedName}</p>
                        <p class="text-xs text-subtext0">${formatFileSize(file.size)}</p>
                    </div>
                    ${!validation.valid ? `<i class="fas fa-exclamation-triangle text-red ml-2" title="${escapedError}" aria-label="Error: ${escapedError}"></i>` : ''}
                </div>
            `;
        }).join('');
    }

    // UPLOAD MODAL LOGIC
    function openUploadModal() {
        pathInput.value = currentPath; // Pre-fill with current path
        uploadModal.classList.remove('hidden');
        uploadModal.classList.add('flex');
        document.body.style.overflow = 'hidden';
        uploadError.classList.add('hidden');
        pathError.classList.add('hidden');
        uploadError.textContent = '';
        pathError.textContent = '';
        // Focus on file input for accessibility
        setTimeout(() => fileDropZone.focus(), 100);
    }

    function closeUploadModal() {
        uploadForm.reset();
        uploadModal.classList.add('hidden');
        uploadModal.classList.remove('flex');
        document.body.style.overflow = 'auto';
        uploadSubmitBtn.disabled = false;
        uploadSpinner.classList.add('hidden');
        uploadSubmitBtnText.classList.remove('hidden');
        uploadProgressContainer.classList.add('hidden');
        uploadProgressBar.style.width = '0%';
        uploadProgressText.textContent = '0%';
        uploadError.classList.add('hidden');
        pathError.classList.add('hidden');
        fileList.classList.add('hidden');
        fileDropZoneContent.classList.remove('hidden');
    }

    async function handleUpload(e) {
        e.preventDefault();
        if (!fileInput.files || fileInput.files.length === 0) {
            uploadError.textContent = 'Please select a file to upload.';
            uploadError.classList.remove('hidden');
            return;
        }

        // Validate all files
        const invalidFiles = [];
        for (const file of fileInput.files) {
            const validation = validateFile(file);
            if (!validation.valid) {
                invalidFiles.push(validation.error);
            }
        }
        if (invalidFiles.length > 0) {
            uploadError.textContent = invalidFiles[0];
            uploadError.classList.remove('hidden');
            return;
        }

        // Validate path
        const pathValidation = validatePath(pathInput.value.trim());
        if (!pathValidation.valid) {
            pathError.textContent = pathValidation.error;
            pathError.classList.remove('hidden');
            return;
        }
        pathError.classList.add('hidden');

        uploadSubmitBtn.disabled = true;
        uploadSpinner.classList.remove('hidden');
        uploadSubmitBtnText.classList.add('hidden');
        uploadError.classList.add('hidden');
        uploadProgressContainer.classList.remove('hidden');
        uploadProgressBar.style.width = '0%';
        uploadProgressText.textContent = '0%';

        const formData = new FormData();
        for (const file of fileInput.files) {
            formData.append('file', file);
        }
        formData.append('path', pathInput.value.trim());
        if (fileInput.files.length === 1) {
            formData.append('filename', filenameInput.value.trim());
        }

        try {
            const xhr = new XMLHttpRequest();

            // Track upload progress
            xhr.upload.addEventListener('progress', (e) => {
                if (e.lengthComputable) {
                    const percentComplete = (e.loaded / e.total) * 100;
                    uploadProgressBar.style.width = `${percentComplete}%`;
                    uploadProgressText.textContent = `${Math.round(percentComplete)}%`;
                }
            });

            const response = await new Promise((resolve, reject) => {
                xhr.addEventListener('load', () => {
                    if (xhr.status >= 200 && xhr.status < 300) {
                        try {
                            resolve(JSON.parse(xhr.responseText));
                        } catch {
                            resolve({ message: 'Upload successful' });
                        }
                    } else {
                        try {
                            const errorData = JSON.parse(xhr.responseText);
                            reject(new Error(errorData.error || 'Upload failed'));
                        } catch {
                            reject(new Error(`Upload failed with status ${xhr.status}`));
                        }
                    }
                });

                xhr.addEventListener('error', () => {
                    reject(new Error('Network error occurred'));
                });

                xhr.addEventListener('abort', () => {
                    reject(new Error('Upload cancelled'));
                });

                xhr.open('POST', '/api/upload');
                xhr.send(formData);
            });

            showNotification(`Successfully uploaded ${fileInput.files.length} file(s)`, 'success');
            closeUploadModal();
            await triggerSync();
        } catch (error) {
            uploadError.textContent = error.message;
            uploadError.classList.remove('hidden');
            uploadProgressContainer.classList.add('hidden');
            showNotification(`Upload failed: ${error.message}`, 'error');
        } finally {
            uploadSubmitBtn.disabled = false;
            uploadSpinner.classList.add('hidden');
            uploadSubmitBtnText.classList.remove('hidden');
        }
    }

    // EVENT LISTENERS
    galleryToggleBtn.addEventListener('click', () => {
        isGalleryView = !isGalleryView;
        const icon = galleryToggleBtn.querySelector('i');
        if (isGalleryView) {
            icon.classList.remove('fa-th-large');
            icon.classList.add('fa-list');
            galleryToggleBtn.title = "Toggle list view";
        } else {
            icon.classList.remove('fa-list');
            icon.classList.add('fa-th-large');
            galleryToggleBtn.title = "Toggle gallery view";
        }
        renderContent(currentDirectoryContent);
    });

    mainContent.addEventListener('click', (e) => {
        const item = e.target.closest('.item');
        if (!item || item.tagName === 'A') return;
        const { path, type, mediaIndex, filename } = item.dataset;
        if (type === 'folder') {
            fetchData(path, filename);
        } else if (mediaIndex) {
            openModal(mediaIndex);
        }
    });

    mainContent.addEventListener('keydown', (e) => {
        const item = e.target.closest('.item');
        if (!item || item.tagName === 'A') return;
        if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            const { path, type, mediaIndex, filename } = item.dataset;
            if (type === 'folder') {
                fetchData(path, filename);
            } else if (mediaIndex) {
                openModal(mediaIndex);
            }
        }
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
    modalPlayBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        toggleSlideshow();
    });
    modalShuffleBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        startShuffleSlideshow();
    });

    // Upload Modal Listeners
    uploadBtn.addEventListener('click', openUploadModal);
    uploadModalCloseBtn.addEventListener('click', closeUploadModal);
    uploadForm.addEventListener('submit', handleUpload);

    // File input change handler
    fileInput.addEventListener('change', updateFileList);

    // Drag and drop handlers
    fileDropZone.addEventListener('click', () => fileInput.click());
    fileDropZone.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            fileInput.click();
        }
    });
    fileDropZone.addEventListener('dragover', (e) => {
        e.preventDefault();
        e.stopPropagation();
        fileDropZone.classList.add('drag-over');
    });
    fileDropZone.addEventListener('dragleave', (e) => {
        e.preventDefault();
        e.stopPropagation();
        fileDropZone.classList.remove('drag-over');
    });
    fileDropZone.addEventListener('drop', (e) => {
        e.preventDefault();
        e.stopPropagation();
        fileDropZone.classList.remove('drag-over');
        if (e.dataTransfer.files.length > 0) {
            fileInput.files = e.dataTransfer.files;
            updateFileList();
        }
    });

    // Path input validation
    pathInput.addEventListener('input', () => {
        const validation = validatePath(pathInput.value.trim());
        if (!validation.valid) {
            pathError.textContent = validation.error;
            pathError.classList.remove('hidden');
        } else {
            pathError.classList.add('hidden');
        }
    });

    // Notification close handler
    notificationClose.addEventListener('click', hideNotification);

    document.addEventListener('keydown', (e) => {
        if (!modal.classList.contains('hidden')) {
            if (e.key === 'Escape') {
                closeModal();
            } else if (e.key === 'ArrowLeft' && !modalPrevBtn.disabled) {
                e.preventDefault();
                stopSlideshow();
                openModal(currentModalIndex - 1);
            } else if (e.key === 'ArrowRight' && !modalNextBtn.disabled) {
                e.preventDefault();
                stopSlideshow();
                openModal(currentModalIndex + 1);
            }
        } else if (!uploadModal.classList.contains('hidden')) {
            if (e.key === 'Escape') {
                closeUploadModal();
            }
        }
    });
    logoEl.addEventListener('click', triggerSync);

    // INITIAL LOAD
    fetchData('');
});
