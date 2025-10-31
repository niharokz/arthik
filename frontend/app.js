// Global state
let currentPage = 1;
let totalTransactions = 0;
let accounts = [];
let dashboardData = null;
let netWorthChart = null;
let budgetChart = null;
let portfolioChart = null;
let editingTransaction = null;
let editingAccount = null;
let csrfToken = null;

// Security: Sanitize output to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
    checkAuthentication();
    fetchReadonlyInfo();

    document.getElementById('loginForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        await handleLogin();
    });

    // Event delegation for dynamic buttons
    document.body.addEventListener('click', (e) => {
        const target = e.target.closest('[data-action]');
        if (!target) return;

        const action = target.getAttribute('data-action');
        
        switch(action) {
            case 'edit-transaction':
                editTransaction(target.getAttribute('data-date'), target.getAttribute('data-time'));
                break;
            case 'save-edit-transaction':
                saveEditTransaction();
                break;
            case 'cancel-edit-transaction':
                loadTransactions(currentPage);
                break;
            case 'delete-transaction':
                deleteTransaction(target.getAttribute('data-date'), target.getAttribute('data-time'));
                break;
            case 'save-transaction':
                saveTransaction();
                break;
            case 'cancel-transaction':
                cancelTransaction();
                break;
            case 'show-add-transaction':
                showAddTransactionForm();
                break;
            case 'prev-page':
                changePage(-1);
                break;
            case 'next-page':
                changePage(1);
                break;
            case 'edit-account':
                editAccount(target.getAttribute('data-account'));
                break;
            case 'save-account':
                saveAccount();
                break;
            case 'save-edit-account':
                saveEditAccount();
                break;
            case 'cancel-account':
                cancelAccount();
                break;
            case 'cancel-edit-account':
                loadAccounts();
                break;
            case 'show-add-account':
                showAddAccountForm();
                break;
            case 'change-password':
                changePassword();
                break;
            case 'logout':
                logout();
                break;
        }
    });

    // Event delegation for change events
    document.body.addEventListener('change', (e) => {
        const target = e.target.closest('[data-change]');
        if (!target) return;

        const action = target.getAttribute('data-change');
        
        switch(action) {
            case 'toggle-account-fields':
                toggleAccountFields();
                break;
            case 'toggle-dark-mode':
                toggleDarkMode();
                break;
            case 'toggle-hide-amount':
                toggleHideAmount();
                break;
        }
    });

    loadSettings();
});

// Fetch readonly mode info
async function fetchReadonlyInfo() {
    try {
        const response = await fetch('/api/readonly-info', {
            method: 'GET',
            credentials: 'include'
        });

        if (response.ok) {
            const data = await response.json();
            if (data.readOnlyMode && data.password) {
                document.getElementById('readonlyPassword').style.display = 'block';
                document.getElementById('passwordDisplay').textContent = data.password;
            }
        }
    } catch (error) {
        console.error('Failed to fetch readonly info:', error);
    }
}

// Check authentication
async function checkAuthentication() {
    try {
        const response = await fetch('/api/dashboard', {
            method: 'GET',
            credentials: 'include'
        });

        if (response.ok) {
            const data = await response.json();
            csrfToken = data.csrfToken;
            console.log('CSRF Token retrieved:', csrfToken ? 'Yes' : 'No');
            showMainApp();
        } else {
            document.getElementById('loginPage').style.display = 'flex';
            document.getElementById('mainApp').style.display = 'none';
        }
    } catch (error) {
        console.error('Auth check failed:', error);
        document.getElementById('loginPage').style.display = 'flex';
        document.getElementById('mainApp').style.display = 'none';
    }
}

// Handle login
async function handleLogin() {
    const password = document.getElementById('password').value;
    const errorDiv = document.getElementById('loginError');
    
    if (!password) {
        errorDiv.textContent = 'Please enter password';
        return;
    }

    try {
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ password })
        });

        const data = await response.json();
        
        if (response.ok && data.success) {
            csrfToken = data.csrfToken;
            errorDiv.textContent = '';
            showMainApp();
        } else {
            errorDiv.textContent = data.error || 'Invalid password';
            document.getElementById('password').value = '';
        }
    } catch (error) {
        errorDiv.textContent = 'Connection error. Please try again.';
        console.error('Login error:', error);
    }
}

