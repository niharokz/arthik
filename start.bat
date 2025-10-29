@echo off
REM Arthik - Personal Finance Dashboard Startup Script (Windows)

echo ==========================================
echo   Arthik - Personal Finance Dashboard
echo ==========================================
echo.

REM Check if Go is installed
where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo X Go is not installed. Please install Go 1.16 or higher.
    echo   Visit: https://golang.org/dl/
    pause
    exit /b 1
)

for /f "tokens=*" %%i in ('go version') do set GO_VERSION=%%i
echo + Go found: %GO_VERSION%
echo.

REM Create directories if they don't exist
echo Creating required directories...
if not exist "data" mkdir data
if not exist "logs" mkdir logs
echo + Directories created
echo.

REM Initialize Go module if needed
if not exist "go.mod" (
    echo Initializing Go module...
    go mod init arthik
    echo + Go module initialized
    echo.
)

REM Build and run
echo Starting Arthik server...
echo.
echo ==========================================
echo   Server will start on:
echo   http://localhost:8080
echo.
echo   Default password: admin123
echo.
echo   Press Ctrl+C to stop the server
echo ==========================================
echo.

go run main.go
