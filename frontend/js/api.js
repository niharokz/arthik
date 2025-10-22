// API Client - Centralized HTTP requests

const API = {
    baseURL: '/api',
    
    // Generic request handler
    async request(url, options = {}) {
        const token = getAuthToken();
        
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers
        };
        
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }
        
        try {
            const response = await fetch(url, { ...options, headers });
            
            if (response.status === 401) {
                handleAuthenticationError();
                throw new Error('Authentication failed');
            }
            
            if (response.status === 429) {
                throw new Error('Too many requests. Please try again later.');
            }
            
            if (!response.ok && response.status !== 400) {
                throw new Error(`Request failed with status ${response.status}`);
            }
            
            return response;
        } catch (error) {
            console.error('API request error:', error);
            throw error;
        }
    },
    
    // Authentication
    async login(password) {
        const response = await fetch(`${this.baseURL}/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ password })
        });
        
        if (response.status === 429) {
            throw new Error('Too many failed login attempts. Please try again later.');
        }
        
        return response.json();
    },
    
    // Accounts
    async getAccounts() {
        const response = await this.request(`${this.baseURL}/accounts`);
        return response.json();
    },
    
    async addAccount(account) {
        const response = await this.request(`${this.baseURL}/accounts`, {
            method: 'POST',
            body: JSON.stringify(account)
        });
        return response.json();
    },
    
    async deleteAccount(name) {
        const response = await this.request(`${this.baseURL}/accounts/${encodeURIComponent(name)}`, {
            method: 'DELETE'
        });
        return response.json();
    },
    
    // Transactions
    async getTransactions(month) {
        const url = month 
            ? `${this.baseURL}/transactions?month=${month}` 
            : `${this.baseURL}/transactions`;
        const response = await this.request(url);
        return response.json();
    },
    
    async saveTransaction(transaction) {
        const response = await this.request(`${this.baseURL}/transactions`, {
            method: 'POST',
            body: JSON.stringify(transaction)
        });
        return response.json();
    },
    
    async deleteTransaction(id) {
        const response = await this.request(`${this.baseURL}/transactions/${id}`, {
            method: 'DELETE'
        });
        return response.json();
    },
    
    // Dashboard
    async getDashboard() {
        const response = await this.request(`${this.baseURL}/dashboard`);
        return response.json();
    },
    
    // Settings
    async getSettings() {
        const response = await this.request(`${this.baseURL}/settings`);
        return response.json();
    },
    
    async updateSettings(settings) {
        const response = await this.request(`${this.baseURL}/settings`, {
            method: 'POST',
            body: JSON.stringify(settings)
        });
        return response.json();
    },
    
    // Recurrence
    async getRecurrences() {
        const response = await this.request(`${this.baseURL}/recurrence`);
        return response.json();
    },
    
    async saveRecurrence(recurrence) {
        const response = await this.request(`${this.baseURL}/recurrence`, {
            method: 'POST',
            body: JSON.stringify(recurrence)
        });
        return response.json();
    },
    
    async deleteRecurrence(id) {
        const response = await this.request(`${this.baseURL}/recurrence/${id}`, {
            method: 'DELETE'
        });
        return response.json();
    },
    
    async applyRecurrence(id) {
        const response = await this.request(`${this.baseURL}/recurrence/apply/${id}`, {
            method: 'POST'
        });
        return response.json();
    },
    
    // Notes
    async getNotes() {
        const response = await this.request(`${this.baseURL}/notes`);
        return response.json();
    },
    
    async saveNote(note) {
        const response = await this.request(`${this.baseURL}/notes`, {
            method: 'POST',
            body: JSON.stringify(note)
        });
        return response.json();
    },
    
    async deleteNote(id) {
        const response = await this.request(`${this.baseURL}/notes/${encodeURIComponent(id)}`, {
            method: 'DELETE'
        });
        return response.json();
    }
};

// Handle authentication errors
function handleAuthenticationError() {
    resetState();
    showLoginScreen();
    alert('Your session has expired. Please login again.');
}

function showLoginScreen() {
    document.getElementById('mainApp').classList.add('hidden');
    document.getElementById('loginScreen').classList.remove('hidden');
}