// =============================================
// BRC - Main JavaScript File
// =============================================

// Global utilities
const BRC = {
    // Show toast notification
    showToast: function(message, type = 'success') {
        const existingToast = document.getElementById('toast-notification');
        if (existingToast) {
            existingToast.remove();
        }
        
        const toast = document.createElement('div');
        toast.id = 'toast-notification';
        toast.className = `fixed bottom-6 right-6 z-50 px-6 py-4 rounded-xl shadow-2xl transform transition-all duration-300 translate-y-20 opacity-0 ${
            type === 'success' ? 'bg-card border border-accent' : 'bg-card border border-red-500'
        }`;
        
        const iconColor = type === 'success' ? 'text-accent' : 'text-red-400';
        toast.innerHTML = `
            <div class="flex items-center gap-3">
                <svg class="w-5 h-5 ${iconColor}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    ${type === 'success' 
                        ? '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>'
                        : '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>'
                    }
                </svg>
                <span>${message}</span>
            </div>
        `;
        
        document.body.appendChild(toast);
        
        // Animate in
        requestAnimationFrame(() => {
            toast.classList.remove('translate-y-20', 'opacity-0');
            toast.classList.add('translate-y-0', 'opacity-100');
        });
        
        // Remove after delay
        setTimeout(() => {
            toast.classList.remove('translate-y-0', 'opacity-100');
            toast.classList.add('translate-y-20', 'opacity-0');
            setTimeout(() => toast.remove(), 300);
        }, 3000);
    },
    
    // Copy to clipboard
    copyToClipboard: async function(text) {
        try {
            await navigator.clipboard.writeText(text);
            this.showToast('Berhasil disalin ke clipboard!');
            return true;
        } catch (err) {
            // Fallback for older browsers
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.select();
            const success = document.execCommand('copy');
            document.body.removeChild(textarea);
            
            if (success) {
                this.showToast('Berhasil disalin ke clipboard!');
            } else {
                this.showToast('Gagal menyalin', 'error');
            }
            return success;
        }
    },
    
    // Format date
    formatDate: function(dateString) {
        const date = new Date(dateString);
        const options = { 
            year: 'numeric', 
            month: 'long', 
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        };
        return date.toLocaleDateString('id-ID', options);
    },
    
    // Debounce function
    debounce: function(func, wait) {
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
    
    // Throttle function
    throttle: function(func, limit) {
        let inThrottle;
        return function(...args) {
            if (!inThrottle) {
                func.apply(this, args);
                inThrottle = true;
                setTimeout(() => inThrottle = false, limit);
            }
        };
    }
};

// =============================================
// Form Autosave Module
// =============================================
const FormAutosave = {
    saveTimeout: null,
    saveInterval: 30000, // 30 seconds
    formId: null,
    saveUrl: null,
    
    init: function(formId, saveUrl) {
        this.formId = formId;
        this.saveUrl = saveUrl;
        
        const form = document.getElementById(formId);
        if (!form) return;
        
        // Listen for input changes
        form.addEventListener('input', this.debounceSave.bind(this));
        
        // Periodic save
        setInterval(this.save.bind(this), this.saveInterval);
        
        // Save before leaving
        window.addEventListener('beforeunload', () => {
            this.save();
        });
    },
    
    debounceSave: function() {
        clearTimeout(this.saveTimeout);
        this.saveTimeout = setTimeout(this.save.bind(this), 2000);
    },
    
    save: async function() {
        const form = document.getElementById(this.formId);
        if (!form) return;
        
        const formData = new FormData(form);
        
        try {
            const response = await fetch(this.saveUrl, {
                method: 'POST',
                body: formData
            });
            
            if (response.ok) {
                this.showSaveIndicator('Tersimpan');
            }
        } catch (err) {
            console.error('Autosave failed:', err);
        }
    },
    
    showSaveIndicator: function(text) {
        const indicator = document.querySelector('.autosave-indicator');
        if (indicator) {
            indicator.textContent = text;
            setTimeout(() => {
                indicator.textContent = 'Auto-save aktif';
            }, 2000);
        }
    }
};

// =============================================
// Questionnaire Form Module
// =============================================
const QuestionnaireForm = {
    currentPage: 1,
    totalPages: 1,
    
    init: function(config) {
        this.currentPage = config.currentPage || 1;
        this.totalPages = config.totalPages || 1;
        
        this.initProgressTracking();
        this.initKeyboardNavigation();
        this.initBeforeUnloadWarning();
    },
    
    initProgressTracking: function() {
        const textareas = document.querySelectorAll('textarea[data-question-id]');
        const answeredCount = Array.from(textareas).filter(ta => ta.value.trim() !== '').length;
        const progress = Math.round((answeredCount / textareas.length) * 100);
        
        const progressBar = document.getElementById('progressBar');
        const progressText = document.getElementById('progressText');
        
        if (progressBar) progressBar.style.width = progress + '%';
        if (progressText) progressText.textContent = progress + '%';
    },
    
    initKeyboardNavigation: function() {
        document.addEventListener('keydown', (e) => {
            // Ctrl/Cmd + Enter to submit
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                const form = document.getElementById('questionnaireForm');
                if (form) form.submit();
            }
            
            // Ctrl/Cmd + S to save
            if ((e.ctrlKey || e.metaKey) && e.key === 's') {
                e.preventDefault();
                const saveBtn = document.querySelector('button[value="save_later"]');
                if (saveBtn) saveBtn.click();
            }
        });
    },
    
    initBeforeUnloadWarning: function() {
        let hasChanges = false;
        
        document.querySelectorAll('textarea').forEach(ta => {
            ta.addEventListener('input', () => {
                hasChanges = true;
            });
        });
        
        window.addEventListener('beforeunload', (e) => {
            if (hasChanges) {
                e.preventDefault();
                e.returnValue = '';
            }
        });
        
        document.getElementById('questionnaireForm')?.addEventListener('submit', () => {
            hasChanges = false;
        });
    }
};