// Show main app
function showMainApp() {
    document.getElementById('loginPage').style.display = 'none';
    document.getElementById('mainApp').style.display = 'block';
    
    setupTabNavigation();
    loadDashboard();
}

// Setup tab navigation
function setupTabNavigation() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const tab = btn.dataset.tab;
            switchTab(tab);
        });
    });
}

// Switch tabs
function switchTab(tab) {
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));
    
    document.querySelector(`[data-tab="${tab}"]`).classList.add('active');
    document.getElementById(tab).classList.add('active');

    if (tab === 'dashboard') {
        loadDashboard();
    } else if (tab === 'ledger') {
        loadTransactions();
    } else if (tab === 'account') {
        loadAccounts();
    }
}

// API call wrapper with error handling
async function apiCall(url, options = {}) {
    try {
        options.credentials = 'include';
        
        if (options.method && options.method !== 'GET') {
            if (!options.headers) {
                options.headers = {};
            }
            if (!csrfToken) {
                console.error('CSRF token is missing!');
                alert('Session error. Please refresh the page and try again.');
                return null;
            }
            options.headers['X-CSRF-Token'] = csrfToken;
            console.log('Sending CSRF token:', csrfToken.substring(0, 10) + '...');
        }

        const response = await fetch(url, options);
        
        if (response.status === 401) {
            document.getElementById('mainApp').style.display = 'none';
            document.getElementById('loginPage').style.display = 'flex';
            document.getElementById('loginError').textContent = 'Session expired. Please login again.';
            return null;
        }

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Request failed');
        }

        return await response.json();
    } catch (error) {
        console.error('API Error:', error);
        alert(error.message || 'An error occurred. Please try again.');
        return null;
    }
}

// Dashboard functions
async function loadDashboard() {
    const data = await apiCall('/api/dashboard');
    if (!data) return;

    dashboardData = data;
    if (data.csrfToken) {
        csrfToken = data.csrfToken;
        console.log('CSRF Token updated:', csrfToken ? 'Yes' : 'No');
    }
    
    document.getElementById('netWorthValue').textContent = formatAmount(data.netWorth);
    document.getElementById('totalAssets').textContent = formatAmount(data.assets);
    document.getElementById('totalLiabilities').textContent = formatAmount(data.liabilities);
    
    renderNetWorthChart(data.records);
    renderBudgetChart(data.budget);
    renderAccountPills(data.accounts);
    renderPortfolioChart(data.accounts);
    renderUpcomingBills(data.upcomingBills);
}

function renderNetWorthChart(records) {
    const ctx = document.getElementById('netWorthChart');
    
    if (netWorthChart) {
        netWorthChart.destroy();
    }

    const theme = document.body.getAttribute('data-theme') || 'purple';
    const colorMap = {
        purple: { primary: '#9333ea', secondary: '#c084fc' },
        blue: { primary: '#2563eb', secondary: '#60a5fa' },
        green: { primary: '#16a34a', secondary: '#4ade80' },
        orange: { primary: '#ea580c', secondary: '#fb923c' },
        pink: { primary: '#db2777', secondary: '#f472b6' },
        teal: { primary: '#0d9488', secondary: '#2dd4bf' }
    };
    const colors = colorMap[theme];

    // Reverse the records to show oldest to newest
    const reversedRecords = [...records].reverse();

    netWorthChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: reversedRecords.map(r => r.date),
            datasets: [{
                label: 'Net Worth',
                data: reversedRecords.map(r => r.netWorth),
                borderColor: colors.primary,
                backgroundColor: colors.secondary + '20',
                fill: true,
                tension: 0.4
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: { display: false }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        callback: function(value) {
                            return '₹' + value.toLocaleString();
                        }
                    }
                }
            }
        }
    });
}

