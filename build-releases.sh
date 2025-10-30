#!/bin/bash
# build-releases.sh - Build arthik binaries for all platforms

set -e

VERSION="0.9"
BINARY_NAME="arthik"

echo "Building arthik v${VERSION} for multiple platforms..."

# Create releases directory
mkdir -p releases

# Build for different platforms
echo "Building Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ${BINARY_NAME}-linux-amd64 main.go

echo "Building Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ${BINARY_NAME}-linux-arm64 main.go

echo "Building macOS Intel..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ${BINARY_NAME}-darwin-amd64 main.go

echo "Building macOS Apple Silicon..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ${BINARY_NAME}-darwin-arm64 main.go

echo "Building Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ${BINARY_NAME}-windows-amd64.exe main.go

echo "Creating release archives..."

# Linux AMD64
tar -czf releases/${BINARY_NAME}-v${VERSION}-linux-amd64.tar.gz \
    ${BINARY_NAME}-linux-amd64 frontend/ README.md
rm ${BINARY_NAME}-linux-amd64

# Linux ARM64
tar -czf releases/${BINARY_NAME}-v${VERSION}-linux-arm64.tar.gz \
    ${BINARY_NAME}-linux-arm64 frontend/ README.md
rm ${BINARY_NAME}-linux-arm64

# macOS Intel
tar -czf releases/${BINARY_NAME}-v${VERSION}-darwin-amd64.tar.gz \
    ${BINARY_NAME}-darwin-amd64 frontend/ README.md
rm ${BINARY_NAME}-darwin-amd64

# macOS Apple Silicon
tar -czf releases/${BINARY_NAME}-v${VERSION}-darwin-arm64.tar.gz \
    ${BINARY_NAME}-darwin-arm64 frontend/ README.md
rm ${BINARY_NAME}-darwin-arm64

# Windows
zip -r releases/${BINARY_NAME}-v${VERSION}-windows-amd64.zip \
    ${BINARY_NAME}-windows-amd64.exe frontend/ README.md
rm ${BINARY_NAME}-windows-amd64.exe

# Generate checksums
echo "Generating checksums..."
cd releases
sha256sum *.tar.gz *.zip > checksums.txt
cd ..

echo "✅ Build complete! Releases are in ./releases/"
echo ""
echo "Files created:"
ls -lh releases/
echo ""
echo "Upload these files to your GitLab release!"
