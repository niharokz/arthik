// Accounts Component

async function loadAccounts() {
    try {
        const accounts = await API.getAccounts();
        setAccounts(accounts);
        displayAccounts();
        populateAccountDropdowns();
    } catch (error) {
        console.error('Error loading accounts:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to load accounts');
        }
    }
}

function displayAccounts() {
    const accounts = getAccounts();
    const categories = ['Assets', 'Liabilities', 'Equity', 'Revenue', 'Expenses'];
    const editingAccountName = AppState.editingAccountName; // Add this line
    
    const html = categories.map(cat => {
        const catAccounts = accounts.filter(a => a.category === cat);
        if (catAccounts.length === 0) return '';
        
        const accountsHtml = catAccounts.map((acc, idx) => {
            const accId = `${cat}_${idx}`;
            
            if (editingAccountName === acc.name) { // Use the local variable
                return renderEditAccountForm(acc, accId);
            }
            
            return renderAccountItem(acc);
        }).join('');
        
        return `<div class="card"><h3>${cat}</h3>${accountsHtml}</div>`;
    }).join('');
    
    document.getElementById('accountsList').innerHTML = html;
}

function renderAccountItem(acc) {
    let additionalDetails = '';
    
    if (acc.category === 'Liabilities' && (acc.dueDate || acc.lastPaymentDate)) {
        additionalDetails = '<div style="font-size: 0.75rem; margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid var(--divider);">';
        if (acc.dueDate) {
            additionalDetails += `<div style="display: flex; justify-content: space-between; margin-bottom: 0.25rem;">
                <span style="color: var(--text-secondary);">Next Due Date:</span>
                <strong style="color: var(--error);">${acc.dueDate}</strong>
            </div>`;
        }
        if (acc.lastPaymentDate) {
            additionalDetails += `<div style="display: flex; justify-content: space-between;">
                <span style="color: var(--text-secondary);">Last Payment:</span>
                <strong style="color: var(--text-primary);">${acc.lastPaymentDate}</strong>
            </div>`;
        }
        additionalDetails += '</div>';
    } else if (acc.category === 'Expenses' && acc.budget && acc.budget > 0) {
        additionalDetails = `<div style="font-size: 0.75rem; margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid var(--divider);">
            <div style="display: flex; justify-content: space-between;">
                <span style="color: var(--text-secondary);">Monthly Budget:</span>
                <strong style="color: var(--warning);">Rs ${acc.budget.toFixed(2)}</strong>
            </div>
        </div>`;
    }
    
    return `
        <div class="transaction-item">
            <div class="transaction-icon">
                <span class="material-icons">account_balance_wallet</span>
            </div>
            <div class="transaction-info" style="flex: 1;">
                <div class="transaction-desc">${Helpers.sanitizeHTML(acc.name)}</div>
                <div class="transaction-meta">${acc.includeInNetWorth ? 'Included in Net Worth' : 'Not included'}</div>
                ${additionalDetails}
            </div>
            <div class="transaction-amount">Rs ${acc.currentBalance.toFixed(2)}</div>
            <button class="btn-icon" onclick="editAccount('${acc.name.replace(/'/g, "\\'")}')">
                <span class="material-icons">edit</span>
            </button>
        </div>
    `;
}

