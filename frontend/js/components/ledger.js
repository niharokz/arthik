// Ledger/Transactions Component

async function loadTransactions() {
    try {
        const transactions = await API.getTransactions();
        setTransactions(transactions);
        setCurrentPage(1); // Reset to page 1 when loading transactions
        displayTransactions();
    } catch (error) {
        console.error('Error loading transactions:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to load transactions');
        }
    }
}

function displayTransactions() {
    const transactions = getTransactions();
    const currentPage = getCurrentPage();
    const itemsPerPage = 100;
    const editingId = getEditingTransactionId(); 
    
    const start = (currentPage - 1) * itemsPerPage;
    const end = start + itemsPerPage;
    const pageTransactions = transactions.slice(start, end);
    
    const arrow = Helpers.getArrow();
    
    const html = pageTransactions.map((t, index) => {
        const globalIndex = start + index;
        const txnDate = new Date(t.date);
        const dateStr = Helpers.formatDate(t.date);
        const timeStr = Helpers.formatTime(t.date);
        
        if (editingId === t.id) {
            return renderEditTransactionForm(t, globalIndex, txnDate);
        }
        
        return `
            <li class="transaction-item">
                <div class="transaction-icon">
                    <span class="material-icons">swap_horiz</span>
                </div>
                <div class="transaction-info">
                    <div class="transaction-desc">${Helpers.sanitizeHTML(t.description)}</div>
                    <div class="transaction-meta">${dateStr} ${timeStr} | ${Helpers.sanitizeHTML(t.from)} ${arrow} ${Helpers.sanitizeHTML(t.to)}</div>
                </div>
                <div class="transaction-amount ${t.amount > 0 ? 'positive' : 'negative'}">
                    Rs ${Math.abs(t.amount).toFixed(2)}
                </div>
                <button class="btn-icon" onclick="editTransaction('${t.id}')">
                    <span class="material-icons">edit</span>
                </button>
            </li>
        `;
    }).join('');
    
    document.getElementById('transactionsList').innerHTML = html || 
        '<li style="text-align: center; padding: 2rem; color: var(--text-secondary);">No transactions yet</li>';
    
    updatePagination(transactions.length, itemsPerPage);
}

function renderEditTransactionForm(t, index, txnDate) {
    const editDateStr = txnDate.toISOString().split('T')[0];
    const editTimeStr = txnDate.toTimeString().slice(0, 5);
    const accounts = getAccounts();
    
    return `
        <li class="edit-mode">
            <div class="edit-form" style="margin: 0;">
                <input type="date" id="editDate${index}" value="${editDateStr}" required>
                <input type="time" id="editTime${index}" value="${editTimeStr}" required>
                <select id="editFrom${index}">
                    <option value="">From</option>
                    ${accounts.map(a => `<option value="${Helpers.sanitizeHTML(a.name)}" ${a.name === t.from ? 'selected' : ''}>${Helpers.sanitizeHTML(a.name)}</option>`).join('')}
                </select>
                <select id="editTo${index}">
                    <option value="">To</option>
                    ${accounts.map(a => `<option value="${Helpers.sanitizeHTML(a.name)}" ${a.name === t.to ? 'selected' : ''}>${Helpers.sanitizeHTML(a.name)}</option>`).join('')}
                </select>
                <input type="text" id="editDesc${index}" value="${Helpers.sanitizeHTML(t.description)}" placeholder="Description">
                <input type="number" id="editAmount${index}" value="${t.amount}" step="0.01" placeholder="Amount">
                <div class="edit-actions">
                    <button class="btn-icon save" onclick="updateTransaction('${t.id}', ${index})" title="Save">
                        <span class="material-icons">check</span>
                    </button>
                    <button class="btn-icon delete" onclick="deleteTransaction('${t.id}')" title="Delete">
                        <span class="material-icons">delete</span>
                    </button>
                    <button class="btn-icon cancel" onclick="cancelEdit()" title="Cancel">
                        <span class="material-icons">close</span>
                    </button>
                </div>
            </div>
        </li>
    `;
}

function updatePagination(totalItems, itemsPerPage) {
    const currentPage = getCurrentPage();
    const totalPages = Math.ceil(totalItems / itemsPerPage) || 1;
    
    document.getElementById('pageInfo').textContent = `Page ${currentPage} of ${totalPages}`;
    
    const prevBtn = document.getElementById('prevPageBtn');
    const nextBtn = document.getElementById('nextPageBtn');
    
    if (prevBtn) {
        prevBtn.disabled = currentPage <= 1;
        prevBtn.style.opacity = currentPage <= 1 ? '0.5' : '1';
        prevBtn.style.cursor = currentPage <= 1 ? 'not-allowed' : 'pointer';
    }
    
    if (nextBtn) {
        nextBtn.disabled = currentPage >= totalPages;
        nextBtn.style.opacity = currentPage >= totalPages ? '0.5' : '1';
        nextBtn.style.cursor = currentPage >= totalPages ? 'not-allowed' : 'pointer';
    }
}

