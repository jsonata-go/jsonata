# JSONata Go API Documentation

This document describes the public API for the Go implementation of JSONata. The API is designed to be simple and intuitive while providing full access to JSONata's powerful query and transformation capabilities.

## Table of Contents

1. [Core Functions](#core-functions)
2. [Types](#types)
3. [Expression Methods](#expression-methods)
4. [Error Handling](#error-handling)
5. [Error Recovery Mode](#error-recovery-mode)
6. [Constants](#constants)
7. [Usage Patterns](#usage-patterns)
8. [Thread Safety](#thread-safety)
9. [Performance Considerations](#performance-considerations)
10. [Production Server Best Practices for DoS Protection](#production-server-best-practices-for-dos-protection)
11. [Limitations](#limitations)

## Core Functions

### Version

```go
func Version() string
```

Returns the current version of the JSONata Go implementation.

**Purpose:** Provides version information for compatibility checking and debugging.

**Returns:**
- `string`: Version string (e.g., "v2.0.6")

**Example:**
```go
fmt.Printf("JSONata Go version: %s\n", jsonata.Version())
```

### Compile

```go
func Compile(expr string, recover bool) (*Expression, error)
```

Compiles a JSONata expression with optional error recovery. This is the primary entry point for using JSONata.

**Purpose:** Compiles a JSONata expression into an executable form that can be evaluated multiple times against different JSON data.

**Parameters:**
- `expr` (string): The JSONata expression to compile
- `recover` (bool): Enable error recovery mode for parsing invalid syntax. When true, parsing attempts to continue past syntax errors and collects error information for debugging

**Returns:**
- `*Expression`: Compiled expression object
- `error`: Syntax error if the expression is invalid

**Example:**
```go
// Simple usage
expr, err := jsonata.Compile("$.users[age > 21].name", false)
if err != nil {
    log.Fatal("Invalid expression:", err)
}

// With error recovery
expr, err := jsonata.Compile("$.data", true)
if err != nil {
    // Primary error
    fmt.Printf("Main error: %v\n", err)
    
    // Additional errors (if any)
    if expr != nil {
        for _, e := range expr.Errors() {
            fmt.Printf("Additional: %v\n", e)
        }
    }
}
```

**Note:** In this Go implementation, `Compile()` is the primary function for creating JSONata expressions, aligning with other JSONata implementations.


### Parse

```go
func Parse(expr string) (*ASTNode, error)
```

Parses a JSONata expression into an Abstract Syntax Tree without creating an evaluable expression.

**Purpose:** Low-level parsing function for tools that need to analyze or transform JSONata expressions. Useful for syntax highlighting, linting, or expression rewriting.

**Parameters:**
- `expr` (string): The JSONata expression to parse

**Returns:**
- `*ASTNode`: Root node of the parsed AST
- `error`: Syntax error if parsing fails

### ParseWithRecovery

```go
func ParseWithRecovery(expr string) (*ASTNode, error)
```

Parses a JSONata expression with error recovery, attempting to build a partial AST even with syntax errors.

**Purpose:** Enables tools like IDEs to provide better error messages and partial understanding of malformed expressions. Collects multiple errors instead of stopping at the first one.

**Parameters:**
- `expr` (string): The JSONata expression to parse

**Returns:**
- `*ASTNode`: Root node of the (possibly partial) AST
- `error`: Primary syntax error (additional errors may be in AST.Errors)

### SetDefaultMaxDepth

```go
func SetDefaultMaxDepth(maxDepth int)
```

Sets the global default maximum recursion depth for all new expressions.

**Purpose:** Provides a way to set a default recursion limit that will be applied to all newly compiled expressions, avoiding the need to call SetMaxDepth on each expression individually.

**Parameters:**
- `maxDepth` (int): Maximum recursion depth (0 = unlimited, which is the default)

**Example:**
```go
// Set a global default of 100 levels for all new expressions
jsonata.SetDefaultMaxDepth(100)

// This expression will have the 100 level limit automatically
expr, _ := jsonata.Compile("$.data", false)
```

### SetDefaultMaxTime

```go
func SetDefaultMaxTime(maxMs int)
```

Sets the global default maximum execution time for all new expressions.

**Purpose:** Provides a way to set a default timeout that will be applied to all newly compiled expressions, ensuring consistent timeout behavior across your application.

**Parameters:**
- `maxMs` (int): Maximum execution time in milliseconds (0 = unlimited, which is the default)

**Example:**
```go
// Set a global default of 5 seconds for all new expressions
jsonata.SetDefaultMaxTime(5000)

// This expression will have the 5 second timeout automatically
expr, _ := jsonata.Compile("$.data", false)
```

### SetDefaultMaxRange

```go
func SetDefaultMaxRange(maxRange int)
```

Sets the global default maximum size for range expressions for all new expressions.

**Purpose:** Provides a way to set a default range limit that will be applied to all newly compiled expressions, protecting against denial-of-service attacks from large range expressions.

**Parameters:**
- `maxRange` (int): Maximum allowed range size (0 = unlimited, which is the default)

**Example:**
```go
// Set a global default of 10,000 elements for all new expressions
jsonata.SetDefaultMaxRange(10000)

// This expression will have the range limit automatically
expr, _ := jsonata.Compile("[1..n]", false)
```

**Note:** These default settings only affect expressions compiled after the defaults are set. Existing compiled expressions retain their original limits.

## Types

### Expression

```go
type Expression struct {
    // Private fields
}
```

Represents a compiled JSONata expression ready for evaluation.

**Purpose:** Encapsulates a parsed and validated JSONata expression that can be efficiently evaluated multiple times against different input data. Thread-safe for concurrent evaluation.

### JSONataFunc

```go
type JSONataFunc func(args []interface{}) (interface{}, error)
```

Function signature for custom JSONata functions.

**Purpose:** Defines the interface for functions that can be registered with JSONata expressions.

**Parameters:**
- `args`: Slice of evaluated arguments

**Returns:**
- `interface{}`: Function result
- `error`: Runtime error


### JSONataError

```go
type JSONataError struct {
    Code     string      // Error code (e.g., "T0410", "S0201")
    Position int         // Character position in expression
    Token    string      // Token that caused the error  
    Value    interface{} // Problematic value
    Value2   interface{} // Second value (for binary operations)
    Message  string      // Human-readable error message
    Stack    string      // Go stack trace
    Index    int         // Argument index for function errors
    Expected string      // Expected type/value
    // Additional fields...
}
```

Structured error type providing detailed context about JSONata errors.

**Purpose:** Rich error information helps developers quickly identify and fix issues in JSONata expressions or input data.

**Methods:**
- `Error() string`: Implements Go's error interface
- `String() string`: Detailed error description

### JSONataNull

```go
type JSONataNull struct{}

var JSONNull = &JSONataNull{}
```

Represents an explicit null value (as distinct from Go's nil which represents undefined).

**Purpose:** Maintains JavaScript's null/undefined distinction in JSONata expressions. Used internally but exposed for advanced scenarios.


### ASTNode

```go
type ASTNode struct {
    Type     string      // Node type (e.g., "path", "literal", "function")
    Value    interface{} // Node value
    Position int         // Source position
    // Additional fields for different node types...
}
```

Node in the JSONata Abstract Syntax Tree.

**Purpose:** Represents the structure of parsed JSONata expressions. Primarily used internally but exposed for advanced tooling scenarios.

## Expression Methods

### Evaluate

```go
func (e *Expression) Evaluate(inputJSON []byte, bindings map[string]interface{}) ([]byte, error)
```

Evaluates the expression against JSON input and returns JSON output.

**Purpose:** Full JSON-to-JSON transformation. Handles both parsing input and serializing output as JSON.

**Parameters:**
- `inputJSON` ([]byte): Input data as JSON bytes
- `bindings` (map[string]interface{}): Variable bindings (can be nil)

**Returns:**
- `[]byte`: Result as JSON bytes
- `error`: Parsing, evaluation, or serialization error

**Example:**
```go
input := []byte(`{"price": 100, "tax": 0.15}`)
bindings := map[string]interface{}{
    "discount": 0.1,
}
result, err := expr.Evaluate(input, bindings)
// result contains JSON: {"total": 103.5}
```

### SetMaxDepth

```go
func (e *Expression) SetMaxDepth(depth int)
```

Sets the maximum recursion depth for expression evaluation.

**Purpose:** Prevents stack overflow from deeply recursive expressions or circular references. Essential for processing untrusted expressions or data.

**Parameters:**
- `depth` (int): Maximum recursion depth (0 = unlimited)

**Example:**
```go
expr.SetMaxDepth(100) // Limit recursion to 100 levels
```

### SetMaxTime

```go
func (e *Expression) SetMaxTime(maxMs int)
```

Sets the maximum execution time for expression evaluation.

**Purpose:** Prevents runaway expressions from consuming excessive CPU time. Essential for processing untrusted expressions or when dealing with large datasets.

**Parameters:**
- `maxMs` (int): Maximum execution time in milliseconds (0 = unlimited)

**Example:**
```go
expr.SetMaxTime(5000) // Limit execution to 5 seconds
```

**Note:** The timeout is checked at strategic points during evaluation, so very fast operations may complete even if they would exceed the limit. The error code "U1002" is returned when timeout is exceeded.

### SetMaxRange

```go
func (e *Expression) SetMaxRange(maxRange int)
```

Sets the maximum size for range expressions to prevent denial-of-service attacks.

**Purpose:** Protects against malicious expressions that attempt to create extremely large arrays using the range operator (e.g., `[1..1000000000]`). This is a critical security feature when evaluating untrusted expressions.

**Parameters:**
- `maxRange` (int): Maximum allowed range size (0 = use default limit of 10 million)

**Example:**
```go
expr.SetMaxRange(10000) // Limit ranges to 10,000 elements
```

**Note:** The default limit is 10 million elements. When a range exceeds the limit, error code "D2014" is returned. This protection applies to expressions like `[start..end]` where `end - start + 1` would exceed the limit.

### RegisterFunction

```go
func (e *Expression) RegisterFunction(name string, implementation JSONataFunc, signature string) error
```

Registers a custom function for use in the expression.

**Purpose:** Extends JSONata with domain-specific functions, allowing integration with Go application logic.

**Parameters:**
- `name` (string): Function name (without $ prefix)
- `implementation` (JSONataFunc): Function implementation
- `signature` (string): JSONata signature string for validation

**Returns:**
- `error`: Signature parsing error

**Example:**
```go
err := expr.RegisterFunction("uuid", 
    func(args []interface{}) (interface{}, error) {
        return uuid.New().String(), nil
    },
    "<:s>") // No args, returns string
```

### RegisterGlobalFunction

```go
func RegisterGlobalFunction(name string, implementation JSONataFunc, signature string) error
```

Registers a custom function globally for all JSONata expressions.

**Purpose:** Adds domain-specific functions that are available to all expressions without needing to register them individually. Functions registered globally are available immediately to all newly compiled expressions.

**Parameters:**
- `name` (string): Function name (without $ prefix)
- `implementation` (JSONataFunc): Function implementation
- `signature` (string): JSONata signature string for validation

**Returns:**
- `error`: Signature parsing error

**Example:**
```go
// Register once at application startup
err := jsonata.RegisterGlobalFunction("uuid", 
    func(args []interface{}) (interface{}, error) {
        return uuid.New().String(), nil
    },
    "<:s>") // No args, returns string

// Now all expressions can use $uuid()
expr1, _ := jsonata.Compile("$uuid()", false)
expr2, _ := jsonata.Compile("{ id: $uuid(), name: name }", false)
```

**Note:** Global functions are bound to the static frame and inherited by all expressions. They cannot be overridden by expression-specific registrations.

### Assign

```go
func (e *Expression) Assign(name string, value interface{})
```

Assigns a value to a variable in the expression's environment.

**Purpose:** Pre-populate variables that can be referenced in the expression using `$name` syntax.

**Parameters:**
- `name` (string): Variable name (without $ prefix)
- `value` (interface{}): Value to assign

**Example:**
```go
expr.Assign("taxRate", 0.08)
expr.Assign("currentUser", "john@example.com")
// Expression can now use $taxRate and $currentUser
```

### AST

```go
func (e *Expression) AST() interface{}
```

Returns the parsed Abstract Syntax Tree.

**Purpose:** Advanced usage for tools that need to analyze or transform the expression structure.

**Returns:**
- `interface{}`: Root node of the AST (typically *ASTNode)

### Errors

```go
func (e *Expression) Errors() []error
```

Returns parsing errors collected during recovery mode.

**Purpose:** When using recovery mode, provides access to all syntax errors found, not just the first one.

**Returns:**
- `[]error`: Slice of parsing errors (empty if no errors)

**Example:**
```go
expr, _ := jsonata.JSONata("malformed", &Options{Recover: true})
for _, err := range expr.Errors() {
    fmt.Printf("Error: %v\n", err)
}
```

## Error Handling

JSONata errors follow a structured format with specific error codes:

### Error Code Categories

- **D1xxx**: Data errors (wrong types, invalid values)
- **D2xxx**: Data operation errors  
- **D3xxx**: Number formatting errors
- **S0xxx**: Syntax errors (parsing failures)
- **T0xxx**: Type errors (function argument mismatches)
- **T1xxx**: Type errors in operations
- **T2xxx**: Type errors in built-in functions
- **U0xxx**: Unknown references (undefined variables/functions)
- **U1xxx**: Runtime errors (stack overflow, etc.)

### Common Error Codes

- `S0201`: Syntax error in expression
- `T0410`: Wrong number of arguments to function
- `T0411`: Argument type mismatch
- `D1001`: Number out of range
- `U0101`: Unknown variable reference
- `U1001`: Stack overflow (recursion limit)
- `U1002`: Evaluation timeout exceeded
- `U1003`: Unexpected panic during compilation or evaluation

## Error Recovery Mode

Recovery mode attempts to parse invalid expressions and collect multiple errors:

```go
expr, err := jsonata.Compile("$.foo..bar baz", true)
// expr may be partially valid
// err contains primary error
// expr.Errors() contains all collected errors
```

**Purpose:** Improves developer experience in IDEs and tools by providing comprehensive error feedback. This is particularly useful for:
- IDE syntax highlighting and error reporting
- Linting tools that need to report all errors at once
- Development environments that provide real-time feedback

## Constants

The package exports minimal constants. Error codes are defined as string literals within error structures rather than exported constants. This design choice maintains flexibility while keeping the API surface small.

To check for specific error types, use type assertions:

```go
if err != nil {
    if jsonataErr, ok := err.(*jsonata.JSONataError); ok {
        switch jsonataErr.Code {
        case "T0410":  // Wrong argument count
            // Handle specific error
        case "U0101":  // Undefined variable
            // Handle specific error
        }
    }
}
```


## Usage Patterns

### Basic Query

```go
// Compile once, evaluate many times
expr, err := jsonata.Compile("$.orders[status='shipped'].total", false)
if err != nil {
    log.Fatal("Invalid expression:", err)
}

for _, orderData := range allOrders {
    total, err := expr.Eval(orderData)
    if err != nil {
        log.Printf("Failed to process order: %v", err)
        continue
    }
    fmt.Printf("Shipped total: %v\n", total)
}
```

### With Custom Functions

```go
expr, err := jsonata.Compile("$uppercase(name)", false)
if err != nil {
    log.Fatal("Invalid expression:", err)
}
err = expr.RegisterFunction("uppercase", 
    func(args []interface{}) (interface{}, error) {
        if len(args) != 1 {
            return nil, fmt.Errorf("uppercase expects 1 argument")
        }
        if str, ok := args[0].(string); ok {
            return strings.ToUpper(str), nil
        }
        return nil, fmt.Errorf("uppercase expects string argument")
    },
    "<s:s>")
if err != nil {
    log.Fatal("Failed to register function:", err)
}
```

### Error Recovery

```go
expr, err := jsonata.Compile("malformed expression", true)
if err != nil {
    // Primary error
    fmt.Printf("Main error: %v\n", err)
    
    // Additional errors (if any)
    if expr != nil {
        for _, e := range expr.Errors() {
            fmt.Printf("Additional: %v\n", e)
        }
    }
}
```

## Thread Safety

`Expression` objects are safe for concurrent use after compilation. Multiple goroutines can call `Eval()` on the same expression simultaneously. However, the expression's environment (registered functions) should not be modified during evaluation.

## Performance Considerations

1. **Compile Once**: Compilation is expensive; compile expressions once and reuse
2. **Set Security Limits**: Always set `MaxDepth`, `MaxTime`, and `MaxRange` when processing untrusted data
3. **Set Time Limits**: Use `SetMaxTime` to prevent runaway expressions from consuming excessive CPU
4. **Set Range Limits**: Use `SetMaxRange` to prevent DoS attacks via large range expressions
5. **Reuse Expressions**: Expression objects are thread-safe and can be shared
6. **Avoid Excessive Nesting**: Deeply nested expressions impact performance

## Production Server Best Practices for DoS Protection

When deploying JSONata in production servers that accept user-provided expressions or process untrusted data, implement these security measures to prevent denial-of-service attacks:

### 1. Always Set Resource Limits

Configure all three protection mechanisms for every expression:

```go
expr, err := jsonata.Compile(userExpression, false)
if err != nil {
    return fmt.Errorf("invalid expression: %w", err)
}

// Essential security limits
expr.SetMaxDepth(50)      // Prevent stack overflow from deep recursion
expr.SetMaxTime(1000)     // 1 second timeout
expr.SetMaxRange(10000)   // Limit range expressions to 10k elements
```

### 2. Validate Expression Complexity

Pre-screen expressions before compilation:

```go
func validateExpression(expr string) error {
    // Check expression length
    if len(expr) > 10000 {
        return errors.New("expression too long")
    }
    
    // Check for potentially dangerous patterns
    dangerousPatterns := []string{
        "**",           // Recursive descent can be expensive
        "[0..1000000",  // Large ranges
        "function(",    // Nested function definitions
    }
    
    for _, pattern := range dangerousPatterns {
        if strings.Contains(expr, pattern) {
            return fmt.Errorf("potentially dangerous pattern: %s", pattern)
        }
    }
    
    return nil
}
```

### 3. Implement Rate Limiting

Limit expression evaluations per client:

```go
type RateLimiter struct {
    requests map[string][]time.Time
    mu       sync.Mutex
    limit    int
    window   time.Duration
}

func (rl *RateLimiter) Allow(clientID string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    cutoff := now.Add(-rl.window)
    
    // Clean old requests
    requests := rl.requests[clientID]
    validRequests := []time.Time{}
    for _, t := range requests {
        if t.After(cutoff) {
            validRequests = append(validRequests, t)
        }
    }
    
    if len(validRequests) >= rl.limit {
        return false
    }
    
    rl.requests[clientID] = append(validRequests, now)
    return true
}
```

### 4. Monitor Resource Usage

Track and alert on suspicious patterns:

```go
type ExpressionMonitor struct {
    timeouts     atomic.Int64
    rangeErrors  atomic.Int64
    depthErrors  atomic.Int64
}

func (m *ExpressionMonitor) EvaluateWithMonitoring(expr *jsonata.Expression, data []byte) ([]byte, error) {
    result, err := expr.Evaluate(data, nil)
    
    if err != nil {
        if jsonataErr, ok := err.(*jsonata.JSONataError); ok {
            switch jsonataErr.Code {
            case "U1002":  // Timeout
                m.timeouts.Add(1)
            case "D2014":  // Range limit exceeded
                m.rangeErrors.Add(1)
            case "U1001":  // Stack overflow
                m.depthErrors.Add(1)
            }
        }
    }
    
    return result, err
}
```

### 5. Use Sandboxed Execution

For highest security, run JSONata in isolated environments:

```go
// Example: Execute in a separate process with resource limits
func sandboxedEvaluate(expression, data string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    
    cmd := exec.CommandContext(ctx, "./jsonata-sandbox", expression)
    cmd.Stdin = strings.NewReader(data)
    
    // Set process resource limits
    cmd.Env = []string{
        "JSONATA_MAX_MEMORY=100M",
        "JSONATA_MAX_CPU=1s",
    }
    
    output, err := cmd.Output()
    if ctx.Err() == context.DeadlineExceeded {
        return "", errors.New("evaluation timeout")
    }
    
    return string(output), err
}
```

### 6. Input Data Validation

Validate and limit input data size:

```go
const maxInputSize = 10 * 1024 * 1024  // 10MB

func validateInput(data []byte) error {
    if len(data) > maxInputSize {
        return errors.New("input data too large")
    }
    
    // Validate JSON structure
    var temp interface{}
    if err := json.Unmarshal(data, &temp); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }
    
    // Check nesting depth
    if depth := jsonDepth(temp); depth > 100 {
        return errors.New("JSON nesting too deep")
    }
    
    return nil
}

func jsonDepth(v interface{}) int {
    switch val := v.(type) {
    case map[string]interface{}:
        maxDepth := 0
        for _, v := range val {
            if d := jsonDepth(v); d > maxDepth {
                maxDepth = d
            }
        }
        return maxDepth + 1
    case []interface{}:
        maxDepth := 0
        for _, v := range val {
            if d := jsonDepth(v); d > maxDepth {
                maxDepth = d
            }
        }
        return maxDepth + 1
    default:
        return 0
    }
}
```

### 7. Secure Default Configuration

Create a secure wrapper for production use:

```go
type SecureJSONata struct {
    maxDepth     int
    maxTime      int
    maxRange     int
    maxExprLen   int
    rateLimiter  *RateLimiter
}

func NewSecureJSONata() *SecureJSONata {
    return &SecureJSONata{
        maxDepth:   50,
        maxTime:    1000,  // 1 second
        maxRange:   10000,
        maxExprLen: 5000,
        rateLimiter: &RateLimiter{
            limit:    100,
            window:   time.Minute,
            requests: make(map[string][]time.Time),
        },
    }
}

func (s *SecureJSONata) Evaluate(clientID, expression string, data []byte) ([]byte, error) {
    // Rate limiting
    if !s.rateLimiter.Allow(clientID) {
        return nil, errors.New("rate limit exceeded")
    }
    
    // Expression validation
    if len(expression) > s.maxExprLen {
        return nil, errors.New("expression too long")
    }
    
    // Input validation
    if err := validateInput(data); err != nil {
        return nil, err
    }
    
    // Compile with limits
    expr, err := jsonata.Compile(expression, false)
    if err != nil {
        return nil, err
    }
    
    expr.SetMaxDepth(s.maxDepth)
    expr.SetMaxTime(s.maxTime)
    expr.SetMaxRange(s.maxRange)
    
    return expr.Evaluate(data, nil)
}
```

### 8. Monitoring and Alerting

Implement comprehensive monitoring:

```go
// Metrics to track:
// - Expression compilation time
// - Expression evaluation time
// - Error rates by type
// - Resource limit violations
// - Unique expressions per time window
// - Data size distribution

// Example Prometheus metrics
var (
    evaluationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "jsonata_evaluation_duration_seconds",
            Help: "Duration of JSONata expression evaluation",
        },
        []string{"status"},
    )
    
    resourceLimitViolations = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "jsonata_resource_limit_violations_total",
            Help: "Number of resource limit violations",
        },
        []string{"limit_type"},
    )
)
```

### Summary

For production deployments:
1. **Never trust user input** - Always validate and limit expressions and data
2. **Set all limits** - Use SetMaxDepth, SetMaxTime, and SetMaxRange on every expression
3. **Monitor everything** - Track resource usage and error patterns
4. **Fail fast** - Reject suspicious patterns before evaluation
5. **Isolate execution** - Consider process-level sandboxing for untrusted expressions
6. **Plan for scale** - Implement rate limiting and resource pooling

These practices ensure JSONata can be safely used in production environments without exposing your servers to denial-of-service attacks.

## Limitations

1. **No Async Support**: Unlike JavaScript, no Promise/async function support
2. **Map Order**: Object key iteration order is not guaranteed
3. **Unicode**: UTF-8 handling differs from JavaScript's UTF-16
4. **Number Precision**: Limited by Go's int64/float64 types