function renderBudgetChart(budget) {
    const ctx = document.getElementById('budgetChart');
    
    if (budgetChart) {
        budgetChart.destroy();
    }

    document.getElementById('totalSpent').textContent = formatAmount(budget.totalSpent);
    document.getElementById('totalBudget').textContent = formatAmount(budget.totalBudget);
    document.getElementById('budgetPercentage').textContent = budget.percentage.toFixed(1);

    const categories = Object.keys(budget.breakdown);
    const spent = categories.map(c => budget.breakdown[c].spent);
    const budgets = categories.map(c => budget.breakdown[c].budget);

    const theme = document.body.getAttribute('data-theme') || 'purple';
    const colorMap = {
        purple: { primary: '#9333ea', secondary: '#c084fc' },
        blue: { primary: '#2563eb', secondary: '#60a5fa' },
        green: { primary: '#16a34a', secondary: '#4ade80' },
        orange: { primary: '#ea580c', secondary: '#fb923c' },
        pink: { primary: '#db2777', secondary: '#f472b6' },
        teal: { primary: '#0d9488', secondary: '#2dd4bf' }
    };
    const colors = colorMap[theme];

    budgetChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: categories,
            datasets: [
                {
                    label: 'Spent',
                    data: spent,
                    backgroundColor: colors.primary
                },
                {
                    label: 'Budget',
                    data: budgets,
                    backgroundColor: colors.secondary + '40'
                }
            ]
        },
        options: {
            indexAxis: 'y',
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                x: {
                    beginAtZero: true,
                    ticks: {
                        callback: function(value) {
                            return '₹' + value.toLocaleString();
                        }
                    }
                }
            }
        }
    });
}

function renderAccountPills(accounts) {
    const container = document.getElementById('accountPills');
    container.innerHTML = '';

    // Only show ASSET, LIABILITIES, and INCOME (no EXPENSE)
    const types = [
        { key: 'ASSET', label: 'Assets', icon: '💰' },
        { key: 'LIABILITIES', label: 'Liabilities', icon: '💳' },
        { key: 'INCOME', label: 'Income', icon: '💵' }
    ];
    
    types.forEach(typeInfo => {
        const typeAccounts = accounts.filter(a => a.type === typeInfo.key);
        if (typeAccounts.length === 0) return;

        const section = document.createElement('div');
        section.className = 'pill-section';
        
        const header = document.createElement('div');
        header.className = 'pill-section-header';
        header.innerHTML = `
            <span class="pill-section-icon">${typeInfo.icon}</span>
            <span class="pill-section-title">${typeInfo.label}</span>
            <span class="pill-section-count">${typeAccounts.length}</span>
        `;
        section.appendChild(header);

        const pillsContainer = document.createElement('div');
        pillsContainer.className = 'pills-grid';

        typeAccounts.forEach(acc => {
            const pill = document.createElement('div');
            pill.className = `account-pill ${acc.type}`;
            pill.innerHTML = `
                <span class="account-name">${escapeHtml(acc.account)}</span>
                <span class="amount">₹${formatAmount(acc.amount)}</span>
            `;
            pillsContainer.appendChild(pill);
        });

        section.appendChild(pillsContainer);
        container.appendChild(section);
    });
}

function renderPortfolioChart(accounts) {
    const ctx = document.getElementById('portfolioChart');
    const legendContainer = document.getElementById('portfolioLegend');
    
    if (portfolioChart) {
        portfolioChart.destroy();
    }

    const investments = accounts.filter(a => a.type === 'ASSET' && a.iinw === 'Yes');
    
    if (investments.length === 0) {
        ctx.style.display = 'none';
        legendContainer.innerHTML = '<p style="text-align: center; color: #666;">No investments to display</p>';
        return;
    }

    ctx.style.display = 'block';
    
    const theme = document.body.getAttribute('data-theme') || 'purple';
    const colorMap = {
        purple: ['#9333ea', '#c084fc', '#a855f7', '#d8b4fe', '#e9d5ff'],
        blue: ['#2563eb', '#60a5fa', '#3b82f6', '#93c5fd', '#dbeafe'],
        green: ['#16a34a', '#4ade80', '#22c55e', '#86efac', '#dcfce7'],
        orange: ['#ea580c', '#fb923c', '#f97316', '#fdba74', '#fed7aa'],
        pink: ['#db2777', '#f472b6', '#ec4899', '#f9a8d4', '#fce7f3'],
        teal: ['#0d9488', '#2dd4bf', '#14b8a6', '#5eead4', '#ccfbf1']
    };
    const colors = colorMap[theme];

    portfolioChart = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: investments.map(i => i.account),
            datasets: [{
                data: investments.map(i => i.amount),
                backgroundColor: colors
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: { display: false }
            }
        }
    });

    legendContainer.innerHTML = investments.map((inv, idx) => `
        <div class="legend-item">
            <span class="legend-color" style="background-color: ${colors[idx % colors.length]}"></span>
            <span class="legend-label">${escapeHtml(inv.account)}</span>
            <span class="legend-value">₹${formatAmount(inv.amount)}</span>
        </div>
    `).join('');
}

