// Exists only to make the app installable; Chrome grants installed PWAs the
// elevated audio privileges this player needs. It caches nothing.
self.addEventListener('fetch', () => {});
