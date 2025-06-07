# Differences Between JavaScript and Go JSONata Implementations

This document provides a comprehensive enumeration of the material design differences between the JavaScript reference implementation of JSONata and this Go port. While the Go implementation strives for semantic compatibility, certain platform differences are unavoidable.

## Table of Contents

1. [Type System Differences](#type-system-differences)
2. [String and Unicode Handling](#string-and-unicode-handling)
3. [Numeric Limitations](#numeric-limitations)
4. [Map and Object Ordering](#map-and-object-ordering)
5. [Concurrency and Async](#concurrency-and-async)
6. [Error Handling](#error-handling)
7. [Regular Expressions](#regular-expressions)
8. [Function Implementation](#function-implementation)
9. [Memory Management](#memory-management)
10. [Test-Specific Differences](#test-specific-differences)
11. [API Differences](#api-differences)

## Type System Differences

### JavaScript Dynamic Typing vs Go Static Typing

**JavaScript:**
- Single `Number` type for all numeric values
- Dynamic typing with implicit coercion
- `undefined` and `null` are distinct types
- Objects can have prototype chains

**Go Implementation:**
- Separate `int64` and `float64` types
- Extensive use of `interface{}` with type assertions
- Custom `JSONataNull` type to distinguish null from Go's `nil` (undefined)
- No prototype chain support

### Implications:
- Type checking is explicit in Go where JavaScript uses duck typing
- Custom type coercion logic implemented to match JavaScript behavior
- Performance overhead from interface{} boxing/unboxing

## String and Unicode Handling

### UTF-16 (JavaScript) vs UTF-8 (Go)

**JavaScript:**
- Strings are sequences of UTF-16 code units
- String length counts UTF-16 code units
- Supports unpaired surrogates
- String indexing operates on code units

**Go Implementation:**
- Strings are UTF-8 encoded byte sequences
- String length counts Unicode code points (runes)
- Invalid UTF-8 sequences replaced with U+FFFD
- String indexing operates on bytes or runes

### Test Impact:
```go
// From test_suite_test.go lines 286-308
// Tests containing Unicode surrogates are skipped:
"groups/string-functions/string-invalid-surrogates.json"
"groups/function-substring/substring-invalid-surrogates.json"
```

### Specific Differences:
1. `$length()` may return different values for strings with:
   - Surrogate pairs
   - Characters outside the Basic Multilingual Plane
2. `$substring()` behaves differently with surrogate pairs
3. Invalid Unicode handling differs between platforms

## Numeric Limitations

### JavaScript Number vs Go Numeric Types

**JavaScript:**
- All numbers are IEEE 754 double-precision floats
- Can represent integers up to 2^53-1 precisely
- Special values: `Infinity`, `-Infinity`, `NaN`

**Go Implementation:**
- `int64`: -9,223,372,036,854,775,808 to 9,223,372,036,854,775,807
- `float64`: IEEE 754 double-precision
- No native `Infinity` keyword (uses `math.Inf()`)

### Specific Limitations:

#### Large Number Word Formatting
```go
// From datetime.go formatInteger()
if number > 9999999999999999 { // Limit for word formatting
    return "", &JSONataError{
        Code: "D3100",
        Value: fmt.Sprintf("Number is too large for word formatting: %v", number),
    }
}
```

**Impact:** Numbers larger than ~9.2×10^18 cannot be formatted as words, while JavaScript can handle up to 10^46.

## Map and Object Ordering

### Iteration Order Guarantees

**JavaScript:**
- Objects maintain property insertion order (ES2015+)
- Predictable iteration order for object properties
- `Object.keys()` returns keys in defined order

**Go Implementation:**
- Map iteration order is explicitly randomized
- No guarantee of consistent ordering between runs
- `$keys()` function returns unpredictable order

### Test Impact:
Special handling in `test_suite_test.go`:
```go
} else if strings.Contains(expr, "$keys(") && strings.Contains(expr, "library.loans") {
    // Special case: library.loans tests with $keys() have map ordering issues
    equal = deepEqualWithUnorderedStringArrays(result, testcase.Result)
}
```

### Workarounds:
- Tests comparing key arrays use set comparison instead of array comparison
- Some tests skipped when order is semantically important
- Production code should not rely on key ordering

## Concurrency and Async

### Promise and Async Support

**JavaScript:**
- Full Promise/async/await support
- Non-blocking I/O operations
- Event loop and microtask queue
- Callback-based async patterns

**Go Implementation:**
- **No Promise support** - all operations are synchronous
- No async/await equivalents
- No event loop concept
- Goroutines not used for JSONata evaluation

### Test Impact:
All async-related tests are skipped:
```go
// From async_function_test.go
"HTTP requests and Promise-based async functionality not applicable to Go implementation"
"Promise-based error handling not applicable to Go implementation"
"JavaScript callback-based concurrency not applicable to Go implementation"
```

## Error Handling

### Exception Model Differences

**JavaScript:**
- Throws exceptions for errors
- Try/catch blocks for error handling
- Stack traces with JavaScript format
- Errors bubble up automatically

**Go Implementation:**
- Explicit error returns (Go idiom)
- No exception throwing
- Error codes preserved for compatibility
- Manual error propagation required

### Error Code Compatibility:
Both implementations use identical error codes:
- `D1xxx`: Data errors
- `S0xxx`: Static errors
- `T1xxx`: Type errors  
- `U0xxx`: User errors

## Regular Expressions

### Regex Engine Differences

**JavaScript:**
- PCRE-compatible regex engine
- Supports lookahead/lookbehind
- Unicode property escapes
- Named capture groups

**Go Implementation:**
- RE2 regex engine (linear time guarantee)
- No lookahead/lookbehind support
- Limited Unicode property support
- Different flag syntax
- No custom regex engine support (uses standard `regexp` package only)

### Specific Differences:
1. Regex literals `/pattern/flags` not supported in Go
2. Flag mapping required (e.g., JavaScript 'g' flag behavior differs)
3. Some regex patterns may need adjustment for RE2 compatibility
4. No ability to provide alternative regex implementations

## Function Implementation

### Built-in Function Differences

#### `$clone()` Function
- **JavaScript:** Deep clones objects including prototypes
- **Go:** Not implemented (specific to Node-RED environment)

#### Recursive Function Definitions
- **JavaScript:** Functions can reference themselves during definition
- **Go:** Limited support - requires variable binding first

### Test Impact:
```go
// From implementation_tests_test.go
t.Skip("$clone function is specific to Node-RED implementation")
t.Skip("Recursive function definition not supported in Go implementation")
```

## Memory Management

### Garbage Collection Differences

**JavaScript:**
- Reference counting with cycle detection
- Generational garbage collection
- WeakMap/WeakSet for weak references

**Go Implementation:**
- Concurrent mark-and-sweep GC
- No weak references
- Different memory pressure characteristics

### Implications:
- Memory usage patterns differ between implementations
- Go may retain memory longer due to GC timing
- No equivalent to JavaScript's WeakMap for caching

## Test-Specific Differences

### Categories of Skipped Tests

1. **Unicode/Surrogate Tests:**
   - `string-invalid-surrogates.json`
   - `substring-invalid-surrogates.json`

2. **Map Ordering Tests:**
   - Tests using `$keys()` with specific order expectations
   - Object property iteration tests

3. **Async/Promise Tests:**
   - All tests in `async_function_test.go`
   - HTTP request handling tests
   - Callback-based tests

4. **JavaScript-Specific Features:**
   - `$clone()` function tests
   - Prototype manipulation tests
   - Recursive function definition tests
   - Regex literal tests

5. **Large Number Tests:**
   - Word formatting for numbers > 9.2×10^18
   - Tests expecting JavaScript's extended numeric range

### Test Workarounds

The implementation includes specific workarounds for certain test cases:

```go
// Special handling for library.loans tests with $keys()
if strings.Contains(expr, "$keys(") && strings.Contains(expr, "library.loans") {
    equal = deepEqualWithUnorderedStringArrays(result, testcase.Result)
}
```

This represents a deviation from proper implementation - the code recognizes specific test patterns and applies special comparison logic rather than fixing the underlying ordering issue.

## Summary

Despite these differences, the Go implementation achieves high compatibility with the JavaScript reference:

- **Semantic Compatibility:** Core JSONata semantics are preserved
- **Error Code Compatibility:** Same error codes and similar messages
- **Function Compatibility:** Most built-in functions behave identically
- **Expression Compatibility:** Same expression syntax and evaluation rules

The differences primarily stem from fundamental platform constraints rather than implementation choices. Users should be aware of these differences when:

1. Processing Unicode text with surrogates
2. Relying on object key ordering
3. Working with very large numbers
4. Using async/Promise patterns
5. Expecting specific regex engine features

For most JSONata use cases, these differences will not impact functionality, and the Go implementation provides a faithful and performant alternative to the JavaScript reference.

## API Differences

### Compilation Function

**JavaScript:**
```javascript
const expression = jsonata("$.foo.bar");
// or
const expression = jsonata.compile("$.foo.bar");
```

**Go Implementation:**
```go
expression, err := jsonata.Compile("$.foo.bar", false)
```

The Go implementation:
- Uses `Compile()` as the primary function name (not `JSONata()`)
- Requires explicit error handling (Go convention)
- Takes a boolean parameter for error recovery mode instead of an options object

### Error Recovery Mode

**JavaScript:**
```javascript
// No built-in error recovery mode in standard JSONata
```

**Go Implementation:**
```go
// Enable error recovery to collect multiple parse errors
expression, err := jsonata.Compile("malformed expression", true)
if err != nil {
    // Primary error available
    if expression != nil {
        // Additional errors available via expression.Errors()
        for _, e := range expression.Errors() {
            // Process additional errors
        }
    }
}
```

### Removed Features

The Go implementation intentionally does not include:

1. **Custom Regex Engine Support**
   - No pluggable regex engine option
   - Uses Go's standard `regexp` package exclusively
   - JavaScript regex syntax differences apply (see Regular Expressions section)

2. **MustCompile Function**
   - No panic-based compilation function
   - All errors returned as values (Go best practice)

3. **Options Struct**
   - Simplified API with boolean recovery parameter
   - No complex configuration options

### Function Registration

**JavaScript:**
```javascript
expression.registerFunction('myFunc', implementation, signature);
```

**Go Implementation:**
```go
err := expression.RegisterFunction("myFunc", implementation, signature)
```

### Variable Assignment

Both implementations support variable assignment with identical syntax:

**JavaScript:**
```javascript
expression.assign('varName', value);
```

**Go Implementation:**
```go
expression.Assign("varName", value)
```

### Evaluation Methods

**JavaScript:**
```javascript
const result = await expression.evaluate(data, bindings);
```

**Go Implementation:**
```go
// For direct Go types
result, err := expression.Eval(data, bindings)

// For JSON input/output
resultJSON, err := expression.Evaluate(inputJSON, bindings)
```

The Go implementation provides two evaluation methods:
- `Eval()`: Works with Go types directly
- `Evaluate()`: Works with JSON byte arrays

### Performance Control

**Go Implementation Only:**
```go
// Set maximum recursion depth
expression.SetMaxDepth(100)

// Set maximum execution time
expression.SetMaxTime(5000) // milliseconds
```

These methods are unique to the Go implementation for controlling resource usage.