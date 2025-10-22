// Global Application State Management

const AppState = {
    // Authentication
    isAuthenticated: false,
    authToken: null,
    
    // Data
    accounts: [],
    transactions: [],
    recurrences: [],
    notes: [],
    
    // Pagination
    currentPage: 1,
    itemsPerPage: 100,
    
    // Edit states
    editingTransactionId: null,
    editingRecurrenceId: null,
    editingNoteId: null,
    editingAccountName: null,
    
    // Chart instances
    charts: {
        monthlyOverview: null,
        budget: null,
        progress: null,
        assetDistribution: null
    },
    
    // UI state
    activeTab: 'dashboard'
};

// State getters
const getState = () => ({ ...AppState });

const getAuthToken = () => AppState.authToken;

const isAuthenticated = () => AppState.isAuthenticated;

const getAccounts = () => AppState.accounts;

const getTransactions = () => AppState.transactions;

const getCurrentPage = () => AppState.currentPage;

const getEditingTransactionId = () => AppState.editingTransactionId;

// State setters
const setAuthToken = (token) => {
    AppState.authToken = token;
    AppState.isAuthenticated = !!token;
    
    if (token) {
        sessionStorage.setItem('authToken', token);
    } else {
        sessionStorage.removeItem('authToken');
    }
};

const setAccounts = (accounts) => {
    AppState.accounts = accounts;
};

const setTransactions = (transactions) => {
    AppState.transactions = transactions;
};

const setCurrentPage = (page) => {
    AppState.currentPage = page;
};

const setEditingTransactionId = (id) => {
    AppState.editingTransactionId = id;
};

const setEditingAccountName = (name) => {
    AppState.editingAccountName = name;
};

const setEditingRecurrenceId = (id) => {
    AppState.editingRecurrenceId = id;
};

const setEditingNoteId = (id) => {
    AppState.editingNoteId = id;
};

const setRecurrences = (recurrences) => {
    AppState.recurrences = recurrences;
};

const setNotes = (notes) => {
    AppState.notes = notes;
};

const setActiveTab = (tab) => {
    AppState.activeTab = tab;
};

// Chart management
const setChart = (name, chart) => {
    if (AppState.charts[name]) {
        AppState.charts[name].destroy();
    }
    AppState.charts[name] = chart;
};

const getChart = (name) => AppState.charts[name];

const destroyChart = (name) => {
    if (AppState.charts[name]) {
        AppState.charts[name].destroy();
        AppState.charts[name] = null;
    }
};

const destroyAllCharts = () => {
    Object.keys(AppState.charts).forEach(name => {
        destroyChart(name);
    });
};

// Initialize state from storage
const initializeState = () => {
    const storedToken = sessionStorage.getItem('authToken');
    if (storedToken) {
        setAuthToken(storedToken);
    }
};

// Reset state (for logout)
const resetState = () => {
    AppState.isAuthenticated = false;
    AppState.authToken = null;
    AppState.accounts = [];
    AppState.transactions = [];
    AppState.recurrences = [];
    AppState.notes = [];
    AppState.currentPage = 1;
    AppState.editingTransactionId = null;
    AppState.editingRecurrenceId = null;
    AppState.editingNoteId = null;
    AppState.editingAccountName = null;
    destroyAllCharts();
    sessionStorage.removeItem('authToken');
};

const getEditingAccountName = () => AppState.editingAccountName;

const getEditingRecurrenceId = () => AppState.editingRecurrenceId;

const getEditingNoteId = () => AppState.editingNoteId;