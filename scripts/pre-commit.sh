#!/bin/bash

# Pre-commit hook for Go-Tuya Library
# This script runs all necessary checks before allowing a commit

set -e

echo "🔍 Running pre-commit checks..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_error "Not in a git repository"
    exit 1
fi

# Get list of staged Go files
STAGED_GO_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)

if [ -z "$STAGED_GO_FILES" ]; then
    print_status "No Go files staged for commit"
    exit 0
fi

echo "📁 Staged Go files:"
echo "$STAGED_GO_FILES"

# 1. Check if code is formatted
echo ""
echo "🔧 Checking code formatting..."
if ! make fmt-check > /dev/null 2>&1; then
    print_error "Code is not properly formatted"
    print_warning "Run 'make fmt' to fix formatting issues"
    exit 1
fi
print_status "Code formatting is correct"

# 2. Check if imports are organized
echo ""
echo "📦 Checking import organization..."
if ! make imports-check > /dev/null 2>&1; then
    print_error "Imports are not properly organized"
    print_warning "Run 'make imports' to fix import issues"
    exit 1
fi
print_status "Imports are properly organized"

# 3. Run go vet
echo ""
echo "🔍 Running go vet..."
if ! make vet > /dev/null 2>&1; then
    print_error "go vet found issues"
    exit 1
fi
print_status "go vet passed"

# 4. Run golangci-lint
echo ""
echo "🧹 Running golangci-lint..."
if ! make lint > /dev/null 2>&1; then
    print_error "golangci-lint found issues"
    print_warning "Run 'make lint-fix' to auto-fix some issues"
    exit 1
fi
print_status "golangci-lint passed"

# 5. Run tests
echo ""
echo "🧪 Running tests..."
if ! make test > /dev/null 2>&1; then
    print_error "Tests failed"
    exit 1
fi
print_status "All tests passed"

# 6. Check for race conditions (optional, can be slow)
if [ "$1" = "--race" ]; then
    echo ""
    echo "🏃 Running race condition tests..."
    if ! make test-race > /dev/null 2>&1; then
        print_error "Race condition tests failed"
        exit 1
    fi
    print_status "Race condition tests passed"
fi

echo ""
print_status "All pre-commit checks passed! 🚀"
echo "Ready to commit your changes." 