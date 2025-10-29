# Arthik System Architecture

## Overview

Arthik is a single-page application (SPA) with a Go backend server, using CSV files for data persistence.

```
┌─────────────────────────────────────────────────────────────┐
│                         USER BROWSER                         │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │              Frontend (SPA)                         │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐        │    │
│  │  │ HTML/CSS │  │JavaScript│  │ Chart.js │        │    │
│  │  │ Material │  │  ES6+    │  │Visualiz. │        │    │
│  │  │ Design   │  │          │  │          │        │    │
│  │  └──────────┘  └──────────┘  └──────────┘        │    │
│  │                                                     │    │
│  │  Components:                                       │    │
│  │  • Login Screen                                    │    │
│  │  • Dashboard (Charts, Widgets)                     │    │
│  │  • Ledger (Transaction Management)                 │    │
│  │  • Account (Account Management)                    │    │
│  │  • Settings (Preferences)                          │    │
│  └────────────────────────────────────────────────────┘    │
│                            │                                 │
│                            │ HTTP/REST API                   │
│                            ▼                                 │
└────────────────────────────┼─────────────────────────────────┘
                             │
                             │
┌────────────────────────────┼─────────────────────────────────┐
│                            │    Go Server (Port 8080)        │
│                            ▼                                 │
│  ┌────────────────────────────────────────────────────┐    │
│  │              API Router                             │    │
│  │  ┌──────────────────────────────────────────┐     │    │
│  │  │ /api/accounts    (GET/POST/PUT/DELETE)   │     │    │
│  │  │ /api/transactions (GET/POST/PUT/DELETE)  │     │    │
│  │  │ /api/records     (GET)                   │     │    │
│  │  │ /                (Serve Frontend)        │     │    │
│  │  └──────────────────────────────────────────┘     │    │
│  └────────────────────────────────────────────────────┘    │
│                            │                                 │
│                            │                                 │
│                            ▼                                 │
│  ┌────────────────────────────────────────────────────┐    │
│  │           Business Logic Layer                      │    │
│  │  • Account Management                               │    │
│  │  • Transaction Processing                           │    │
│  │  • Balance Calculation                              │    │
│  │  • Record Generation                                │    │
│  │  • Sorting & Pagination                             │    │
│  │  • Data Validation                                  │    │
│  └────────────────────────────────────────────────────┘    │
│                            │                                 │
│                            │                                 │
│                            ▼                                 │
│  ┌────────────────────────────────────────────────────┐    │
│  │           Daily Batch Job Scheduler                 │    │
│  │  • 24-Hour Ticker                                   │    │
│  │  • End-of-Day Calculations                          │    │
│  │  • Record Updates                                   │    │
│  │  • Data Validation                                  │    │
│  │  • Log Generation                                   │    │
│  └────────────────────────────────────────────────────┘    │
│                            │                                 │
│                            │                                 │
└────────────────────────────┼─────────────────────────────────┘
                             │
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                     File System Storage                      │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   data/      │  │   logs/      │  │  frontend/   │     │
│  │              │  │              │  │              │     │
│  │ • account.csv│  │ • batch.log  │  │ • index.html │     │
│  │ • record.csv │  │              │  │ • app.js     │     │
│  │ • tran_YYYY  │  │              │  │ • style.css  │     │
│  │   .csv       │  │              │  │              │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

## Component Details

### Frontend Layer

#### HTML Structure (index.html)
```
┌─ Login Screen
│  └─ Password Form
│
├─ Main App
│  ├─ Header ("arthik")
│  │
│  ├─ Navigation Tabs
│  │  ├─ Dashboard
│  │  ├─ Ledger
│  │  ├─ Account
│  │  └─ Settings
│  │
│  └─ Tab Content Areas
│     ├─ Dashboard Content
│     │  ├─ Net Worth Card + Chart
│     │  ├─ Budget Progress
│     │  ├─ Account Pills
│     │  ├─ Portfolio Chart
│     │  └─ Bills Table
│     │
│     ├─ Ledger Content
│     │  ├─ Transaction Form
│     │  ├─ Transaction List
│     │  ├─ Pagination
│     │  └─ FAB (+) Button
│     │
│     ├─ Account Content
│     │  ├─ Account Form
│     │  ├─ Accounts Grid
│     │  └─ FAB (+) Button
│     │
│     └─ Settings Content
│        ├─ Dark Mode Toggle
│        ├─ Hide Amounts Toggle
│        ├─ Password Change
│        └─ Logout Button
```

#### JavaScript Architecture (app.js)
```
Global State
├─ isAuthenticated
├─ password
├─ darkMode
├─ hideAmounts
├─ accounts []
├─ transactions []
├─ records []
└─ pagination state

Event Listeners
├─ Authentication
├─ Tab Navigation
├─ Transaction CRUD
├─ Account CRUD
└─ Settings Management

