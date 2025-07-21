// frontend/app.js
document.addEventListener('DOMContentLoaded', () => {
    // --- STATE ---
    let currentPath = '';
    let currentDirectoryContent = { images: [], videos: [] };
    let currentModalIndex = -1;
    let currentMediaList = []; // Combined list of images and videos for the modal

    // --- DOM ELEMENTS ---
    const mainContent = document.getElementById('main-content');
    const breadcrumbsEl = document.getElementById('breadcrumbs');
    const searchInput = document.getElementById('search-input');
    const modal = document.getElementById('media-modal');
    const modalContentContainer = modal.querySelector('.modal-content-container');
    const modalCloseBtn = document.getElementById('modal-close-btn');
    const modalPrevBtn = document.getElementById('modal-prev-btn');
    const modalNextBtn = document.getElementById('modal-next-btn');

    // --- Plyr Player Instance ---
    let player;

    // --- API FUNCTIONS ---
    async function fetchData(path) {
        try {
            const response = await fetch(`/api/browse/${path}`);
            if (!response.ok) throw new Error('Network response was not ok');
            const data = await response.json();
            currentPath = path;
            currentDirectoryContent = data;
            renderContent(data);
        } catch (error) {
            mainContent.innerHTML = `<p class="text-red-400">Error loading content: ${error.message}</p>`;
        }
    }

    async function searchFiles(query) {
        if (!query) {
            fetchData(currentPath); // Go back to browsing
            return;
        }
        try {
            const response = await fetch(`/api/search?q=${encodeURIComponent(query)}`);
            if (!response.ok) throw new Error('Network response was not ok');
            const results = await response.json();
            renderSearchResults(results);
        } catch (error) {
            mainContent.innerHTML = `<p class="text-red-400">Error searching: ${error.message}</p>`;
        }
    }

    // --- RENDER FUNCTIONS ---
    function renderContent(data) {
        breadcrumbsEl.innerHTML = renderBreadcrumbs(data.breadcrumbs);
        mainContent.innerHTML = `
            ${data.folders.length > 0 ? renderSection('Folders', renderFolderItems(data.folders)) : ''}
            ${data.images.length > 0 ? renderSection('Images', renderMediaItems(data.images, 'image')) : ''}
            ${data.videos.length > 0 ? renderSection('Videos', renderMediaItems(data.videos, 'video')) : ''}
            ${data.others.length > 0 ? renderSection('Other Files', renderOtherItems(data.others)) : ''}
        `;
    }
    
    function renderBreadcrumbs(breadcrumbs) {
        return breadcrumbs.map((crumb, index) => {
            const isLast = index === breadcrumbs.length - 1;
            if (isLast) {
                return `<span class="text-white font-medium">${crumb.Name}</span>`;
            }
            return `<a href="#" class="hover:text-indigo-400" data-path="${crumb.Path}">${crumb.Name}</a><span class="mx-2">/</span>`;
        }).join('');
    }

    function renderSection(title, content) {
        const id = title.toLowerCase().replace(' ', '-');
        return `
            <section id="${id}-section" class="mb-12">
                <div class="section-title mb-4">
                    <h2 class="text-xl font-semibold text-white">${title}</h2>
                    <svg class="section-arrow w-5 h-5 ml-2 text-gray-400 transform transition-transform" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
                        <path fill-rule="evenodd" d="M5.293 9.293a1 1 0 011.414 0L10 12.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd" />
                    </svg>
                </div>
                <div id="${id}-content" class="section-content grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
                    ${content}
                </div>
            </section>
        `;
    }

    function renderFolderItems(folders) {
        return folders.map(folder => `
            <a href="#" class="group flex flex-col items-center p-4 bg-gray-800 rounded-lg hover:bg-gray-700 transition-colors duration-200" data-path="${folder.Path}">
                <svg class="w-12 h-12 text-indigo-400 mb-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                </svg>
                <span class="text-sm text-center text-gray-300 group-hover:text-white truncate w-full">${folder.Name}</span>
            </a>
        `).join('');
    }
    
    function renderMediaItems(items, type) {
        const gridClass = type === 'image' 
            ? 'grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4' 
            : 'grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6';

        const content = items.map((item, index) => {
            const mediaIndex = type === 'image' ? index : currentDirectoryContent.images.length + index;
            const aspectClass = type === 'image' ? 'aspect-w-1 aspect-h-1' : 'aspect-w-16 aspect-h-9';
            const placeholder = type === 'image' 
                ? `https://placehold.co/400x400/1f2937/4b5563?text=${encodeURIComponent(item.Name)}`
                : `https://placehold.co/640x360/1f2937/4b5563?text=${encodeURIComponent(item.Name)}`;
            
            return `
                <div class="group cursor-pointer" data-media-index="${mediaIndex}" data-media-type="${type}">
                    <div class="relative overflow-hidden rounded-lg ${aspectClass} bg-gray-800">
                        <img src="${type === 'image' ? `/media/${item.Path}` : placeholder}" alt="${item.Name}" class="w-full h-full object-cover transform group-hover:scale-110 transition-transform duration-300" loading="lazy" onerror="this.src='${placeholder}'">
                        <div class="absolute inset-0 bg-black bg-opacity-0 group-hover:bg-opacity-20 transition-all duration-300 flex items-center justify-center">
                            ${type === 'video' ? `
                                <svg class="w-12 h-12 text-white opacity-0 group-hover:opacity-80 transform group-hover:scale-110 transition-all" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd" />
                                </svg>` : ''}
                        </div>
                    </div>
                    <p class="text-xs text-gray-400 mt-2 truncate">${item.Name}</p>
                </div>
            `;
        }).join('');
        
        // This is a bit of a hack to replace the grid class in the section
        // A better long-term solution would be a more robust templating function
        setTimeout(() => {
            const sectionContent = document.getElementById(`${type}s-content`);
            if(sectionContent) {
                sectionContent.className = `section-content grid ${gridClass}`;
            }
        }, 0);

        return content;
    }

    function renderOtherItems(items) {
         return items.map(item => `
            <a href="/media/${item.Path}" target="_blank" class="group flex flex-col items-center p-4 bg-gray-800 rounded-lg hover:bg-gray-700 transition-colors duration-200">
                <svg class="w-12 h-12 text-gray-400 mb-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
                </svg>
                <span class="text-sm text-center text-gray-300 group-hover:text-white truncate w-full">${item.Name}</span>
            </a>
        `).join('');
    }

    function renderSearchResults(results) {
        breadcrumbsEl.innerHTML = `<span class="text-white font-medium">Search Results</span>`;
        if (results.length === 0) {
            mainContent.innerHTML = `<p>No results found.</p>`;
            return;
        }
        const resultItems = results.map(item => `
             <a href="#" class="p-3 flex items-center bg-gray-800 rounded-lg hover:bg-gray-700 transition-colors duration-200" data-path="${item.Path.substring(0, item.Path.lastIndexOf('/'))}">
                <svg class="w-5 h-5 mr-3 text-gray-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                <span class="text-gray-300 group-hover:text-white">${item.Path}</span>
            </a>
        `).join('');
        mainContent.innerHTML = `
            <section class="mb-12">
                <div class="grid grid-cols-1 gap-2">
                    ${resultItems}
                </div>
            </section>
        `;
    }

    // --- MODAL LOGIC ---
    function openModal(mediaIndex, mediaType) {
        currentMediaList = [...currentDirectoryContent.images, ...currentDirectoryContent.videos];
        currentModalIndex = parseInt(mediaIndex, 10);
        
        if (currentModalIndex < 0 || currentModalIndex >= currentMediaList.length) return;

        const item = currentMediaList[currentModalIndex];
        modalContentContainer.innerHTML = ''; // Clear previous content

        if (mediaType === 'image') {
            modalContentContainer.innerHTML = `<img id="modal-image" src="/media/${item.Path}" alt="${item.Name}">`;
        } else if (mediaType === 'video') {
            modalContentContainer.innerHTML = `<video id="modal-video-player" playsinline controls><source src="/media/${item.Path}" type="video/mp4" /></video>`;
            player = new Plyr('#modal-video-player');
        }

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
        if (currentModalIndex < currentMediaList.length - 1) {
            const nextIndex = currentModalIndex + 1;
            const nextItem = currentMediaList[nextIndex];
            openModal(nextIndex, nextItem.Type);
        }
    }

    function showPrevMedia() {
        if (currentModalIndex > 0) {
            const prevIndex = currentModalIndex - 1;
            const prevItem = currentMediaList[prevIndex];
            openModal(prevIndex, prevItem.Type);
        }
    }

    // --- EVENT LISTENERS ---
    mainContent.addEventListener('click', (e) => {
        const anchor = e.target.closest('a[data-path]');
        if (anchor) {
            e.preventDefault();
            fetchData(anchor.dataset.path);
            return;
        }
        
        const sectionTitle = e.target.closest('.section-title');
        if(sectionTitle) {
            sectionTitle.classList.toggle('collapsed');
            sectionTitle.nextElementSibling.classList.toggle('collapsed');
            return;
        }

        const mediaItem = e.target.closest('div[data-media-index]');
        if (mediaItem) {
            openModal(mediaItem.dataset.mediaIndex, mediaItem.dataset.mediaType);
        }
    });

    breadcrumbsEl.addEventListener('click', (e) => {
        const anchor = e.target.closest('a[data-path]');
        if (anchor) {
            e.preventDefault();
            fetchData(anchor.dataset.path);
        }
    });

    let searchTimeout;
    searchInput.addEventListener('keyup', () => {
        clearTimeout(searchTimeout);
        searchTimeout = setTimeout(() => {
            searchFiles(searchInput.value.trim());
        }, 300);
    });

    modalCloseBtn.addEventListener('click', closeModal);
    modal.addEventListener('click', (e) => {
        if (e.target === modal) closeModal(); // Close only if clicking the background
    });
    modalPrevBtn.addEventListener('click', showPrevMedia);
    modalNextBtn.addEventListener('click', showNextMedia);

    document.addEventListener('keydown', (e) => {
        if (modal.style.display !== 'flex') return;
        if (e.key === 'Escape') closeModal();
        if (e.key === 'ArrowLeft') showPrevMedia();
        if (e.key === 'ArrowRight') showNextMedia();
    });

    // --- INITIAL LOAD ---
    fetchData('');
});