function renderUpcomingBills(bills) {
    const tbody = document.querySelector('#billsTable tbody');
    tbody.innerHTML = '';

    if (bills.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="text-align: center; color: #666;">No upcoming bills</td></tr>';
        return;
    }

    bills.forEach(bill => {
        const row = document.createElement('tr');
        row.className = `urgency-${bill.urgency}`;
        row.innerHTML = `
            <td>${escapeHtml(bill.name)}</td>
            <td>${escapeHtml(bill.dueDate)}</td>
            <td>₹${formatAmount(bill.amount)}</td>
            <td>${bill.daysLeft} days</td>
        `;
        tbody.appendChild(row);
    });
}

// Transaction functions
async function loadTransactions(page = 1) {
    currentPage = page;
    const data = await apiCall(`/api/transactions?page=${page}`);
    if (!data) return;

    totalTransactions = data.total;
    const transactions = data.transactions;

    const listContainer = document.getElementById('transactionList');
    listContainer.innerHTML = '';

    if (transactions.length === 0) {
        listContainer.innerHTML = '<p style="text-align: center; padding: 2rem; color: #666;">No transactions found</p>';
    } else {
        transactions.forEach(tran => {
            const card = document.createElement('div');
            card.className = 'transaction-card';
            card.setAttribute('data-date', tran.tranDate);
            card.setAttribute('data-time', tran.tranTime);
            card.innerHTML = `
                <div class="transaction-grid">
                    <div><strong>Date:</strong> ${escapeHtml(tran.tranDate)}</div>
                    <div><strong>Time:</strong> ${escapeHtml(tran.tranTime)}</div>
                    <div><strong>From:</strong> ${escapeHtml(tran.from)}</div>
                    <div><strong>To:</strong> ${escapeHtml(tran.to)}</div>
                    <div><strong>Description:</strong> ${escapeHtml(tran.description)}</div>
                    <div><strong>Amount:</strong> ₹${formatAmount(tran.amount)}</div>
                    <div class="action-buttons">
                        <button class="btn-icon btn-edit" data-action="edit-transaction" data-date="${escapeHtml(tran.tranDate)}" data-time="${escapeHtml(tran.tranTime)}" title="Edit">
                            <span class="material-icons">edit</span>
                        </button>
                        <button class="btn-icon btn-delete" data-action="delete-transaction" data-date="${escapeHtml(tran.tranDate)}" data-time="${escapeHtml(tran.tranTime)}" title="Delete">
                            <span class="material-icons">delete</span>
                        </button>
                    </div>
                </div>
            `;
            listContainer.appendChild(card);
        });
    }

    updatePagination();
}

function updatePagination() {
    const totalPages = Math.ceil(totalTransactions / 10);
    document.getElementById('pageInfo').textContent = `Page ${currentPage} of ${totalPages}`;
    document.getElementById('prevPage').disabled = currentPage === 1;
    document.getElementById('nextPage').disabled = currentPage === totalPages || totalPages === 0;
}

function changePage(delta) {
    const newPage = currentPage + delta;
    const totalPages = Math.ceil(totalTransactions / 10);
    
    if (newPage >= 1 && newPage <= totalPages) {
        loadTransactions(newPage);
    }
}

async function populateAccountDropdowns() {
    const data = await apiCall('/api/accounts');
    if (!data) return;
    
    accounts = data;
    
    const fromSelect = document.getElementById('fromAccount');
    const toSelect = document.getElementById('toAccount');
    
    fromSelect.innerHTML = '<option value="">From</option>';
    toSelect.innerHTML = '<option value="">To</option>';
    
    accounts.forEach(acc => {
        fromSelect.innerHTML += `<option value="${escapeHtml(acc.account)}">${escapeHtml(acc.account)}</option>`;
        toSelect.innerHTML += `<option value="${escapeHtml(acc.account)}">${escapeHtml(acc.account)}</option>`;
    });
}