function toggleNewTransaction() {
    const form = document.getElementById('newTransactionForm');
    form.classList.toggle('hidden');
    
    if (!form.classList.contains('hidden')) {
        document.getElementById('txnDate').value = Helpers.getCurrentDate();
        document.getElementById('txnTime').value = Helpers.getCurrentTime();
        window.scrollTo({ top: 0, behavior: 'smooth' });
    }
}

async function saveNewTransaction() {
    try {
        const transaction = {
            from: document.getElementById('txnFrom').value,
            to: document.getElementById('txnTo').value,
            description: document.getElementById('txnDesc').value,
            amount: parseFloat(document.getElementById('txnAmount').value),
            date: document.getElementById('txnDate').value,
            time: document.getElementById('txnTime').value
        };
        
        const validatedTransaction = Validation.validateTransaction(transaction);
        
        await API.saveTransaction(validatedTransaction);
        cancelNewTransaction();
        
        await Promise.all([
            loadTransactions(),
            loadAccounts(),
            loadDashboard()
        ]);
    } catch (error) {
        console.error('Error saving transaction:', error);
        if (error.message !== 'Authentication failed') {
            alert(error.message || 'Failed to save transaction');
        }
    }
}

function cancelNewTransaction() {
    document.getElementById('txnDate').value = '';
    document.getElementById('txnTime').value = '';
    document.getElementById('txnFrom').value = '';
    document.getElementById('txnTo').value = '';
    document.getElementById('txnDesc').value = '';
    document.getElementById('txnAmount').value = '';
    Helpers.hide('newTransactionForm');
}

function editTransaction(id) {
    setEditingTransactionId(id);
    displayTransactions();
}

function cancelEdit() {
    setEditingTransactionId(null);
    displayTransactions();
}

async function updateTransaction(id, index) {
    try {
        const transaction = {
            id: id,
            from: document.getElementById(`editFrom${index}`).value,
            to: document.getElementById(`editTo${index}`).value,
            description: document.getElementById(`editDesc${index}`).value,
            amount: parseFloat(document.getElementById(`editAmount${index}`).value),
            date: document.getElementById(`editDate${index}`).value,
            time: document.getElementById(`editTime${index}`).value
        };
        
        const validatedTransaction = Validation.validateTransaction(transaction);
        validatedTransaction.id = id;
        
        await API.saveTransaction(validatedTransaction);
        setEditingTransactionId(null);
        
        await Promise.all([
            loadTransactions(),
            loadAccounts(),
            loadDashboard()
        ]);
    } catch (error) {
        console.error('Error updating transaction:', error);
        if (error.message !== 'Authentication failed') {
            alert(error.message || 'Failed to update transaction');
        }
    }
}

async function deleteTransaction(id) {
    if (!confirm('Are you sure you want to delete this transaction?')) {
        return;
    }
    
    try {
        await API.deleteTransaction(id);
        setEditingTransactionId(null);
        
        await Promise.all([
            loadTransactions(),
            loadAccounts(),
            loadDashboard()
        ]);
    } catch (error) {
        console.error('Error deleting transaction:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to delete transaction');
        }
    }
}

function changePage(delta) {
    const transactions = getTransactions();
    const currentPage = getCurrentPage();
    const totalPages = Math.ceil(transactions.length / 100) || 1;
    const newPage = currentPage + delta;
    
    if (newPage >= 1 && newPage <= totalPages) {
        setCurrentPage(newPage);
        displayTransactions();
        Helpers.scrollTo('transactionsList');
    }
}

function setupTransactionListeners() {
    const fabButton = document.getElementById('fabButton');
    const saveTxnBtn = document.getElementById('saveTxnBtn');
    const cancelTxnBtn = document.getElementById('cancelTxnBtn');
    const prevPageBtn = document.getElementById('prevPageBtn');
    const nextPageBtn = document.getElementById('nextPageBtn');
    
    if (fabButton) fabButton.addEventListener('click', toggleNewTransaction);
    if (saveTxnBtn) saveTxnBtn.addEventListener('click', saveNewTransaction);
    if (cancelTxnBtn) cancelTxnBtn.addEventListener('click', cancelNewTransaction);
    if (prevPageBtn) prevPageBtn.addEventListener('click', () => changePage(-1));
    if (nextPageBtn) nextPageBtn.addEventListener('click', () => changePage(1));
}