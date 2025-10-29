#!/bin/bash

# Arthik - Secure Personal Finance Application
# Start Script

echo "======================================"
echo "  Arthik - Personal Finance Dashboard"
echo "======================================"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed!"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

# Set default password hash if not set (admin123 - CHANGE IN PRODUCTION!)
if [ -z "$ARTHIK_PASSWORD_HASH" ]; then
    export ARTHIK_PASSWORD_HASH="240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9"
    echo "⚠️  WARNING: Using default password 'admin123'"
    echo "⚠️  To change: export ARTHIK_PASSWORD_HASH=\$(echo -n 'YourPassword' | sha256sum | cut -d' ' -f1)"
    echo ""
fi

# Create directory structure
mkdir -p frontend data logs

# Copy frontend files
echo "Setting up frontend..."
cp index.html frontend/
cp app.js frontend/
cp style2.css frontend/ 2>/dev/null || echo "Note: style2.css not found, make sure to add it to frontend/"

# Build and run
echo "Building application..."
go build -o arthik main.go

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Build successful!"
    echo ""
    echo "Starting server..."
    echo "Server will be available at: http://localhost:8080"
    echo "Default password: admin123"
    echo ""
    echo "Press Ctrl+C to stop the server"
    echo "======================================"
    echo ""
    ./arthik
else
    echo "❌ Build failed! Please check for errors above."
    exit 1
fi