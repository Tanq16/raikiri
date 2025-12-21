// HTML and JavaScript escaping utilities to prevent XSS attacks

const Escape = {
    // Escape HTML entities for use in HTML text content
    html(text) {
        if (text == null) return '';
        const div = document.createElement('div');
        div.textContent = String(text);
        return div.innerHTML;
    },
    
    // Escape for use in HTML attributes (handles quotes)
    attr(value) {
        if (value == null) return '';
        return String(value)
            .replace(/&/g, '&amp;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }
};

export default Escape;

