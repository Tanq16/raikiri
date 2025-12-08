const state = {
    mode: 'files', // 'files' | 'music'
    path: '/',
    view: 'grid',
    items: [],
    
    // Derived getters logic if needed, but keeping it simple prop store
    setPath(newPath) {
        this.path = newPath;
        // Update URL hash - encode each path segment to handle special characters
        const pathSegments = newPath.split('/').filter(p => p).map(p => encodeURIComponent(p));
        const encodedPath = pathSegments.length > 0 ? '/' + pathSegments.join('/') : '/';
        window.location.hash = `#/${this.mode}${encodedPath}`;
    },
    
    setMode(newMode) {
        this.mode = newMode;
        this.path = '/';
        window.location.hash = `#/${newMode}/`;
    }
};

export default state;
