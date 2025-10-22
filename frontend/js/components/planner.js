// Planner Component - Recurrences and Notes

// ===== RECURRENCES =====

async function loadRecurrences() {
    try {
        const recurrences = await API.getRecurrences();
        setRecurrences(recurrences);
        displayRecurrences();
    } catch (error) {
        console.error('Error loading recurrences:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to load recurrences');
        }
    }
}

function displayRecurrences() {
    const recurrences = AppState.recurrences;
    const arrow = Helpers.getArrow();
    
    const html = recurrences.map(rec => {
        const nextDate = new Date(rec.nextDate);
        const dateStr = nextDate.toLocaleDateString('en-IN', { day: '2-digit', month: 'short' });
        
        return `
            <li class="recurrence-item">
                <div class="recurrence-date">${dateStr}</div>
                <div class="recurrence-info">
                    <div class="recurrence-desc">${Helpers.sanitizeHTML(rec.description)}</div>
                    <div class="recurrence-meta">${Helpers.sanitizeHTML(rec.from)} ${arrow} ${Helpers.sanitizeHTML(rec.to)} | Day ${rec.dayOfMonth} of month</div>
                </div>
                <div class="recurrence-amount">Rs ${rec.amount.toFixed(2)}</div>
                <div class="recurrence-actions">
                    <button class="btn-icon btn-apply" onclick="applyRecurrence('${rec.id}')" title="Apply Now">
                        <span class="material-icons">play_arrow</span>
                    </button>
                    <button class="btn-icon delete" onclick="deleteRecurrence('${rec.id}')" title="Delete">
                        <span class="material-icons">delete</span>
                    </button>
                </div>
            </li>
        `;
    }).join('');
    
    document.getElementById('recurrenceList').innerHTML = html || 
        '<li style="text-align: center; padding: 2rem; color: var(--text-secondary);">No recurring transactions yet</li>';
}

function toggleAddRecurrence() {
    const form = document.getElementById('newRecurrenceForm');
    form.classList.toggle('hidden');
    
    if (!form.classList.contains('hidden')) {
        const accounts = getAccounts();
        const accountOptions = accounts.map(a => 
            `<option value="${Helpers.sanitizeHTML(a.name)}">${Helpers.sanitizeHTML(a.name)} (${a.category})</option>`
        ).join('');
        
        document.getElementById('recFrom').innerHTML = '<option value="">From Account</option>' + accountOptions;
        document.getElementById('recTo').innerHTML = '<option value="">To Account</option>' + accountOptions;
        
        window.scrollTo({ top: 0, behavior: 'smooth' });
    }
}

async function saveRecurrence() {
    try {
        const recurrence = {
            dayOfMonth: document.getElementById('recDayOfMonth').value,
            from: document.getElementById('recFrom').value,
            to: document.getElementById('recTo').value,
            description: document.getElementById('recDesc').value,
            amount: parseFloat(document.getElementById('recAmount').value)
        };
        
        const validatedRec = Validation.validateRecurrence(recurrence);
        
        const now = new Date();
        let nextDate = new Date(now.getFullYear(), now.getMonth(), validatedRec.dayOfMonth);
        
        if (nextDate < now) {
            nextDate = new Date(now.getFullYear(), now.getMonth() + 1, validatedRec.dayOfMonth);
        }
        
        validatedRec.nextDate = nextDate.toISOString().split('T')[0];
        validatedRec.id = '';
        
        await API.saveRecurrence(validatedRec);
        cancelAddRecurrence();
        await loadRecurrences();
    } catch (error) {
        console.error('Error saving recurrence:', error);
        if (error.message !== 'Authentication failed') {
            alert(error.message || 'Failed to save recurrence');
        }
    }
}

function cancelAddRecurrence() {
    document.getElementById('recDayOfMonth').value = '';
    document.getElementById('recFrom').value = '';
    document.getElementById('recTo').value = '';
    document.getElementById('recDesc').value = '';
    document.getElementById('recAmount').value = '';
    Helpers.hide('newRecurrenceForm');
}

async function applyRecurrence(id) {
    if (!confirm('Apply this recurring transaction now?')) {
        return;
    }
    
    try {
        await API.applyRecurrence(id);
        alert('Transaction applied successfully!');
        await Promise.all([
            loadRecurrences(),
            loadAccounts(),
            loadTransactions(),
            loadDashboard()
        ]);
    } catch (error) {
        console.error('Error applying recurrence:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to apply recurrence');
        }
    }
}