async function showAddTransactionForm() {
    await populateAccountDropdowns();
    
    const form = document.getElementById('addTransactionForm');
    const now = new Date();
    
    document.getElementById('tranDate').value = formatDateForInput(now);
    document.getElementById('tranTime').value = now.toTimeString().slice(0, 5);
    
    form.style.display = 'block';
    form.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

async function saveTransaction() {
    const dateInput = document.getElementById('tranDate').value;
    const timeInput = document.getElementById('tranTime').value;
    const from = document.getElementById('fromAccount').value;
    const to = document.getElementById('toAccount').value;
    const description = document.getElementById('description').value.trim();
    const amount = parseFloat(document.getElementById('amount').value);

    if (!dateInput || !timeInput || !from || !to || !description || !amount) {
        alert('Please fill all required fields');
        return;
    }

    if (amount <= 0) {
        alert('Amount must be positive');
        return;
    }

    if (description.length > 100) {
        alert('Description too long (max 100 characters)');
        return;
    }

    const [year, month, day] = dateInput.split('-');
    const formattedDate = `${day}-${month}-${year}`;

    const transaction = {
        tranDate: formattedDate,
        tranTime: timeInput,
        from: from,
        to: to,
        description: description,
        amount: amount
    };

    const result = await apiCall('/api/transactions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(transaction)
    });

    if (result && result.success) {
        cancelTransaction();
        loadTransactions(currentPage);
        loadDashboard();
    }
}

function cancelTransaction() {
    document.getElementById('addTransactionForm').style.display = 'none';
    document.getElementById('tranDate').value = '';
    document.getElementById('tranTime').value = '';
    document.getElementById('fromAccount').value = '';
    document.getElementById('toAccount').value = '';
    document.getElementById('description').value = '';
    document.getElementById('amount').value = '';
    editingTransaction = null;
}

async function editTransaction(date, time) {
    editingTransaction = { oldTranDate: date, oldTranTime: time };
    
    const data = await apiCall(`/api/transactions?page=${currentPage}`);
    if (!data) return;
    
    const transaction = data.transactions.find(t => t.tranDate === date && t.tranTime === time);
    if (!transaction) return;

    const card = document.querySelector(`.transaction-card[data-date="${date}"][data-time="${time}"]`);
    if (!card) return;

    const [day, month, year] = transaction.tranDate.split('-');
    const dateValue = `${year}-${month}-${day}`;

    await populateAccountDropdowns();

    card.innerHTML = `
        <div class="transaction-grid edit-mode">
            <input type="date" id="editTranDate" value="${dateValue}" required>
            <input type="time" id="editTranTime" value="${escapeHtml(transaction.tranTime)}" required>
            <select id="editFromAccount" required>
                ${accounts.map(acc => `<option value="${escapeHtml(acc.account)}" ${acc.account === transaction.from ? 'selected' : ''}>${escapeHtml(acc.account)}</option>`).join('')}
            </select>
            <select id="editToAccount" required>
                ${accounts.map(acc => `<option value="${escapeHtml(acc.account)}" ${acc.account === transaction.to ? 'selected' : ''}>${escapeHtml(acc.account)}</option>`).join('')}
            </select>
            <input type="text" id="editDescription" value="${escapeHtml(transaction.description)}" maxlength="100" required>
            <input type="number" id="editAmount" value="${transaction.amount}" step="0.01" required>
            <div class="action-buttons">
                <button class="btn-icon btn-save" data-action="save-edit-transaction" title="Save">
                    <span class="material-icons">check</span>
                </button>
                <button class="btn-icon btn-cancel" data-action="cancel-edit-transaction" title="Cancel">
                    <span class="material-icons">close</span>
                </button>
            </div>
        </div>
    `;
}

async function saveEditTransaction() {
    const dateInput = document.getElementById('editTranDate').value;
    const timeInput = document.getElementById('editTranTime').value;
    const from = document.getElementById('editFromAccount').value;
    const to = document.getElementById('editToAccount').value;
    const description = document.getElementById('editDescription').value.trim();
    const amount = parseFloat(document.getElementById('editAmount').value);

    if (!dateInput || !timeInput || !from || !to || !description || !amount) {
        alert('Please fill all required fields');
        return;
    }

    if (amount <= 0) {
        alert('Amount must be positive');
        return;
    }

    const [year, month, day] = dateInput.split('-');
    const formattedDate = `${day}-${month}-${year}`;

    const updateData = {
        oldTranDate: editingTransaction.oldTranDate,
        oldTranTime: editingTransaction.oldTranTime,
        tranDate: formattedDate,
        tranTime: timeInput,
        from: from,
        to: to,
        description: description,
        amount: amount
    };

    const result = await apiCall('/api/transactions', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updateData)
    });

    if (result && result.success) {
        editingTransaction = null;
        loadTransactions(currentPage);
        loadDashboard();
    }
}

