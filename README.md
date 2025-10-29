# ARTHIK - Personal Finance Dashboard

A complete Material Design personal finance management system with Go backend and vanilla JavaScript frontend.

## Quick Start

```bash
cd arthik
go build -o arthik main.go
./arthik
```

Open browser: http://localhost:8080  
Default password: **admin123**

## Project Structure

```
arthik/
├── main.go              # Go backend server with all APIs
├── go.mod               # Go module file
├── frontend/
│   ├── index.html       # Material Design UI
│   ├── app.js          # All frontend logic
│   └── style.css       # Responsive Material Design CSS
├── data/
│   ├── account.csv     # Account master data
│   ├── tran_2025.csv   # Current year transactions
│   └── record.csv      # Historical daily records
└── logs/               # Server and batch logs
```

## Features

### Dashboard Tab
✓ Large net worth display  
✓ Multi-line chart (net worth, assets, liabilities, expenses)  
✓ Budget vs expenses progress bar with percentage  
✓ All accounts displayed as colored pills  
✓ Investment portfolio pie chart with percentages  
✓ Upcoming bills table (30 days, color-coded by urgency)

### Ledger Tab
✓ Add new transactions via floating action button  
✓ Edit existing transactions inline  
✓ Delete transactions with confirmation  
✓ Auto-sort latest to oldest  
✓ Pagination (30 per page)  
✓ Automatic account balance updates  
✓ Backdated transaction support  
✓ Cross-year transaction management

### Account Tab
✓ Add/edit/delete accounts  
✓ Four account types: ASSET, LIABILITIES, INCOME, EXPENSE  
✓ Net worth inclusion toggle  
✓ Budget field for expense accounts  
✓ Due date field for liabilities  
✓ Automatic balance calculation

### Settings Tab
✓ Dark/light mode toggle (persisted)  
✓ Hide/show amounts toggle (persisted)  
✓ Change password functionality

## Technical Details

### Backend (Go)
- RESTful API with JSON responses
- SHA-256 password hashing
- Automatic daily batch processing
- CSV file management with proper sorting
- Concurrent-safe operations
- Comprehensive error logging

### Frontend (Vanilla JS + Chart.js)
- No framework dependencies (except Chart.js)
- Material Design components
- Responsive layout (mobile-first)
- Local storage for preferences
- Real-time data updates

### Data Files

**account.csv**
```
Account,Type,Amount,IINW,Budget,DueDate
Salary,INCOME,-1000.00,No,0.00,
ICICIBank,ASSET,950.00,Yes,0.00,
Food,EXPENSE,50.00,No,500.00,
```

**tran_2025.csv** (auto-creates tran_2026.csv etc)
```
TranDate,TranTime,From,To,Description,Amount
29-10-2025,17:00,ICICIBank,Food,Dinner,50.00
28-10-2025,13:00,Salary,ICICIBank,SalaryCredit,1000.00
```

**record.csv** (auto-updated daily)
```
Date,NetWorth,Assets,Liabilities,Expenses
28-10-2025,950.00,950.00,0.00,0.00
29-10-2025,900.00,900.00,0.00,50.00
```

## API Endpoints

```
POST   /api/login           - Authenticate user
GET    /api/dashboard       - Get dashboard data
GET    /api/transactions    - List transactions (paginated)
POST   /api/transactions    - Create transaction
PUT    /api/transactions    - Update transaction
DELETE /api/transactions    - Delete transaction
GET    /api/accounts        - List accounts
POST   /api/accounts        - Create account
PUT    /api/accounts        - Update account
DELETE /api/accounts        - Delete account
POST   /api/settings        - Update settings
```

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

3. **Data Integrity**
   - Validate all CSV operations
   - Maintain transaction ordering
   - Ensure balance accuracy
   - Cross-reference accounts

## Security

- Password: SHA-256 hashed (default: admin123)
- Token-based session management
- Input validation on all fields
- Safe CSV file operations
- No SQL injection risks (file-based)

## Responsive Design

✓ Desktop: Full multi-column layout  
✓ Tablet: Adaptive 2-column grid  
✓ Mobile: Single column, optimized navigation  
✓ All buttons and forms touch-friendly

## Color Coding

**Account Types:**
- Assets: Green
- Liabilities: Red
- Income: Blue
- Expenses: Orange

**Bill Urgency:**
- <3 days: Red background
- <7 days: Yellow background
- >7 days: Normal

## Notes

- CSV files are NOT encrypted (manual editing allowed)
- Batch process validates and fixes manual edits
- All amounts: 2 decimal places
- Date format: DD-MM-YYYY
- Time format: HH:MM (24-hour)
- Server runs on port 8080

## Browser Support

✓ Chrome/Edge (recommended)  
✓ Firefox  
✓ Safari  
✓ Mobile browsers

## Requirements

- Go 1.22+
- Modern web browser
- Port 8080 available

---

**Created with Material Design principles**  
**Backend: Go | Frontend: Vanilla JS | Charts: Chart.js**
