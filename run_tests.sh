#!/bin/bash

# Test Suite Quick Reference Guide
# Run this script from the project root directory

echo "🧪 Release Tracker Test Suite"
echo "=============================="
echo ""

# Function to run tests with description
run_test() {
    echo "📋 $1"
    echo "Command: $2"
    echo ""
    eval "$2"
    echo ""
    echo "---"
    echo ""
}

# Menu
case "${1:-all}" in
    all)
        echo "Running all tests..."
        go test -v
        ;;
    
    coverage)
        echo "Running tests with coverage analysis..."
        go test -cover
        ;;
    
    detailed)
        echo "Running tests with detailed output..."
        go test -v 2>&1
        ;;
    
    race)
        echo "Running tests with race condition detection..."
        go test -race -v
        ;;
    
    bench)
        echo "Running benchmarks with memory stats..."
        go test -bench=. -benchmem
        ;;
    
    store)
        echo "Running store operation tests..."
        go test -run TestStore -v
        ;;
    
    handlers)
        echo "Running HTTP handler tests..."
        go test -run TestHandle -v
        ;;
    
    concurrency)
        echo "Running concurrency tests..."
        go test -run TestConcurrent -v
        ;;
    
    persistence)
        echo "Running state persistence tests..."
        go test -run "TestStateFile|TestLoadState" -v
        ;;
    
    help|--help|-h)
        echo "Usage: $0 [command]"
        echo ""
        echo "Available commands:"
        echo "  all          - Run all tests (default)"
        echo "  coverage     - Run tests with coverage report"
        echo "  detailed     - Run tests with detailed output"
        echo "  race         - Run tests with race condition detection"
        echo "  bench        - Run benchmarks with memory stats"
        echo "  store        - Run store operation tests only"
        echo "  handlers     - Run HTTP handler tests only"
        echo "  concurrency  - Run concurrency tests only"
        echo "  persistence  - Run persistence tests only"
        echo "  help         - Show this help message"
        echo ""
        echo "Examples:"
        echo "  ./run_tests.sh                 # Run all tests"
        echo "  ./run_tests.sh coverage        # Check coverage"
        echo "  ./run_tests.sh race            # Detect data races"
        echo "  ./run_tests.sh bench           # Run benchmarks"
        ;;
    
    *)
        echo "Unknown command: $1"
        echo "Run '$0 help' for usage information"
        exit 1
        ;;
esac
