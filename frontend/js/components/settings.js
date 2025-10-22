// Settings Component

function loadTheme() {
    const savedTheme = localStorage.getItem('arthik-theme') || 'light';
    applyTheme(savedTheme);
}

function changeTheme(theme) {
    applyTheme(theme);
    localStorage.setItem('arthik-theme', theme);
    
    if (isAuthenticated()) {
        API.updateSettings({ theme: theme }).catch(error => {
            console.error('Error saving theme:', error);
        });
    }
}

function applyTheme(theme) {
    if (theme === 'dark') {
        document.body.classList.add('dark-theme');
    } else {
        document.body.classList.remove('dark-theme');
    }
    updateThemeSelection();
}

function updateThemeSelection() {
    const currentTheme = document.body.classList.contains('dark-theme') ? 'dark' : 'light';
    document.querySelectorAll('.theme-btn').forEach(btn => {
        if (btn.dataset.theme === currentTheme) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });
}

function toggleSettingsDropdown() {
    const dropdown = document.getElementById('settingsDropdown');
    dropdown.classList.toggle('show');
    updateThemeSelection();
}

function closeSettingsDropdown() {
    const dropdown = document.getElementById('settingsDropdown');
    dropdown.classList.remove('show');
}

function showChangePasswordModal() {
    closeSettingsDropdown();
    document.getElementById('changePasswordModal').style.display = 'block';
}

function closeChangePasswordModal() {
    document.getElementById('changePasswordModal').style.display = 'none';
    document.getElementById('changePasswordForm').reset();
}

async function changePassword(event) {
    event.preventDefault();
    
    const oldPassword = document.getElementById('oldPassword').value;
    const newPassword = document.getElementById('newPassword').value;
    const confirmPassword = document.getElementById('confirmPassword').value;
    
    if (newPassword !== confirmPassword) {
        alert('New passwords do not match!');
        return;
    }
    
    if (newPassword.length < 6) {
        alert('Password must be at least 6 characters long');
        return;
    }
    
    try {
        const data = await API.updateSettings({
            oldPassword: oldPassword,
            newPassword: newPassword
        });
        
        if (data.success) {
            alert('Password changed successfully!');
            closeChangePasswordModal();
        } else {
            alert(data.error || 'Failed to change password.');
        }
    } catch (error) {
        console.error('Error changing password:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to change password');
        }
    }
}

function setupSettingsListeners() {
    const settingsBtn = document.getElementById('settingsBtn');
    
    if (settingsBtn) {
        settingsBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            toggleSettingsDropdown();
        });
    }
    
    document.addEventListener('click', function(event) {
        const settingsContainer = document.querySelector('.settings-container');
        const passwordModal = document.getElementById('changePasswordModal');
        
        if (settingsContainer && !settingsContainer.contains(event.target)) {
            closeSettingsDropdown();
        }
        
        if (event.target === passwordModal) {
            closeChangePasswordModal();
        }
    });
}