#!/bin/bash

# Setup script for pre-commit hooks
# This script installs the pre-commit hook and makes it executable

set -e

echo "🔧 Setting up pre-commit hooks..."

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "❌ Not in a git repository. Please run this script from the root of your git repository."
    exit 1
fi

# Create scripts directory if it doesn't exist
if [ ! -d "scripts" ]; then
    echo "❌ Scripts directory not found. Please ensure this script is run from the project root."
    exit 1
fi

# Make the pre-commit script executable
if [ -f "scripts/pre-commit.sh" ]; then
    chmod +x scripts/pre-commit.sh
    print_status "Made pre-commit.sh executable"
else
    echo "❌ pre-commit.sh not found in scripts directory"
    exit 1
fi

# Create .git/hooks directory if it doesn't exist
mkdir -p .git/hooks

# Copy the pre-commit hook
cp scripts/pre-commit.sh .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit

print_status "Pre-commit hook installed successfully!"

# Check if required tools are installed
echo ""
echo "🔍 Checking for required tools..."

# Check for golangci-lint
if ! command -v golangci-lint &> /dev/null; then
    print_warning "golangci-lint not found. Run 'make install-tools' to install it."
else
    print_status "golangci-lint is installed"
fi

# Check for goimports
if ! command -v goimports &> /dev/null; then
    print_warning "goimports not found. Run 'make install-tools' to install it."
else
    print_status "goimports is installed"
fi

echo ""
print_status "Setup complete! 🚀"
echo ""
echo "The pre-commit hook will now run automatically on every commit."
echo "To run pre-commit checks manually, use:"
echo "  make pre-commit"
echo "  or"
echo "  .git/hooks/pre-commit"
echo ""
echo "To install development tools, run:"
echo "  make install-tools" 