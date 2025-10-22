// Client-side Validation Utilities

const Validation = {
    MAX_INPUT_LENGTH: 500,
    MAX_DESCRIPTION_LENGTH: 1000,
    MAX_AMOUNT: 999999999.99,
    
    sanitizeInput(input) {
        const div = document.createElement('div');
        div.textContent = input;
        return div.innerHTML;
    },
    
    validateInput(input, maxLength) {
        if (!input || input.trim() === '') {
            return { valid: false, error: 'Input is required' };
        }
        
        if (input.length > maxLength) {
            return { valid: false, error: `Input exceeds maximum length of ${maxLength} characters` };
        }
        
        return { valid: true, value: this.sanitizeInput(input.trim()) };
    },
    
    validateAmount(amount) {
        const numAmount = parseFloat(amount);
        
        if (isNaN(numAmount)) {
            return { valid: false, error: 'Invalid amount' };
        }
        
        if (numAmount < 0) {
            return { valid: false, error: 'Amount cannot be negative' };
        }
        
        if (numAmount > this.MAX_AMOUNT) {
            return { valid: false, error: 'Amount exceeds maximum allowed value' };
        }
        
        return { valid: true, value: numAmount };
    },
    
    validateTransaction(txn) {
        if (!txn.from) {
            throw new Error('Please select From account');
        }
        
        if (!txn.to) {
            throw new Error('Please select To account');
        }
        
        const amountValidation = this.validateAmount(txn.amount);
        if (!amountValidation.valid) {
            throw new Error(amountValidation.error);
        }
        
        const descValidation = this.validateInput(txn.description, this.MAX_DESCRIPTION_LENGTH);
        if (!descValidation.valid) {
            throw new Error(descValidation.error);
        }
        
        if (!txn.date || !txn.time) {
            throw new Error('Please select date and time');
        }
        
        return {
            ...txn,
            description: descValidation.value,
            amount: amountValidation.value
        };
    },
    
    validateAccount(account) {
        const nameValidation = this.validateInput(account.name, this.MAX_INPUT_LENGTH);
        if (!nameValidation.valid) {
            throw new Error(nameValidation.error);
        }
        
        if (!account.category) {
            throw new Error('Please select account type');
        }
        
        if (account.budget) {
            const budgetValidation = this.validateAmount(account.budget);
            if (!budgetValidation.valid) {
                throw new Error(budgetValidation.error);
            }
            account.budget = budgetValidation.value;
        }
        
        account.name = nameValidation.value;
        return account;
    },
    
    validateRecurrence(rec) {
        const dayOfMonth = parseInt(rec.dayOfMonth);
        
        if (!dayOfMonth || dayOfMonth < 1 || dayOfMonth > 31) {
            throw new Error('Please enter a valid day of month (1-31)');
        }
        
        if (!rec.from || !rec.to) {
            throw new Error('Please select both From and To accounts');
        }
        
        const descValidation = this.validateInput(rec.description, this.MAX_DESCRIPTION_LENGTH);
        if (!descValidation.valid) {
            throw new Error(descValidation.error);
        }
        
        const amountValidation = this.validateAmount(rec.amount);
        if (!amountValidation.valid) {
            throw new Error(amountValidation.error);
        }
        
        return {
            ...rec,
            description: descValidation.value,
            amount: amountValidation.value
        };
    },
    
    validateNote(note) {
        const headingValidation = this.validateInput(note.heading, this.MAX_INPUT_LENGTH);
        if (!headingValidation.valid) {
            throw new Error(headingValidation.error);
        }
        
        const contentValidation = this.validateInput(note.content, this.MAX_DESCRIPTION_LENGTH * 2);
        if (!contentValidation.valid) {
            throw new Error(contentValidation.error);
        }
        
        return {
            ...note,
            heading: headingValidation.value,
            content: contentValidation.value
        };
    }
};