async function deleteRecurrence(id) {
    if (!confirm('Are you sure you want to delete this recurring transaction?')) {
        return;
    }
    
    try {
        await API.deleteRecurrence(id);
        await loadRecurrences();
    } catch (error) {
        console.error('Error deleting recurrence:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to delete recurrence');
        }
    }
}

// ===== NOTES =====

async function loadNotes() {
    try {
        const notes = await API.getNotes();
        setNotes(notes);
        displayNotes();
    } catch (error) {
        console.error('Error loading notes:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to load notes');
        }
    }
}

function displayNotes() {
    const notes = AppState.notes;
    
    const html = notes.map(note => {
        const created = new Date(note.created);
        const dateStr = created.toLocaleDateString('en-IN', { 
            day: '2-digit', 
            month: 'short', 
            year: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
        
        return `
            <div class="note-card">
                <div class="note-header">
                    <div>
                        <h4 class="note-title">${Helpers.sanitizeHTML(note.heading)}</h4>
                        <div class="note-date">${dateStr}</div>
                    </div>
                    <div class="note-actions">
                        <button class="btn-icon" onclick="editNote('${note.id}')" title="Edit">
                            <span class="material-icons">edit</span>
                        </button>
                        <button class="btn-icon delete" onclick="deleteNote('${note.id}')" title="Delete">
                            <span class="material-icons">delete</span>
                        </button>
                    </div>
                </div>
                <div class="note-content">${Helpers.sanitizeHTML(note.content)}</div>
            </div>
        `;
    }).join('');
    
    document.getElementById('notesList').innerHTML = html || 
        '<p style="text-align: center; color: var(--text-secondary); padding: 2rem;">No notes yet.</p>';
}

function toggleAddNote() {
    Helpers.toggle('newNoteForm');
    
    if (!document.getElementById('newNoteForm').classList.contains('hidden')) {
        document.getElementById('noteHeading').value = '';
        document.getElementById('noteContent').value = '';
        setEditingNoteId(null);
    }
}

async function saveNote() {
    try {
        const note = {
            heading: document.getElementById('noteHeading').value,
            content: document.getElementById('noteContent').value
        };
        
        const validatedNote = Validation.validateNote(note);
        
        const editingId = getEditingNoteId();
        if (editingId) {
            validatedNote.id = editingId;
        } else {
            validatedNote.id = '';
            validatedNote.created = new Date().toISOString();
        }
        
        await API.saveNote(validatedNote);
        cancelNote();
        await loadNotes();
    } catch (error) {
        console.error('Error saving note:', error);
        if (error.message !== 'Authentication failed') {
            alert(error.message || 'Failed to save note');
        }
    }
}

function cancelNote() {
    document.getElementById('noteHeading').value = '';
    document.getElementById('noteContent').value = '';
    Helpers.hide('newNoteForm');
    setEditingNoteId(null);
}

function editNote(id) {
    const notes = AppState.notes;
    const note = notes.find(n => n.id === id);
    if (!note) return;
    
    setEditingNoteId(id);
    document.getElementById('noteHeading').value = note.heading;
    document.getElementById('noteContent').value = note.content;
    Helpers.show('newNoteForm');
    
    window.scrollTo({ 
        top: document.getElementById('newNoteForm').offsetTop - 100, 
        behavior: 'smooth' 
    });
}

async function deleteNote(id) {
    if (!confirm('Are you sure you want to delete this note?')) {
        return;
    }
    
    try {
        await API.deleteNote(id);
        await loadNotes();
    } catch (error) {
        console.error('Error deleting note:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to delete note');
        }
    }
}

function setupPlannerListeners() {
    const fabAddRecurrence = document.getElementById('fabAddRecurrence');
    const saveRecurrenceBtn = document.getElementById('saveRecurrenceBtn');
    const cancelRecurrenceBtn = document.getElementById('cancelRecurrenceBtn');
    const addNoteBtn = document.getElementById('addNoteBtn');
    
    if (fabAddRecurrence) fabAddRecurrence.addEventListener('click', toggleAddRecurrence);
    if (saveRecurrenceBtn) saveRecurrenceBtn.addEventListener('click', saveRecurrence);
    if (cancelRecurrenceBtn) cancelRecurrenceBtn.addEventListener('click', cancelAddRecurrence);
    if (addNoteBtn) addNoteBtn.addEventListener('click', toggleAddNote);
}