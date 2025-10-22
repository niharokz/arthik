// Authentication Logic

async function login() {
    const password = document.getElementById('passwordInput').value;
    
    if (!password) {
        alert('Please enter a password');
        return;
    }
    
    try {
        const data = await API.login(password);
        
        if (data.success && data.token) {
            setAuthToken(data.token);
            
            document.getElementById('loginScreen').classList.add('hidden');
            document.getElementById('mainApp').classList.remove('hidden');
            document.getElementById('passwordInput').value = '';
            
            await loadInitialData();
        } else {
            alert(data.message || 'Invalid password');
        }
    } catch (error) {
        console.error('Login error:', error);
        alert(error.message || 'Login failed. Please try again.');
    }
}

async function logout() {
    if (confirm('Are you sure you want to logout?')) {
        resetState();
        
        document.getElementById('mainApp').classList.add('hidden');
        document.getElementById('loginScreen').classList.remove('hidden');
        document.getElementById('passwordInput').value = '';
        
        closeSettingsDropdown();
        showTab('dashboard');
    }
}

async function loadInitialData() {
    try {
        await loadAccounts();
        await loadDashboard();
        await loadTransactions();
    } catch (error) {
        console.error('Error loading initial data:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to load data');
        }
    }
}

function setupLoginListener() {
    const passwordInput = document.getElementById('passwordInput');
    const loginButton = document.getElementById('loginButton');
    
    if (passwordInput) {
        passwordInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                login();
            }
        });
    }
    
    if (loginButton) {
        loginButton.addEventListener('click', login);
    }
}