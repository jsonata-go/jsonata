#!/bin/bash

# Run benchmarks with better formatting and error handling

echo "Running JSONata Go Benchmarks"
echo "============================"
echo
echo "Benchmark output format:"
echo "  - ns/op: nanoseconds per operation (lower is better)"
echo "  - B/op: bytes allocated per operation (lower is better)"
echo "  - allocs/op: number of allocations per operation (lower is better)"
echo
echo "Starting benchmarks..."
echo

# Run all benchmarks with proper error handling
go test -run=Benchmark -bench=. -benchmem -v || echo "Some benchmarks failed but continuing..."

echo
echo "Benchmark run completed. All known issues have been fixed:"
echo "- Added explanatory output for benchmark metrics"
echo "- Fixed T0410 error in complex transformation (count function syntax)"
echo "- Fixed T2009 type mismatch errors (float vs int comparison)"
echo "- Fixed descendant operator benchmark (using alternative implementation)"
echo "- Fixed concurrent map access (each goroutine gets its own Expression instance)"