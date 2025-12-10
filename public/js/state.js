const state = {
    mode: 'files', // 'files' | 'music'
    path: '/',
    view: 'grid',
    items: [],
    
    setPath(newPath) {
        this.path = newPath;
        const pathSegments = newPath.split('/').filter(p => p).map(p => encodeURIComponent(p));
        const encodedPath = pathSegments.length > 0 ? '/' + pathSegments.join('/') : '/';
        window.location.hash = `#/${this.mode}${encodedPath}`;
    },
    
    setMode(newMode) {
        this.mode = newMode;
        this.path = '/';
        window.location.hash = `#/${newMode}/`;
    },
};

export default state;