async function deleteTransaction(date, time) {
    if (!confirm('Are you sure you want to delete this transaction?')) return;

    const result = await apiCall('/api/transactions', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tranDate: date, tranTime: time })
    });

    if (result && result.success) {
        loadTransactions(currentPage);
        loadDashboard();
    }
}

// Account functions
async function loadAccounts() {
    const data = await apiCall('/api/accounts');
    if (!data) return;

    accounts = data;
    const listContainer = document.getElementById('accountList');
    listContainer.innerHTML = '';

    const types = ['ASSET', 'LIABILITIES', 'INCOME', 'EXPENSE'];
    
    types.forEach(type => {
        const typeAccounts = accounts.filter(a => a.type === type);
        if (typeAccounts.length === 0) return;

        const section = document.createElement('div');
        section.className = 'account-section';
        
        const title = document.createElement('h3');
        title.textContent = type.charAt(0) + type.slice(1).toLowerCase();
        section.appendChild(title);

        typeAccounts.forEach(acc => {
            const card = document.createElement('div');
            card.className = 'account-card';
            card.innerHTML = `
                <div class="account-grid">
                    <div><strong>Name:</strong> ${escapeHtml(acc.account)}</div>
                    <div><strong>Amount:</strong> ₹${formatAmount(acc.amount)}</div>
                    <div><strong>In Net Worth:</strong> ${escapeHtml(acc.iinw)}</div>
                    ${acc.budget > 0 ? `<div><strong>Budget:</strong> ₹${acc.budget.toFixed(2)}</div>` : ''}
                    ${acc.dueDate ? `<div><strong>Due Date:</strong> ${escapeHtml(acc.dueDate)}</div>` : ''}
                    <div class="action-buttons">
                        <button class="btn-icon btn-edit" data-action="edit-account" data-account="${escapeHtml(acc.account)}" title="Edit">
                            <span class="material-icons">edit</span>
                        </button>
                        <button class="btn-icon btn-delete" onclick="deleteAccount('${escapeHtml(acc.account).replace(/'/g, "\\'")}')">
                            <span class="material-icons">delete</span>
                        </button>
                    </div>
                </div>
            `;
            section.appendChild(card);
        });
        
        listContainer.appendChild(section);
    });
}

function showAddAccountForm() {
    editingAccount = null;
    const form = document.getElementById('addAccountForm');
    document.getElementById('accountName').disabled = false;
    form.style.display = 'block';
    form.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function toggleAccountFields() {
    const type = document.getElementById('accountType').value;
    const budgetField = document.getElementById('accountBudget');
    const dueDateField = document.getElementById('accountDueDate');

    budgetField.style.display = type === 'EXPENSE' ? 'block' : 'none';
    dueDateField.style.display = type === 'LIABILITIES' ? 'block' : 'none';
}

async function saveAccount() {
    const name = document.getElementById('accountName').value.trim();
    const type = document.getElementById('accountType').value;
    const amount = parseFloat(document.getElementById('accountAmount').value) || 0;
    const iinw = document.querySelector('input[name="iinw"]:checked')?.value || 'No';
    const budget = parseFloat(document.getElementById('accountBudget').value) || 0;
    const dateInput = document.getElementById('accountDueDate').value;

    if (!name || !type) {
        alert('Please fill required fields');
        return;
    }

    if (name.length > 50) {
        alert('Account name too long (max 50 characters)');
        return;
    }

    let formattedDate = '';
    if (dateInput) {
        const [year, month, day] = dateInput.split('-');
        formattedDate = `${day}-${month}-${year}`;
    }

    const account = {
        account: name,
        type: type,
        amount: amount,
        iinw: iinw,
        budget: budget,
        dueDate: formattedDate
    };

    let result;
    if (editingAccount) {
        result = await apiCall('/api/accounts', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(account)
        });
    } else {
        result = await apiCall('/api/accounts', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(account)
        });
    }

    if (result && result.success) {
        cancelAccount();
        loadAccounts();
        loadDashboard();
    }
}

