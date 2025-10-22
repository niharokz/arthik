// Utility Helper Functions

const Helpers = {
    // Format currency
    formatCurrency(amount) {
        return 'Rs ' + parseFloat(amount).toFixed(2);
    },
    
    // Format date
    formatDate(dateStr) {
        const date = new Date(dateStr);
        return date.toLocaleDateString('en-IN', { 
            day: '2-digit', 
            month: 'short', 
            year: 'numeric' 
        });
    },
    
    // Format time
    formatTime(dateStr) {
        const date = new Date(dateStr);
        return date.toLocaleTimeString('en-IN', { 
            hour: '2-digit', 
            minute: '2-digit' 
        });
    },
    
    // Format datetime
    formatDateTime(dateStr) {
        const date = new Date(dateStr);
        return date.toLocaleDateString('en-IN', { 
            day: '2-digit', 
            month: 'short', 
            year: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
    },
    
    // Get current date in YYYY-MM-DD format
    getCurrentDate() {
        const now = new Date();
        return now.toISOString().split('T')[0];
    },
    
    // Get current time in HH:MM format
    getCurrentTime() {
        const now = new Date();
        return now.toTimeString().slice(0, 5);
    },
    
    // Sanitize HTML
    sanitizeHTML(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    },
    
    // Debounce function
    debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    },
    
    // Show/hide element
    show(elementId) {
        const element = document.getElementById(elementId);
        if (element) {
            element.classList.remove('hidden');
        }
    },
    
    hide(elementId) {
        const element = document.getElementById(elementId);
        if (element) {
            element.classList.add('hidden');
        }
    },
    
    toggle(elementId) {
        const element = document.getElementById(elementId);
        if (element) {
            element.classList.toggle('hidden');
        }
    },
    
    // Scroll to element
    scrollTo(elementId, behavior = 'smooth') {
        const element = document.getElementById(elementId);
        if (element) {
            element.scrollIntoView({ behavior, block: 'start' });
        }
    },
    
    // Check if mobile
    isMobile() {
        return window.innerWidth < 640;
    },
    
    // Arrow character for transactions
    getArrow() {
        return String.fromCharCode(8594); // →
    },
    
    // Calculate percentage
    calculatePercentage(value, total) {
        if (total === 0) return 0;
        return ((value / total) * 100).toFixed(1);
    }
};