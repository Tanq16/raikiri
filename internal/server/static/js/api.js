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
    },

    getContentUrl(path, mode) {
        // Double slash prevention
        const cleanPath = path.startsWith('/') ? path.substring(1) : path;
        return `/content/${cleanPath}?mode=${mode}`;
    }
};

export default API;