function renderEditAccountForm(acc, accId) {
    let conditionalFields = '';
    
    if (acc.category === 'Liabilities') {
        conditionalFields = `
            <div style="width: 100%; display: flex; gap: 0.5rem; flex-wrap: wrap;">
                <div style="flex: 1; min-width: 150px;">
                    <label style="display: block; font-size: 0.75rem; margin-bottom: 0.25rem;">Next Due Date</label>
                    <input type="date" id="editAccDueDate_${accId}" value="${acc.dueDate || ''}" style="margin: 0;">
                </div>
                <div style="flex: 1; min-width: 150px;">
                    <label style="display: block; font-size: 0.75rem; margin-bottom: 0.25rem;">Last Payment Date</label>
                    <input type="date" id="editAccLastPayment_${accId}" value="${acc.lastPaymentDate || ''}" style="margin: 0;">
                </div>
            </div>
        `;
    } else if (acc.category === 'Expenses') {
        conditionalFields = `
            <div style="width: 100%; display: flex; gap: 0.5rem;">
                <div style="flex: 1; min-width: 150px;">
                    <label style="display: block; font-size: 0.75rem; margin-bottom: 0.25rem;">Monthly Budget (Rs)</label>
                    <input type="number" id="editAccBudget_${accId}" value="${acc.budget || 0}" step="0.01" min="0" style="margin: 0;">
                </div>
            </div>
        `;
    }
    
    return `
        <div class="transaction-item edit-mode">
            <div class="edit-form" style="margin: 0; flex: 1;">
                <input type="text" id="editAccName_${accId}" value="${Helpers.sanitizeHTML(acc.name)}" placeholder="Account Name" style="flex: 2;">
                <div style="display: flex; align-items: center; gap: 0.5rem;">
                    <input type="checkbox" id="editAccInclude_${accId}" ${acc.includeInNetWorth ? 'checked' : ''} style="width: auto;">
                    <span style="font-size: 0.875rem; white-space: nowrap;">In Net Worth</span>
                </div>
                ${conditionalFields}
                <div class="edit-actions">
                    <button class="btn-icon save" onclick="saveAccountEdit('${acc.name.replace(/'/g, "\\'")}', '${accId}')">
                        <span class="material-icons">check</span>
                    </button>
                    <button class="btn-icon delete" onclick="deleteAccount('${acc.name.replace(/'/g, "\\'")}')">
                        <span class="material-icons">delete</span>
                    </button>
                    <button class="btn-icon cancel" onclick="cancelAccountEdit()">
                        <span class="material-icons">close</span>
                    </button>
                </div>
            </div>
        </div>
    `;
}

function populateAccountDropdowns() {
    const accounts = getAccounts();
    const accountOptions = accounts.map(a => 
        `<option value="${Helpers.sanitizeHTML(a.name)}">${Helpers.sanitizeHTML(a.name)} (${a.category})</option>`
    ).join('');
    
    const txnFrom = document.getElementById('txnFrom');
    const txnTo = document.getElementById('txnTo');
    
    if (txnFrom) {
        txnFrom.innerHTML = '<option value="">Select From Account</option>' + accountOptions;
    }
    if (txnTo) {
        txnTo.innerHTML = '<option value="">Select To Account</option>' + accountOptions;
    }
}

function toggleAddAccount() {
    Helpers.toggle('newAccountForm');
    if (!document.getElementById('newAccountForm').classList.contains('hidden')) {
        window.scrollTo({ top: 0, behavior: 'smooth' });
    }
}

function handleCategoryChange() {
    const category = document.getElementById('accCategory').value;
    const liabilitiesFields = document.getElementById('liabilitiesFields');
    const expensesFields = document.getElementById('expensesFields');
    
    if (liabilitiesFields) {
        liabilitiesFields.classList.add('hidden');
        liabilitiesFields.style.display = 'none';
    }
    if (expensesFields) {
        expensesFields.classList.add('hidden');
        expensesFields.style.display = 'none';
    }
    
    if (category === 'Liabilities' && liabilitiesFields) {
        liabilitiesFields.classList.remove('hidden');
        liabilitiesFields.style.display = 'flex';
    } else if (category === 'Expenses' && expensesFields) {
        expensesFields.classList.remove('hidden');
        expensesFields.style.display = 'flex';
    }
}

async function addAccount() {
    try {
        const account = {
            name: document.getElementById('accName').value,
            category: document.getElementById('accCategory').value,
            includeInNetWorth: document.getElementById('accInclude').checked,
            currentBalance: 0
        };
        
        const validatedAccount = Validation.validateAccount(account);
        
        if (account.category === 'Liabilities') {
            validatedAccount.dueDate = document.getElementById('accDueDate').value || '';
            validatedAccount.lastPaymentDate = document.getElementById('accLastPaymentDate').value || '';
        } else if (account.category === 'Expenses') {
            const budget = parseFloat(document.getElementById('accBudget').value) || 0;
            validatedAccount.budget = budget;
        }
        
        await API.addAccount(validatedAccount);
        cancelAddAccount();
        await loadAccounts();
        alert('Account added successfully!');
    } catch (error) {
        console.error('Error adding account:', error);
        if (error.message !== 'Authentication failed') {
            alert(error.message || 'Failed to add account');
        }
    }
}