// =============================================
// Document Viewer Module
// =============================================
const DocumentViewer = {
    init: function() {
        this.initScrollSpy();
        this.initPrintButton();
    },
    
    initScrollSpy: function() {
        const headings = document.querySelectorAll('.document-content h2, .document-content h3');
        const nav = document.querySelector('.document-nav');
        
        if (!nav || headings.length === 0) return;
        
        const observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    const id = entry.target.id;
                    nav.querySelectorAll('a').forEach(a => {
                        a.classList.toggle('active', a.getAttribute('href') === '#' + id);
                    });
                }
            });
        }, { rootMargin: '-20% 0px -80% 0px' });
        
        headings.forEach(h => {
            if (!h.id) {
                h.id = h.textContent.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '');
            }
            observer.observe(h);
        });
    },
    
    initPrintButton: function() {
        const printBtn = document.getElementById('printBtn');
        if (printBtn) {
            printBtn.addEventListener('click', () => window.print());
        }
    }
};

// =============================================
// Dashboard Module
// =============================================
const Dashboard = {
    init: function() {
        this.initAnimations();
        this.initTooltips();
    },
    
    initAnimations: function() {
        // Intersection Observer for fade-in animations
        const observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    entry.target.classList.add('fade-in-visible');
                }
            });
        }, { threshold: 0.1 });
        
        document.querySelectorAll('.animate-on-scroll').forEach(el => {
            observer.observe(el);
        });
    },
    
    initTooltips: function() {
        document.querySelectorAll('[data-tooltip]').forEach(el => {
            el.classList.add('tooltip');
        });
    }
};

// =============================================
// Initialize on DOM Ready
// =============================================
document.addEventListener('DOMContentLoaded', function() {
    // Initialize modules based on page
    const body = document.body;
    
    if (body.classList.contains('page-dashboard')) {
        Dashboard.init();
    }
    
    if (body.classList.contains('page-questionnaire-form')) {
        QuestionnaireForm.init({
            currentPage: parseInt(document.querySelector('meta[name="current-page"]')?.content || 1),
            totalPages: parseInt(document.querySelector('meta[name="total-pages"]')?.content || 1)
        });
    }
    
    if (body.classList.contains('page-document')) {
        DocumentViewer.init();
    }
    
    // Global copy link functionality
    document.querySelectorAll('[data-copy]').forEach(el => {
        el.addEventListener('click', function() {
            const text = this.getAttribute('data-copy');
            BRC.copyToClipboard(text);
        });
    });
    
    // Tab switching
    window.switchTab = function(tabName) {
        // Update tab buttons
        document.querySelectorAll('[data-tab-btn]').forEach(btn => {
            btn.classList.toggle('active', btn.getAttribute('data-tab-btn') === tabName);
        });
        
        // Update tab content
        document.querySelectorAll('[data-tab-content]').forEach(content => {
            content.classList.toggle('hidden', content.getAttribute('data-tab-content') !== tabName);
        });
        
        // Update URL without reload
        const url = new URL(window.location);
        url.searchParams.set('tab', tabName);
        window.history.pushState({}, '', url);
    };
    
    // Read initial tab from URL
    const urlParams = new URLSearchParams(window.location.search);
    const initialTab = urlParams.get('tab');
    if (initialTab) {
        switchTab(initialTab);
    }
});

// =============================================
// Export for use in inline scripts
// =============================================
window.BRC = BRC;
window.QuestionnaireForm = QuestionnaireForm;
window.DocumentViewer = DocumentViewer;