# ARTHIK - Personal Finance Dashboard

**Version 0.9**

A complete Material Design personal finance management system with Go backend and vanilla JavaScript frontend.

**Website:** [arthik.nihars.com](https://arthik.nihars.com)  
**Live Demo (Read-Only):** [demoarthik.nihars.com](https://demoarthik.nihars.com)  
**License:** MIT License (Open Source)

## Quick Start

```bash
cd arthik
go build -o arthik main.go
./arthik
```

Open browser: http://localhost:8080  
Default password: **admin123**

## Features

### Dashboard Tab
- Large net worth display with assets and liabilities breakdown
- Net worth trend chart (multi-line: net worth, assets, liabilities, expenses)
- Budget vs expenses with visual progress bar
- All accounts as colored pills
- Investment portfolio pie chart
- Upcoming bills (30 days, color-coded by urgency)

### Ledger Tab
- Add/edit/delete transactions
- Automatic account balance updates
- Auto-sort by date and time
- Pagination (30 per page)
- Backdated transaction support
- Cross-year transaction management

### Account Tab
- Four account types: ASSET, LIABILITIES, INCOME, EXPENSE
- Net worth inclusion toggle
- Budget field for expense accounts
- Due date field for liabilities
- Automatic balance calculation

### Settings Tab
- Dark/light mode toggle
- Theme color selection (6 colors)
- Hide/show amounts toggle
- Change password

## Project Structure

```
arthik/
├── main.go              # Go backend server
├── go.mod               # Go module file
├── frontend/
│   ├── index.html       # Material Design UI
│   ├── app.js          # Frontend logic
│   └── style.css       # Material Design CSS
├── data/
│   ├── account.csv     # Account master data
│   ├── tran_2025.csv   # Current year transactions
│   └── record.csv      # Historical daily records
└── logs/               # Server and batch logs
```

## Data Files

All data is stored in human-readable CSV format for easy manual editing.

**account.csv**
```csv
Account,Type,Amount,IINW,Budget,DueDate
Salary,INCOME,-1000.00,No,0.00,
ICICIBank,ASSET,950.00,Yes,0.00,
Food,EXPENSE,50.00,No,500.00,
```

**tran_2025.csv** (auto-creates tran_2026.csv etc)
```csv
TranDate,TranTime,From,To,Description,Amount
29-10-2025,17:00,ICICIBank,Food,Dinner,50.00
28-10-2025,13:00,Salary,ICICIBank,SalaryCredit,1000.00
```

**record.csv** (auto-updated daily)
```csv
Date,NetWorth,Assets,Liabilities,Expenses
28-10-2025,950.00,950.00,0.00,0.00
29-10-2025,900.00,900.00,0.00,50.00
```

## Technical Stack

**Backend:** Go 1.22+  
**Frontend:** Vanilla JavaScript  
**Charts:** Chart.js 4.4.0  
**Design:** Material Design principles  
**Data Storage:** CSV files (no database required)

## Automatic Features

1. **Transaction Processing**
   - Auto-update account balances
   - Auto-sort by date and time
   - Auto-create new year CSV files
   - Auto-adjust backdated entries

2. **Daily Batch** (runs at midnight)
   - Update record.csv with daily snapshot
   - Recalculate all balances
   - Fix data inconsistencies
   - Log all operations

## Security

- SHA-256 password hashing
- Session-based authentication with CSRF protection
- Rate limiting on login attempts
- Read-only mode for safe sharing
- Input validation and sanitization
- Security headers (XSS, CSRF, Clickjacking protection)

## Command Line Options

```bash
# Set custom password
./arthik -p "YourSecurePassword"

# Run in read-only mode
./arthik -r

# Read-only mode with password display
./arthik -r -p "DemoPassword123"
```

## Environment Variables

```bash
# Set password hash (production)
export ARTHIK_PASSWORD_HASH="your_sha256_hash"
./arthik
```

## API Endpoints

```
POST   /api/login           - Authenticate user
POST   /api/logout          - Logout user
GET    /api/dashboard       - Get dashboard data
GET    /api/transactions    - List transactions (paginated)
POST   /api/transactions    - Create transaction
PUT    /api/transactions    - Update transaction
DELETE /api/transactions    - Delete transaction
GET    /api/accounts        - List accounts
POST   /api/accounts        - Create account
PUT    /api/accounts        - Update account
DELETE /api/accounts        - Delete account
POST   /api/settings        - Update password
GET    /api/readonly-info   - Get readonly mode status
GET    /health              - Health check
```

## Color Coding

**Account Types:**
- Assets: Green
- Liabilities: Red
- Income: Blue
- Expenses: Orange

**Bill Urgency:**
- <3 days: Red (high urgency)
- <7 days: Yellow (medium urgency)
- >7 days: Normal

## Responsive Design

- Desktop: Full multi-column layout
- Tablet: Adaptive 2-column grid
- Mobile: Single column with optimized navigation
- Touch-friendly buttons and forms

## Browser Support

- Chrome/Edge (recommended)
- Firefox
- Safari
- Mobile browsers

## Requirements

- Go 1.22 or higher
- Modern web browser
- Port 8080 available

## To Be Added

### 1. AI-Featured Auto-Completion for Transactions
- Intelligent transaction suggestions based on historical data
- Auto-complete account names and descriptions
- Smart amount predictions
- Learning from user patterns
- Category recommendations

### 2. Search Feature
- Full-text search across all transactions
- Advanced filters (date range, account, amount range)
- Search by description, account names, or amounts
- Quick search shortcuts
- Search result highlighting

### 3. Modularity
- Plugin system for custom features
- Modular account types
- Custom report generators
- Theme extensions
- Export/import modules
- API extensions for third-party integrations

## Contributing

This is an open-source project under MIT License. Contributions are welcome!

## Notes

- Date format: DD-MM-YYYY
- Time format: HH:MM (24-hour)
- All amounts: 2 decimal places
- CSV files can be manually edited
- Server runs on port 8080

---

**arthik v0.9** - Personal Finance Dashboard  
**License:** MIT | **Website:** arthik.nihars.com | **Demo:** demoarthik.nihars.com