API Communication
├─ loadData()
├─ fetch() calls
└─ Error handling

UI Updates
├─ Dashboard rendering
├─ Chart updates
├─ List rendering
└─ Form management

Helper Functions
├─ formatNumber()
├─ formatDate()
├─ sortTransactions()
└─ calculateBalances()
```

#### CSS Structure (style.css)
```
Base Styles
├─ Reset & Normalize
├─ CSS Custom Properties (Theme Variables)
└─ Typography

Components
├─ Login Screen
├─ Header & Navigation
├─ Cards & Containers
├─ Forms & Inputs
├─ Buttons & FABs
├─ Tables & Lists
├─ Charts & Legends
└─ Settings Controls

Utilities
├─ Responsive Breakpoints
├─ Animations
├─ Dark Mode Overrides
└─ Scrollbar Styling
```

### Backend Layer

#### Go Server Structure (main.go)
```
main()
├─ initDirectories()
├─ loadData()
├─ initializeSampleData()
├─ scheduleDailyBatch()
└─ setupRoutes() + ListenAndServe()

Data Structures
├─ Account struct
├─ Transaction struct
└─ Record struct

HTTP Handlers
├─ handleAccounts() - CRUD operations
├─ handleTransactions() - CRUD operations
├─ handleRecords() - Read operations
└─ serveFile() - Static file serving

Data Operations
├─ loadAccounts()
├─ loadTransactions()
├─ loadRecords()
├─ saveAccounts()
├─ saveTransactions()
└─ saveRecords()

Business Logic
├─ addTransaction()
├─ updateTransaction()
├─ deleteTransaction()
├─ updateAccountBalances()
├─ recalculateAll()
├─ calculateDailyRecord()
└─ sortTransactions()

Batch Processing
├─ scheduleDailyBatch()
├─ runDailyBatch()
└─ logBatch()
```

### Data Layer

#### CSV File Relationships
```
account.csv
├─ Stores: All account definitions
├─ Updated: On account CRUD, transaction processing
└─ Read by: API, Dashboard, Forms

tran_YYYY.csv (Year-based)
├─ Stores: Transactions for specific year
├─ Updated: On transaction CRUD
├─ Read by: API, Ledger, Calculations
└─ Multiple files: tran_2024.csv, tran_2025.csv, etc.

record.csv
├─ Stores: Daily financial snapshots
├─ Updated: Daily batch job, backdated transactions
├─ Read by: Dashboard charts
└─ Chronologically ordered

logs/batch.log
├─ Stores: System operation logs
├─ Updated: Daily batch job
└─ Used for: Debugging, audit trail
```

## Data Flow Diagrams

### Transaction Creation Flow
```
User Input
    │
    ▼
Frontend Form
    │
    ▼
Validation
    │
    ▼
POST /api/transactions
    │
    ▼
Go Handler
    │
    ├──▶ Parse JSON
    │
    ├──▶ Add to tran_YYYY.csv
    │
    ├──▶ Sort transactions
    │
    ├──▶ Update account balances
    │
    ├──▶ Update records.csv
    │
    └──▶ Save to disk
         │
         ▼
    Return Success
         │
         ▼
    Frontend Reload
         │
         ▼
    Update UI (Dashboard, Ledger)
```

### Dashboard Refresh Flow
```
User Switches to Dashboard Tab
    │
    ▼
updateDashboard()
    │
    ├──▶ updateNetworth()
    │    ├─ Calculate from records
    │    └─ Render chart
    │
    ├──▶ updateBudgetProgress()
    │    ├─ Sum expense budgets
    │    ├─ Sum actual expenses
    │    └─ Calculate percentage
    │
    ├──▶ updateAccountsPills()
    │    └─ Render asset/liability accounts
    │
    ├──▶ updatePortfolioChart()
    │    ├─ Filter asset accounts
    │    ├─ Calculate percentages
    │    └─ Render pie chart
    │
    └──▶ updateUpcomingBills()
         ├─ Filter liabilities by due date
         ├─ Sort by urgency
         └─ Render table with color coding
```

### Daily Batch Job Flow
```
24-Hour Ticker Fires
    │
    ▼
runDailyBatch()
    │
    ├──▶ Get today's date
    │
    ├──▶ calculateDailyRecord()
    │    │
    │    ├─ Sum all assets (IINW=Yes)
    │    │
    │    ├─ Sum all liabilities
    │    │
    │    ├─ Sum all expenses
    │    │
    │    ├─ Calculate net worth
    │    │
    │    └─ Create/Update record entry
    │
    ├──▶ saveRecords()
    │
    └──▶ logBatch()
         │
         └─ Write to logs/batch.log
```

## Security Architecture

### Authentication Flow
```
User Access
    │
    ▼
