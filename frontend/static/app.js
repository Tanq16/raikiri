document.addEventListener('DOMContentLoaded', () => {
    const imageModal = document.getElementById('image-modal');

    // Function to close the modal
    window.closeModal = function() {
        imageModal.classList.remove('is-active');
        // Clear the content to prevent old images from flashing
        const container = document.getElementById('modal-image-container');
        if (container) {
            container.innerHTML = '';
        }
    }

    // Event listener for keyboard controls
    document.addEventListener('keydown', (e) => {
        if (!imageModal.classList.contains('is-active')) return;

        if (e.key === 'Escape') {
            closeModal();
        } else if (e.key === 'ArrowLeft') {
            // Use htmx.trigger to simulate a click on the prev button
            const prevButton = document.getElementById('prev-image-btn');
            if (prevButton) {
                htmx.trigger(prevButton, 'click');
            }
        } else if (e.key === 'ArrowRight') {
            // Use htmx.trigger to simulate a click on the next button
            const nextButton = document.getElementById('next-image-btn');
            if (nextButton) {
                htmx.trigger(nextButton, 'click');
            }
        }
    });

    // --- Swipe navigation for touch devices ---
    let touchStartX = 0;
    let touchEndX = 0;

    // We add listeners to the modal itself, not the image, to catch the gesture
    // even if the user's finger moves off the image.
    imageModal.addEventListener('touchstart', (e) => {
        if (!imageModal.classList.contains('is-active')) return;
        // Only track single-finger touches
        if (e.touches.length === 1) {
            touchStartX = e.changedTouches[0].screenX;
        }
    }, { passive: true });

    imageModal.addEventListener('touchend', (e) => {
        if (!imageModal.classList.contains('is-active') || touchStartX === 0) return;
        
        if (e.changedTouches.length === 1) {
            touchEndX = e.changedTouches[0].screenX;
            handleGesture();
        }
        // Reset start position
        touchStartX = 0;
    });

    function handleGesture() {
        // A swipe is a significant horizontal movement
        const swipeThreshold = 50; 
        if (touchEndX < touchStartX - swipeThreshold) { // Swiped left
            const nextButton = document.getElementById('next-image-btn');
            if (nextButton) htmx.trigger(nextButton, 'click');
        } else if (touchEndX > touchStartX + swipeThreshold) { // Swiped right
            const prevButton = document.getElementById('prev-image-btn');
            if (prevButton) htmx.trigger(prevButton, 'click');
        }
    }
});
