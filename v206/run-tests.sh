#!/bin/bash

# JSONata Go Implementation Test Runner
# ====================================
# 
# This script runs all tests in order from simplest/lowest-level to most complex.
# It's designed for debugging transliteration issues by testing components in 
# dependency order, with the comprehensive test suite at the end.
#
# Test Order Rationale:
# 1. Foundation Layer: Basic utilities and type checking
# 2. Core Engine Layer: Parser, built-in functions 
# 3. Parser Features Layer: Advanced parser capabilities
# 4. Integration Layer: Full expression evaluation
# 5. Full Test Suite: Comprehensive JavaScript compatibility testing

set -e  # Exit on first failure when using --failfast

echo "🧪 JSONata Go Implementation Test Suite"
echo "========================================"
echo ""

# Foundation Layer - Basic utilities and infrastructure
echo "📋 FOUNDATION LAYER - Basic Utilities"
echo "------------------------------------"

echo "🔧 Testing utility functions (isNumeric, array helpers, etc.)"
go test -v --failfast -run "TestIsNumeric|TestIsArrayOfStrings|TestIsArrayOfNumbers|TestStringToArray|TestIsDeepEqual"

echo ""
echo "🏷️  Testing function signature parsing and type checking"
go test -v --failfast -run "TestParseSignature|TestGetSymbol"

echo ""
echo "✅ Foundation layer complete"
echo ""

# Core Engine Layer - Parser and basic functionality  
echo "⚙️  CORE ENGINE LAYER - Parser and Built-in Functions"
echo "---------------------------------------------------"

echo "📝 Testing tokenizer and parser fundamentals"
go test -v --failfast -run "TestTokenizer|TestOperatorPrecedence"

echo ""
echo "🔤 Testing parser AST generation and error codes"
go test -v --failfast -run "TestParser|TestParserErrorCodes"

echo ""
echo "📅 Testing date/time functions (formatInteger, toMillis, fromMillis)"
go test -v --failfast -run "TestFormatInteger|TestToMillis|TestFromMillis"

echo ""
echo "🔢 Testing individual built-in functions"
go test -v --failfast -run "TestSumFunction|TestCountFunction|TestMaxFunction|TestStringFunction|TestJoinFunction|TestBooleanFunction|TestNumberFunction"

echo ""
echo "✅ Core engine layer complete"
echo ""

# Parser Features Layer - Advanced parser capabilities
echo "🚀 PARSER FEATURES LAYER - Advanced Parser Capabilities"
echo "------------------------------------------------------"

echo "🛠️  Testing parser error recovery mode"
go test -v --failfast -run "TestParserRecovery|TestBasicParserErrors"

echo ""
echo "🔍 Testing pluggable regex engine support"
go test -v --failfast -run "TestParserPluggableRegex"

echo ""
echo "✅ Parser features layer complete"
echo ""

# Integration Layer - Full expression evaluation
echo "🔗 INTEGRATION LAYER - Full Expression Evaluation"
echo "------------------------------------------------"

echo "⚡ Testing implementation-specific features and Go user-defined functions"
go test -v --failfast -run "TestImplementationSpecific"

echo ""
echo "🌐 Testing async function infrastructure (basic functionality only)"
go test -v --failfast -run "TestAsyncFunction"

echo ""
echo "📜 Testing JavaScript-specific regex functionality"  
go test -v --failfast -run "TestJavaScriptSpecificRegex"

echo ""
echo "✅ Integration layer complete"
echo ""

# Full Test Suite - Comprehensive testing with JavaScript comparison
echo "🏆 FULL TEST SUITE - Comprehensive JavaScript Compatibility"
echo "===========================================================" 

echo "🔬 Running complete JSONata test suite with dual JavaScript/Go execution"
echo "⚠️  This test compares Go implementation against JavaScript reference"
echo "💡 Look for 'RESULT MISMATCH' and 'IMPLEMENTATION MISMATCH' in output"
go test -v --failfast -run "TestJSONataTestSuite"

echo ""
echo "🎉 All tests complete!"
echo ""
echo "📊 Test Summary:"
echo "  ✅ Foundation Layer: Utilities and type checking"
echo "  ✅ Core Engine Layer: Parser and built-in functions"  
echo "  ✅ Parser Features Layer: Advanced parser capabilities"
echo "  ✅ Integration Layer: Full expression evaluation"
echo "  ✅ Full Test Suite: Comprehensive compatibility testing"
echo ""
echo "🐛 If tests failed, check the output above for specific failures."
echo "💡 For transliteration issues, focus on the simpler layers first."
echo "🔍 For functionality issues, look at the 'MISMATCH' messages in the test suite."