function cancelAccount() {
    document.getElementById('addAccountForm').style.display = 'none';
    document.getElementById('accountName').value = '';
    document.getElementById('accountName').disabled = false;
    document.getElementById('accountType').value = '';
    document.getElementById('accountAmount').value = '';
    document.querySelector('input[name="iinw"][value="No"]').checked = true;
    document.getElementById('accountBudget').value = '';
    document.getElementById('accountDueDate').value = '';
    document.getElementById('accountBudget').style.display = 'none';
    document.getElementById('accountDueDate').style.display = 'none';
    editingAccount = null;
}

async function editAccount(name) {
    const account = accounts.find(a => a.account === name);
    if (!account) return;

    editingAccount = account;
    
    // Find the account card and replace it with edit form
    const listContainer = document.getElementById('accountList');
    const allCards = listContainer.querySelectorAll('.account-card');
    
    allCards.forEach(card => {
        const cardName = card.querySelector('[data-account]')?.getAttribute('data-account');
        if (cardName === name) {
            let dueDateValue = '';
            if (account.dueDate) {
                const [day, month, year] = account.dueDate.split('-');
                dueDateValue = `${year}-${month}-${day}`;
            }
            
            // Show/hide budget and due date fields based on type
            const showBudget = account.type === 'EXPENSE';
            const showDueDate = account.type === 'LIABILITIES';
            
            card.innerHTML = `
                <div class="account-grid">
                    <input type="text" id="editAccountName" placeholder="Account Name" value="${escapeHtml(account.account)}" required>
                    <select id="editAccountType" required>
                        <option value="ASSET" ${account.type === 'ASSET' ? 'selected' : ''}>Asset</option>
                        <option value="LIABILITIES" ${account.type === 'LIABILITIES' ? 'selected' : ''}>Liabilities</option>
                        <option value="INCOME" ${account.type === 'INCOME' ? 'selected' : ''}>Income</option>
                        <option value="EXPENSE" ${account.type === 'EXPENSE' ? 'selected' : ''}>Expense</option>
                    </select>
                    <input type="number" id="editAccountAmount" placeholder="Amount" step="0.01" value="${account.amount}" required>
                    <div class="radio-group">
                        <label class="radio-label">
                            <input type="radio" name="editIinw" value="Yes" ${account.iinw === 'Yes' ? 'checked' : ''}>
                            <span>In Net Worth</span>
                        </label>
                        <label class="radio-label">
                            <input type="radio" name="editIinw" value="No" ${account.iinw === 'No' ? 'checked' : ''}>
                            <span>Not in Net Worth</span>
                        </label>
                    </div>
                    <input type="number" id="editAccountBudget" placeholder="Budget (for expenses)" step="0.01" value="${account.budget || ''}" style="display: ${showBudget ? 'block' : 'none'};">
                    <input type="date" id="editAccountDueDate" placeholder="Due Date" value="${dueDateValue}" style="display: ${showDueDate ? 'block' : 'none'};">
                    <div class="action-buttons">
                        <button class="btn-icon btn-save" data-action="save-edit-account" title="Save">
                            <span class="material-icons">check</span>
                        </button>
                        <button class="btn-icon btn-cancel" data-action="cancel-edit-account" title="Cancel">
                            <span class="material-icons">close</span>
                        </button>
                    </div>
                </div>
            `;
            
            // Add change listener for account type
            const typeSelect = card.querySelector('#editAccountType');
            typeSelect.addEventListener('change', () => {
                const type = typeSelect.value;
                const budgetField = card.querySelector('#editAccountBudget');
                const dueDateField = card.querySelector('#editAccountDueDate');
                budgetField.style.display = type === 'EXPENSE' ? 'block' : 'none';
                dueDateField.style.display = type === 'LIABILITIES' ? 'block' : 'none';
            });
            
            card.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
    });
}

async function deleteAccount(name) {
    if (!confirm('Are you sure you want to delete this account?')) return;

    const result = await apiCall('/api/accounts', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ account: name })
    });

    if (result && result.success) {
        cancelAccount();
        loadAccounts();
        loadDashboard();
    }
}