Login Screen
    │
    ├──▶ Enter Password
    │
    ├──▶ Validate against stored password
    │
    ├──▶ If Valid:
    │    ├─ Set localStorage('arthik_auth', 'true')
    │    ├─ Update state.isAuthenticated
    │    └─ Show Main App
    │
    └──▶ If Invalid:
         └─ Show error message

On Subsequent Visits
    │
    ▼
checkAuthentication()
    │
    ├──▶ Check localStorage('arthik_auth')
    │
    ├──▶ If 'true':
    │    └─ Auto-login (skip login screen)
    │
    └──▶ If not:
         └─ Show login screen

On Logout
    │
    ▼
logout()
    │
    ├──▶ Clear localStorage('arthik_auth')
    │
    ├──▶ Set state.isAuthenticated = false
    │
    └──▶ Show login screen
```

### API Security
```
Browser Request
    │
    ▼
CORS Middleware
    │
    ├──▶ Set Access-Control-Allow-Origin: *
    │
    ├──▶ Set Access-Control-Allow-Methods
    │
    └──▶ Set Access-Control-Allow-Headers
         │
         ▼
    Route Handler
         │
         ├──▶ Process Request
         │
         ├──▶ Validate Data
         │
         └──▶ Return Response
```

## Performance Optimizations

### Frontend Optimizations
- Pagination (30 items per page) - reduces DOM nodes
- CSS transforms for animations (GPU accelerated)
- Debounced form inputs (future enhancement)
- Chart.js canvas rendering (efficient for large datasets)
- localStorage caching of settings
- Lazy chart initialization (only when tab visible)

### Backend Optimizations
- CSV parsing with buffered readers
- In-memory caching of current data
- Efficient sorting algorithms
- Minimal file I/O operations
- Goroutine for batch job (non-blocking)

## Scalability Considerations

### Current Limits
- Handles 10,000+ transactions efficiently
- Chart performance degrades after 1000 data points
- CSV file size manageable up to ~10MB
- Browser localStorage limit: 5-10MB

### Scaling Strategies
- Pagination keeps UI responsive
- Year-based transaction files prevent single file bloat
- Selective data loading (only current + previous year)
- Chart data point limiting (show last N records)

## Deployment Architecture

### Development
```
Developer Machine
├─ Clone/Download project
├─ Run start.sh or start.bat
├─ Go server starts on localhost:8080
└─ Access via browser
```

### Production (Simple)
```
Server/VPS
├─ Install Go runtime
├─ Upload project files
├─ Configure firewall (allow port 8080)
├─ Run: go build main.go
├─ Execute: ./main (or use systemd service)
└─ Access via server IP/domain
```

### Production (Advanced)
```
                    ┌─────────┐
                    │  Nginx  │ (Reverse Proxy)
                    │  :80/:443│
                    └────┬────┘
                         │ SSL/TLS
                         │
                    ┌────▼────┐
                    │  Go App │
                    │  :8080  │
                    └────┬────┘
                         │
                    ┌────▼────┐
                    │  Files  │
                    │  System │
                    └─────────┘
```

## Technology Stack Summary

```
┌───────────────────────────────────────┐
│          Frontend Stack               │
├───────────────────────────────────────┤
│ • HTML5 (Semantic markup)             │
│ • CSS3 (Custom properties, Flexbox,   │
│   Grid, Animations)                   │
│ • JavaScript ES6+ (Async/await,       │
│   Modules, Arrow functions)           │
│ • Chart.js 4.4.0 (Visualization)      │
│ • Material Design (UI framework)      │
│ • Google Fonts (Roboto)               │
│ • Material Icons                      │
└───────────────────────────────────────┘

┌───────────────────────────────────────┐
│          Backend Stack                │
├───────────────────────────────────────┤
│ • Go 1.16+ (Programming language)     │
│ • Standard library (net/http,         │
│   encoding/csv, encoding/json)        │
│ • No external dependencies            │
└───────────────────────────────────────┘

┌───────────────────────────────────────┐
│          Data & Storage               │
├───────────────────────────────────────┤
│ • CSV files (Data storage)            │
│ • File system (Persistence)           │
│ • localStorage (Settings/Session)     │
└───────────────────────────────────────┘
```

---

## Integration Points

### Browser ↔ Go Server
- **Protocol**: HTTP/REST
- **Format**: JSON
- **Methods**: GET, POST, PUT, DELETE
- **CORS**: Enabled for cross-origin

### Go Server ↔ File System
- **Operations**: Read, Write, Create
- **Format**: CSV (RFC 4180)
- **Encoding**: UTF-8
- **Atomicity**: File replacement on write

### Frontend ↔ Charts
- **Library**: Chart.js
- **Format**: JavaScript objects
- **Update**: Destroy and recreate
- **Theme**: Dynamic color injection

---

*This architecture provides a solid foundation for a personal finance application while maintaining simplicity and ease of maintenance.*
