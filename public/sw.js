self.addEventListener('install', (event) => {
    console.log('Service Worker: Installed');
});
self.addEventListener('activate', (event) => {
    console.log('Service Worker: Activated');
});
self.addEventListener('fetch', (event) => {
    const url = new URL(event.request.url);

    // Let the browser handle large media files directly (enables Range / 206)
    if (url.pathname.startsWith('/content/')) {
        return;
    }

    event.respondWith(fetch(event.request));
});
