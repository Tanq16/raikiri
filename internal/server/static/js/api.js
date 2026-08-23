const API = {
    async list(path, mode, recursive = false) {
        try {
            const params = new URLSearchParams({ path, mode, recursive });
            const res = await fetch(`/api/list?${params.toString()}`);
            if (!res.ok) throw new Error('Failed to fetch');
            return await res.json();
        } catch (e) {
            console.error(e);
            return [];
        }
    },

    async upload(files, path, mode) {
        try {
            const formData = new FormData();
            for (let i = 0; i < files.length; i++) {
                formData.append('files', files[i]);
            }
            formData.append('path', path);
            formData.append('mode', mode);

            const res = await fetch('/api/upload', {
                method: 'POST',
                body: formData
            });
            return res.ok;
        } catch (e) {
            console.error('Upload failed', e);
            return false;
        }
    },

    async getHistory(mode) {
        try {
            const res = await fetch(`/api/history?mode=${encodeURIComponent(mode)}`);
            if (!res.ok) throw new Error('Failed to fetch history');
            return await res.json();
        } catch (e) {
            console.error(e);
            return [];
        }
    },

    async recordHistory(entry) {
        try {
            await fetch('/api/history', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(entry)
            });
        } catch (e) {
            console.error('Failed to record history', e);
        }
    },

    async previewDelete(mode, paths) {
        try {
            const res = await fetch('/api/delete/preview', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ mode, paths })
            });
            if (!res.ok) throw new Error('Failed to preview deletion');
            return await res.json();
        } catch (e) {
            console.error(e);
            return null;
        }
    },

    async deleteItems(mode, paths) {
        try {
            const res = await fetch('/api/delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ mode, paths })
            });
            if (!res.ok) throw new Error('Failed to delete');
            return await res.json();
        } catch (e) {
            console.error(e);
            return null;
        }
    },

    getContentUrl(path, mode) {
        const cleanPath = path.startsWith('/') ? path.substring(1) : path;
        const encoded = cleanPath.split('/').map(s => encodeURIComponent(s)).join('/');
        return `/content/${encoded}?mode=${mode}`;
    }
};

export default API;