function cancelAddAccount() {
    document.getElementById('accName').value = '';
    document.getElementById('accCategory').value = '';
    document.getElementById('accInclude').checked = true;
    document.getElementById('accDueDate').value = '';
    document.getElementById('accLastPaymentDate').value = '';
    document.getElementById('accBudget').value = '';
    
    const liabilitiesFields = document.getElementById('liabilitiesFields');
    const expensesFields = document.getElementById('expensesFields');
    
    if (liabilitiesFields) {
        liabilitiesFields.classList.add('hidden');
        liabilitiesFields.style.display = 'none';
    }
    if (expensesFields) {
        expensesFields.classList.add('hidden');
        expensesFields.style.display = 'none';
    }
    
    Helpers.hide('newAccountForm');
}

function editAccount(name) {
    setEditingAccountName(name);
    loadAccounts();
}

function cancelAccountEdit() {
    setEditingAccountName(null);
    loadAccounts();
}

async function saveAccountEdit(oldName, accId) {
    try {
        const accounts = getAccounts();
        const acc = accounts.find(a => a.name === oldName);
        if (!acc) return;
        
        const newName = document.getElementById(`editAccName_${accId}`).value;
        const includeInNetWorth = document.getElementById(`editAccInclude_${accId}`).checked;
        
        // Delete old account
        await API.deleteAccount(oldName);
        
        // Create updated account
        const updatedAccount = {
            name: newName,
            category: acc.category,
            includeInNetWorth: includeInNetWorth,
            currentBalance: acc.currentBalance
        };
        
        if (acc.category === 'Liabilities') {
            const dueDateEl = document.getElementById(`editAccDueDate_${accId}`);
            const lastPaymentEl = document.getElementById(`editAccLastPayment_${accId}`);
            updatedAccount.dueDate = dueDateEl ? dueDateEl.value : (acc.dueDate || '');
            updatedAccount.lastPaymentDate = lastPaymentEl ? lastPaymentEl.value : (acc.lastPaymentDate || '');
        } else if (acc.category === 'Expenses') {
            const budgetEl = document.getElementById(`editAccBudget_${accId}`);
            updatedAccount.budget = budgetEl ? parseFloat(budgetEl.value) || 0 : (acc.budget || 0);
        }
        
        await API.addAccount(updatedAccount);
        
        window.editingAccountName = null;
        await Promise.all([
            loadAccounts(),
            loadDashboard()
        ]);
    } catch (error) {
        console.error('Error saving account:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to save account changes');
        }
    }
}

async function deleteAccount(name) {
    if (!confirm(`Are you sure you want to delete "${name}"? This cannot be undone.`)) {
        return;
    }
    
    try {
        await API.deleteAccount(name);
        window.editingAccountName = null;
        await Promise.all([
            loadAccounts(),
            loadDashboard()
        ]);
    } catch (error) {
        console.error('Error deleting account:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to delete account');
        }
    }
}

function setupAccountListeners() {
    const fabAddAccount = document.getElementById('fabAddAccount');
    const saveAccountBtn = document.getElementById('saveAccountBtn');
    const cancelAddAccountBtn = document.getElementById('cancelAddAccountBtn');
    const accCategory = document.getElementById('accCategory');
    
    if (fabAddAccount) fabAddAccount.addEventListener('click', toggleAddAccount);
    if (saveAccountBtn) saveAccountBtn.addEventListener('click', addAccount);
    if (cancelAddAccountBtn) cancelAddAccountBtn.addEventListener('click', cancelAddAccount);
    if (accCategory) accCategory.addEventListener('change', handleCategoryChange);
}