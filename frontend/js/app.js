// Main Application File - Initialize and coordinate everything

// Initialize app on DOM ready
document.addEventListener('DOMContentLoaded', function() {
    // Initialize state
    initializeState();
    
    // Setup all event listeners
    setupAllListeners();
    
    // Load theme
    loadTheme();
    
    // Check for stored token and auto-login
    if (isAuthenticated()) {
        autoLogin();
    }
    
    // Handle window resize for responsive charts
    setupResizeHandler();
});

// Auto-login with stored token
async function autoLogin() {
    document.getElementById('loginScreen').classList.add('hidden');
    document.getElementById('mainApp').classList.remove('hidden');
    
    try {
        await loadInitialData();
    } catch (error) {
        console.error('Auto-login failed:', error);
        handleAuthenticationError();
    }
}

// Setup all event listeners
function setupAllListeners() {
    setupTabListeners();
    setupLoginListener();
    setupTransactionListeners();
    setupAccountListeners();
    setupSettingsListeners();
    setupPlannerListeners();
}

// Tab navigation
function setupTabListeners() {
    const tabs = document.querySelectorAll('.tab');
    tabs.forEach(tab => {
        tab.addEventListener('click', function() {
            const tabName = this.getAttribute('data-tab');
            showTab(tabName);
        });
    });
}

function showTab(tabName) {
    // Update tab UI
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.content').forEach(c => c.classList.remove('active'));
    
    const activeTab = document.querySelector(`.tab[data-tab="${tabName}"]`);
    if (activeTab) {
        activeTab.classList.add('active');
    }
    
    const activeContent = document.getElementById(tabName);
    if (activeContent) {
        activeContent.classList.add('active');
    }
    
    // Update state
    setActiveTab(tabName);
    
    // Load tab-specific data
    loadTabData(tabName);
}

async function loadTabData(tabName) {
    try {
        switch (tabName) {
            case 'dashboard':
                await loadDashboard();
                break;
            case 'ledger':
                setCurrentPage(1);
                await loadTransactions();
                break;
            case 'accounts':
                await loadAccounts();
                break;
            case 'planner':
                await Promise.all([
                    loadRecurrences(),
                    loadNotes()
                ]);
                break;
        }
    } catch (error) {
        console.error(`Error loading ${tabName}:`, error);
    }
}

// Handle window resize for charts
function setupResizeHandler() {
    let resizeTimer;
    window.addEventListener('resize', function() {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(function() {
            const activeTab = AppState.activeTab;
            if (activeTab === 'dashboard' && isAuthenticated()) {
                loadDashboard();
            }
        }, 250);
    });
}

// Export global functions that are called from HTML onclick handlers
window.login = login;
window.logout = logout;
window.showTab = showTab;
window.changeTheme = changeTheme;
window.showChangePasswordModal = showChangePasswordModal;
window.closeChangePasswordModal = closeChangePasswordModal;
window.changePassword = changePassword;

// Transaction functions
window.editTransaction = editTransaction;
window.cancelEdit = cancelEdit;
window.updateTransaction = updateTransaction;
window.deleteTransaction = deleteTransaction;

// Account functions
window.editAccount = editAccount;
window.cancelAccountEdit = cancelAccountEdit;
window.saveAccountEdit = saveAccountEdit;
window.deleteAccount = deleteAccount;

// Recurrence functions
window.applyRecurrence = applyRecurrence;
window.deleteRecurrence = deleteRecurrence;

// Note functions
window.editNote = editNote;
window.deleteNote = deleteNote;
window.cancelNote = cancelNote;
window.saveNote = saveNote;