async function saveEditAccount() {
    const newName = document.getElementById('editAccountName').value.trim();
    const type = document.getElementById('editAccountType').value;
    const amount = parseFloat(document.getElementById('editAccountAmount').value) || 0;
    const iinw = document.querySelector('input[name="editIinw"]:checked')?.value || 'No';
    const budget = parseFloat(document.getElementById('editAccountBudget').value) || 0;
    const dateInput = document.getElementById('editAccountDueDate').value;

    if (!newName || !type) {
        alert('Please fill required fields');
        return;
    }

    if (newName.length > 50) {
        alert('Account name too long (max 50 characters)');
        return;
    }

    let formattedDate = '';
    if (dateInput) {
        const [year, month, day] = dateInput.split('-');
        formattedDate = `${day}-${month}-${year}`;
    }

    const updateData = {
        oldAccount: editingAccount.account,
        account: newName,
        type: type,
        amount: amount,
        iinw: iinw,
        budget: budget,
        dueDate: formattedDate
    };

    const result = await apiCall('/api/accounts', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updateData)
    });

    if (result && result.success) {
        editingAccount = null;
        loadAccounts();
        loadDashboard();
    }
}

// Settings functions
function loadSettings() {
    const darkMode = localStorage.getItem('darkMode') === 'true';
    const hideAmount = localStorage.getItem('hideAmount') === 'true';
    const theme = localStorage.getItem('theme') || 'purple';

    const darkToggle = document.getElementById('darkModeToggle');
    const hideToggle = document.getElementById('hideAmountToggle');
    
    if (darkToggle) darkToggle.checked = darkMode;
    if (hideToggle) hideToggle.checked = hideAmount;

    if (darkMode) document.body.classList.add('dark-mode');
    if (hideAmount) document.body.classList.add('hide-amounts');
    
    document.body.setAttribute('data-theme', theme);
    
    setTimeout(() => {
        setupColorPicker();
    }, 100);
}

function setupColorPicker() {
    const colorOptions = document.querySelectorAll('.color-option');
    const currentTheme = localStorage.getItem('theme') || 'purple';
    
    colorOptions.forEach(option => {
        if (option.getAttribute('data-theme') === currentTheme) {
            option.classList.add('active');
        }
        
        option.addEventListener('click', () => {
            const theme = option.getAttribute('data-theme');
            changeTheme(theme);
        });
    });
}

function changeTheme(theme) {
    localStorage.setItem('theme', theme);
    document.body.setAttribute('data-theme', theme);
    
    document.querySelectorAll('.color-option').forEach(option => {
        option.classList.remove('active');
        if (option.getAttribute('data-theme') === theme) {
            option.classList.add('active');
        }
    });
    
    if (document.getElementById('dashboard').classList.contains('active')) {
        loadDashboard();
    }
}

function toggleDarkMode() {
    const enabled = document.getElementById('darkModeToggle').checked;
    localStorage.setItem('darkMode', enabled);
    
    if (enabled) {
        document.body.classList.add('dark-mode');
    } else {
        document.body.classList.remove('dark-mode');
    }
}

function toggleHideAmount() {
    const enabled = document.getElementById('hideAmountToggle').checked;
    localStorage.setItem('hideAmount', enabled);
    
    if (enabled) {
        document.body.classList.add('hide-amounts');
    } else {
        document.body.classList.remove('hide-amounts');
    }
}

async function changePassword() {
    const newPassword = document.getElementById('newPassword').value;
    
    if (!newPassword) {
        alert('Please enter a new password');
        return;
    }

    if (newPassword.length < 8) {
        alert('Password must be at least 8 characters');
        return;
    }

    const result = await apiCall('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ newPassword })
    });

    if (result && result.success) {
        alert(result.message || 'Password changed successfully');
        document.getElementById('newPassword').value = '';
    }
}

async function logout() {
    if (!confirm('Are you sure you want to logout?')) {
        return;
    }

    await apiCall('/api/logout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
    });

    localStorage.removeItem('darkMode');
    localStorage.removeItem('hideAmount');
    localStorage.removeItem('theme');
    
    document.getElementById('mainApp').style.display = 'none';
    document.getElementById('loginPage').style.display = 'flex';
    document.getElementById('password').value = '';
    document.getElementById('loginError').textContent = '';
    
    document.body.classList.remove('dark-mode');
    document.body.classList.remove('hide-amounts');
    
    csrfToken = null;
}

// Utility functions
function formatAmount(amount) {
    return (parseFloat(amount) || 0).toFixed(2);
}

function formatDateForInput(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
}