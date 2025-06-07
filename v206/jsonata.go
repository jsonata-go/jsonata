// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

/*
JSONata - A Go Implementation of the JSONata Query and Transformation Language
=============================================================================

Portions Copyright IBM Corp. 2016, 2017 All Rights Reserved
Project name: JSONata
This project is licensed under the MIT License, see LICENSE

Overview:
--------
JSONata is a lightweight query and transformation language for JSON data. This Go implementation
transliterates the core JavaScript reference implementation, maintaining semantic equivalence
while adapting to Go's type system and concurrency model.

Core Concepts:
-------------

1. **Declarative Functional Language**
   JSONata follows a declarative functional programming paradigm where expressions describe
   WHAT to compute rather than HOW to compute it. It's built on map/filter/reduce operations
   exposed through intuitive syntax.

2. **Sequence Processing Model**
   JSONata operates on sequences of values. Understanding the sequence/singleton/array distinction
   is fundamental to how JSONata works:

   - Empty sequence: Represents "nothing" or "no match"
   - Singleton sequence: Single value, equivalent to the value itself
   - Multi-value sequence: Represented as JSON array, subject to flattening rules

   CRITICAL: Arrays from input JSON are preserved, but path navigation results become sequences
   that are automatically flattened unless explicitly preserved with array constructors.

3. **Path Expression Evaluation**
   Path expressions are evaluated left-to-right with each stage transforming the input sequence:
   - Map (`seq.expr`): Evaluates expression for each item, flattens results
   - Filter (`seq[expr]`): Filters items by predicate
   - Sort (`seq^(expr)`): Re-orders sequence
   - Index (`seq#$var`): Binds position variable
   - Join (`seq@$var`): Binds context variable
   - Reduce (`seq{key:value}`): Groups and aggregates

4. **Context Management**
   JSONata maintains evaluation context as expressions navigate through data:
   - `$` refers to current context value
   - `$$` refers to input document root
   - Context flows through path stages and can be captured with variables

5. **Function System**
   Functions are first-class values that can be stored, passed, and returned:
   - Built-in functions are bound in the static environment
   - User-defined functions can be created with lambda expressions
   - Functions capture their lexical environment (closures)
   - Partial application allows creating specialized functions

Implementation Notes:
--------------------
This Go implementation must carefully handle:
- Sequence flattening vs array preservation
- Context management and variable scoping
- Type coercion between Go types and JSONata's type system
- Function binding and closure capture
- Error handling with graceful degradation
*/

package jsonata

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Version returns the current version of the JSONata Go implementation
func Version() string {
	return "v2.0.6"
}

// Debug enables debug logging when set to true
var Debug bool

// Global default limits for DoS protection
var (
	defaultMaxDepth int = 0 // 0 means unlimited
	defaultMaxTime  int = 0 // 0 means unlimited (milliseconds)
	defaultMaxRange int = 0 // 0 means unlimited
)

// SetDefaultMaxDepth sets the global default maximum recursion depth for all new expressions
// A value of 0 means unlimited depth (default)
func SetDefaultMaxDepth(maxDepth int) {
	defaultMaxDepth = maxDepth
}

// SetDefaultMaxTime sets the global default maximum execution time for all new expressions
// A value of 0 means unlimited time (default)
// Time is specified in milliseconds
func SetDefaultMaxTime(maxMs int) {
	defaultMaxTime = maxMs
}

// SetDefaultMaxRange sets the global default maximum size for range expressions for all new expressions
// A value of 0 means unlimited range (default)
func SetDefaultMaxRange(maxRange int) {
	defaultMaxRange = maxRange
}

// JSONataFunc is the unified function signature for all JSONata functions
// This enables consistent function handling and user-defined functions
type JSONataFunc func(args []interface{}) (interface{}, error)

/*
Compile - Main Entry Point and Expression Compilation
=====================================================

This function parses a JSONata expression string into an executable Expression object.
The compilation process involves:
1. Parsing the expression into an Abstract Syntax Tree (AST)
2. Setting up the execution environment with built-in functions
3. Preparing timestamp functions ($now, $millis) for consistent evaluation

Parameters:
  - expr: JSONata expression string (e.g., "$.person.name", "$sum(values)", "books[price > 10]")
  - recover: Enable error recovery mode for parsing invalid syntax.
    When true, parsing attempts to continue past syntax errors
    and collects error information for debugging

Returns:
- Compiled Expression ready for evaluation against JSON data
- Error if expression syntax is invalid

The Expression encapsulates:
- Parsed AST representation of the expression
- Evaluation environment with function bindings
- Timestamp for consistent $now()/$millis() results across evaluation
*/
func Compile(expr string, recoveryMode bool) (compiledExpr *Expression, compileErr error) {
	// Add panic recovery to ensure any crashes are converted to errors
	defer func() {
		if r := recover(); r != nil {
			// Convert panic to error
			var panicErr error
			switch v := r.(type) {
			case error:
				panicErr = v
			case string:
				panicErr = &JSONataError{
					Code:    "U1003",
					Message: v,
					Stack:   getStack(),
				}
			default:
				panicErr = &JSONataError{
					Code:    "U1003",
					Message: fmt.Sprintf("Unexpected panic: %v", r),
					Stack:   getStack(),
				}
			}
			compileErr = panicErr
			compiledExpr = nil
		}
	}()

	// Parse the JSONata expression into an Abstract Syntax Tree
	// This transforms the textual expression into a structured representation
	// that can be efficiently evaluated against input data
	var ast *ASTNode
	var err error

	// Choose parsing strategy based on recoveryMode parameter
	// Recovery mode attempts to parse invalid syntax for error reporting
	if recoveryMode {
		ast, err = ParseWithRecovery(expr)
	} else {
		ast, err = Parse(expr)
	}

	if err != nil {
		// Enhance error with human-readable message before returning
		populateMessage(err) // possible side-effects on `err`
		return nil, err
	}

	// Extract any parse errors from AST for later handling
	// In recovery mode, parsing may succeed while collecting errors
	var parseErrors []error // JS: handled parsing errors differently, no separate variable
	if ast.Errors != nil {
		parseErrors = ast.Errors
		ast.Errors = nil
	}

	// Create execution environment inheriting from static built-in functions
	// This establishes the lexical scope chain for function resolution
	environment := createFrame(staticFrame)

	// Prepare timestamp for $now() and $millis() functions
	// The timestamp is captured at compilation time but updated at evaluation time
	// to ensure consistent time values throughout a single evaluation cycle
	timestamp := time.Now() // will be overridden on each call to evaluate() // JS: no separate timestamp variable, handled differently

	// Bind temporal functions with access to the shared timestamp
	// These functions provide JSONata's date/time capabilities
	environment.bind("now", defineFunction(func(args []interface{}) (interface{}, error) {
		var picture, timezone string
		if len(args) > 0 {
			if pic, ok := args[0].(string); ok {
				picture = pic
			}
		}
		if len(args) > 1 {
			if tz, ok := args[1].(string); ok {
				timezone = tz
			}
		}
		return dateTime.fromMillis(timestamp.UnixMilli(), picture, timezone)
	}, "<s?s?:s>"))
	environment.bind("millis", defineFunction(func(args []interface{}) (interface{}, error) {
		// TRANSLITERATION: JavaScript millis() uses evaluation-time timestamp via closure
		// In Go, this should be overridden by special handling in applyProcedure
		return float64(timestamp.UnixMilli()), nil
	}, "<:n>"))

	// Create expression with default limits
	compiledExpr = &Expression{
		ast:         ast,         // Parsed abstract syntax tree
		environment: environment, // Function bindings and lexical scope
		errors:      parseErrors, // Any parsing errors from recovery mode
		timestamp:   &timestamp,  // Shared timestamp for temporal functions
	}

	// Apply global default limits
	if defaultMaxDepth > 0 {
		compiledExpr.SetMaxDepth(defaultMaxDepth)
	}
	if defaultMaxTime > 0 {
		compiledExpr.SetMaxTime(defaultMaxTime)
	}
	if defaultMaxRange > 0 {
		compiledExpr.SetMaxRange(defaultMaxRange)
	}

	return compiledExpr, nil
}

/*
Expression - Compiled JSONata Expression Ready for Evaluation
==========================================================

An Expression represents a compiled JSONata expression that can be evaluated
multiple times against different input data. The expression encapsulates:

Fields:
- ast: The parsed Abstract Syntax Tree representing the expression structure
- environment: Lexical scope containing function bindings and variables
- errors: Any parse errors collected during recovery mode parsing
- timestamp: Shared timestamp ensuring $now()/$millis() consistency within evaluation

The Expression maintains immutable state after compilation, allowing safe
concurrent evaluation against different input data while preserving the
original expression semantics.
*/
type Expression struct {
	ast         interface{}   // Parsed AST ready for evaluation
	environment *Frame        // Function bindings and lexical scope
	errors      []error       // Parse errors from recovery mode
	timestamp   *time.Time    // Shared timestamp for temporal functions
	maxTime     time.Duration // Maximum execution time (0 = unlimited)
	startTime   time.Time     // Evaluation start time for timeout checking
	maxRange    int           // Maximum size for range expressions (0 = unlimited)
}

// SetMaxDepth sets the maximum recursion depth for the expression
// A value of 0 means unlimited depth (default)
func (e *Expression) SetMaxDepth(maxDepth int) {
	if e.environment.depthCounter == nil {
		// Initialize depth tracking
		depth := 0
		e.environment.depthCounter = &depth
	}
	e.environment.maxDepth = maxDepth
}

// SetMaxTime sets the maximum execution time for the expression
// A value of 0 means unlimited time (default)
// This prevents runaway expressions from consuming excessive CPU time
func (e *Expression) SetMaxTime(maxMs int) {
	if maxMs > 0 {
		e.maxTime = time.Duration(maxMs) * time.Millisecond
	} else {
		e.maxTime = 0
	}
}

// SetMaxRange sets the maximum size for range expressions
// A value of 0 means unlimited range (default)
// This prevents denial-of-service attacks using expressions like [1..1000000000]
func (e *Expression) SetMaxRange(maxRange int) {
	e.maxRange = maxRange
	// Also store in environment for access during evaluation
	if e.environment != nil {
		e.environment.maxRange = maxRange
	}
}

/*
Evaluate - Execute JSONata Expression Against Input Data
======================================================

This method evaluates the compiled JSONata expression against input JSON data,
applying the sequence processing model and context management that define
JSONata's behavior.

Key Evaluation Steps:

1. **Error Checking**: Validates that expression compiled successfully
2. **Environment Setup**: Creates execution context with variable bindings
3. **Context Binding**: Establishes root context ($) for path navigation
4. **Timestamp Capture**: Updates temporal functions for consistent time values
5. **Array Wrapping**: Wraps input arrays to preserve JSONata semantics
6. **Expression Evaluation**: Executes the AST against prepared input/context
7. **Result Processing**: Applies sequence flattening rules to output

Parameters:
- input: JSON data to evaluate against (any valid JSON value)
- bindings: Optional variable bindings for the evaluation context

Returns:
- Evaluated result following JSONata's sequence processing rules
- Error if evaluation fails or expression had compilation errors

Critical Array Handling:
When input is a JSON array, it gets wrapped in a sequence with outerWrapper=true.
This preserves the semantic difference between:
- Input arrays (preserved structure): [1,2,3] remains [1,2,3]
- Result sequences (subject to flattening): path results become flattened

The outerWrapper flag tells the sequence processing engine that this represents
an input array rather than a computed sequence, preventing inappropriate flattening.
*/
// Evaluate evaluates the expression with JSON input and returns JSON output
func (e *Expression) Evaluate(inputJSON []byte, bindings map[string]interface{}) (resultJSON []byte, evalErr error) {
	// Add panic recovery to ensure any crashes during evaluation are converted to errors
	defer func() {
		if r := recover(); r != nil {
			// Convert panic to error
			switch v := r.(type) {
			case error:
				evalErr = v
			case string:
				evalErr = &JSONataError{
					Code:    "U1003",
					Message: v,
					Stack:   getStack(),
				}
			default:
				evalErr = &JSONataError{
					Code:    "U1003",
					Message: fmt.Sprintf("Unexpected panic during evaluation: %v", r),
					Stack:   getStack(),
				}
			}
			resultJSON = nil
		}
	}()

	// Reject evaluation if expression compilation encountered syntax errors
	// This prevents executing malformed expressions that could produce
	// unexpected results or runtime panics
	if len(e.errors) > 0 {
		err := &JSONataError{
			Code:     "S0500",
			Position: 0,
		}
		populateMessage(err) // possible side-effects on `err`
		return nil, err
	}

	// Unmarshal JSON input
	var input interface{}
	if len(inputJSON) > 0 {
		if err := json.Unmarshal(inputJSON, &input); err != nil {
			return nil, fmt.Errorf("invalid JSON input: %w", err)
		}
		// Convert JSON null values to JSONataNull to distinguish from undefined
		input = convertNullsToJSONataNull(input)
	}

	// Prepare execution environment with optional variable bindings
	// If bindings are provided, create a new scope frame to avoid
	// contaminating the compiled expression's static environment
	var execEnv *Frame // JS: used 'environment' parameter directly without separate variable
	if bindings != nil {
		// Create child frame inheriting built-in functions but adding user variables
		execEnv = createFrame(e.environment)
		for k, v := range bindings {
			// If the binding value is a JSONataFunc, wrap it in a Function structure
			if jsonataFunc, ok := v.(JSONataFunc); ok {
				execEnv.bind(k, defineFunction(jsonataFunc, ""))
			} else {
				// Try to handle raw function types and wrap them
				switch fn := v.(type) {
				case func([]interface{}) (interface{}, error):
					// This is a JSONataFunc but wasn't recognized due to type assertion
					execEnv.bind(k, defineFunction(JSONataFunc(fn), ""))
				default:
					execEnv.bind(k, v)
				}
			}
		}
	} else {
		// Use static environment directly when no custom bindings needed
		execEnv = e.environment
	}

	// Establish root context for path navigation
	// The $ variable always refers to the input document root,
	// providing the starting point for all path expressions
	execEnv.bind("$", input)

	// Update timestamp for consistent temporal function results
	// All $now() and $millis() calls within this evaluation will
	// return the same timestamp, ensuring deterministic behavior
	*e.timestamp = time.Now()
	execEnv.timestamp = e.timestamp

	// Set evaluation start time for timeout checking
	if e.maxTime > 0 {
		e.startTime = time.Now()
	}

	// Store Expression reference in the environment for timeout checking
	execEnv.expression = e
	// Store maxRange for range limit checking
	execEnv.maxRange = e.maxRange

	// Handle input array wrapping for semantic correctness
	// JSONata distinguishes between input arrays (preserved) and computed sequences (flattened)
	// Input arrays must be wrapped in a special sequence to maintain this distinction
	// Use type assertions instead of reflection for performance
	if arr, ok := input.([]interface{}); ok && !isSequence(input) {
		// Create wrapper sequence that preserves array semantics
		seq := createSequence()
		for _, item := range arr {
			seq.Push(item)
		}
		// Mark as outer wrapper to prevent inappropriate flattening
		seq.SetOuterWrapper(true) // JavaScript: inputSequence.outerWrapper = true
		input = seq
	} else if arrMap, ok := input.([]map[string]interface{}); ok && !isSequence(input) {
		// Handle []map[string]interface{} arrays (common in JSON unmarshaling)
		seq := createSequence()
		for _, item := range arrMap {
			seq.Push(item)
		}
		seq.SetOuterWrapper(true) // JavaScript: inputSequence.outerWrapper = true
		input = seq
	}

	// Execute the compiled expression against prepared input and context
	// This traverses the AST, applying JSONata's evaluation rules to
	// produce the final result following sequence processing semantics
	result, err := evaluate(e.ast, input, execEnv)
	if err != nil {
		// Enhance error with human-readable message for debugging
		populateMessage(err) // possible side-effects on `err`
		return nil, err
	}

	// Check if result is a function type
	switch v := result.(type) {
	case JSONataFunc, *Function,
		func([]interface{}) (interface{}, error),
		func(...interface{}) interface{},
		func(...interface{}) (interface{}, error),
		func(string, int) interface{}:
		return nil, &JSONataError{
			Code:    "T0410",
			Message: "did you mean to put () after that function name?",
		}
	default:
		// Also check if it's a wrapped function in an interface{}
		if _, ok := v.(*Function); ok {
			return nil, &JSONataError{
				Code:    "T0410",
				Message: "did you mean to put () after that function name?",
			}
		}
	}

	// Marshal result to JSON
	// This automatically handles converting internal types like JSONataArray
	// to standard JSON arrays
	if result == nil {
		return []byte("null"), nil
	}

	// Try to marshal and if it fails due to function type, return specific error
	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		// Check if the error is due to function type
		if strings.Contains(marshalErr.Error(), "unsupported type: func") {
			return nil, &JSONataError{
				Code:    "T0410",
				Message: "did you mean to put () after that function name?",
			}
		}
		return nil, marshalErr
	}
	return resultJSON, nil
}

// Assign assigns a value to a variable in the expression's environment
func (e *Expression) Assign(name string, value interface{}) {
	e.environment.bind(name, value)
}

// RegisterFunction registers a custom function
func (e *Expression) RegisterFunction(name string, implementation JSONataFunc, signature string) error {
	fn, err := defineFunctionWithError(implementation, signature)
	if err != nil {
		return err
	}
	e.environment.bind(name, fn)
	return nil
}

// RegisterGlobalFunction registers a custom function globally for all expressions
func RegisterGlobalFunction(name string, implementation JSONataFunc, signature string) error {
	fn, err := defineFunctionWithError(implementation, signature)
	if err != nil {
		return err
	}
	staticFrame.bind(name, fn)
	return nil
}

// AST returns the parsed AST
func (e *Expression) AST() interface{} {
	return e.ast
}

// Errors returns any parsing errors
func (e *Expression) Errors() []error {
	return e.errors
}

// Start of Evaluator code

var staticFrame = createFrame(nil)

// functionDefinitionErrors holds any errors that occurred during function registration
var functionDefinitionErrors []string

/**
 * Evaluate expression against input data
 * JavaScript: jsonata.js lines 50-145
 *
 * This is the core evaluation engine that recursively walks the AST and evaluates
 * each node according to its type. The evaluation is context-sensitive, with the
 * 'input' parameter serving as the context value (accessible via $ in expressions).
 *
 * The algorithm:
 * 1. Null/undefined inputs are passed through unless the expression has explicit handling
 * 2. Each AST node type has specific evaluation logic (literals, paths, operators, etc.)
 * 3. Results are transformed according to JSONata semantics (array flattening, etc.)
 * 4. Errors maintain JavaScript compatibility with specific error codes
 *
 * Key behaviors:
 * - Path expressions that find nothing return undefined (not an error)
 * - Operators have JavaScript-like type coercion
 * - Arrays from paths are flattened, arrays from constructors are not
 * - Functions capture their lexical environment (closures)
 *
 * @param {Object} expr - JSONata expression (AST node)
 * @param {Object} input - Input data to evaluate against (context value)
 * @param {Object} environment - Environment with variable bindings and functions
 * @returns {*} Evaluated result
 */
/*
evaluate - Core JSONata Expression Evaluation Engine
===================================================

This is the heart of the JSONata evaluation engine. It recursively traverses
the Abstract Syntax Tree (AST) and applies JSONata's evaluation semantics,
including sequence processing, context management, and type coercion.

The evaluation process implements JSONata's declarative functional model:
- Each AST node type has specific evaluation semantics
- Results follow sequence flattening rules
- Context flows through expressions as they're evaluated
- Functions are first-class values that capture their lexical environment

Parameters:
- expr: AST node to evaluate (various node types representing different language constructs)
- input: Current context value (the data being processed)
- environment: Lexical environment containing variable bindings and function definitions

Returns:
- Evaluated result following JSONata's type system and sequence rules
- Error if evaluation fails

Node Type Evaluation Strategy:
- "path": Navigate through JSON structure with map/filter operations
- "binary"/"unary": Apply arithmetic, comparison, and logical operations
- "function": Invoke built-in or user-defined functions
- "variable": Resolve variable references in lexical environment
- "literal": Return constant values (strings, numbers, booleans)
- "condition": Evaluate conditional expressions (if-then-else)
- "lambda": Create function values with lexical closure
- And many more specialized node types...

After node-specific evaluation, the engine applies:
1. Predicate filtering (for expressions with [filter] syntax)
2. Grouping operations (for expressions with {key:value} syntax)
3. Sequence processing (singleton unwrapping, flattening rules)
*/
// checkTimeout checks if the evaluation has exceeded the maximum allowed time
func checkTimeout(environment *Frame) error {
	if environment.expression != nil && environment.expression.maxTime > 0 &&
		time.Since(environment.expression.startTime) > environment.expression.maxTime {
		return &JSONataError{
			Code:    "U1002",
			Stack:   getStack(),
			Message: "Evaluation timeout exceeded",
		}
	}
	return nil
}

func evaluate(expr interface{}, input interface{}, environment *Frame) (interface{}, error) {
	var result interface{}

	// Check timeout at the beginning of evaluation
	if err := checkTimeout(environment); err != nil {
		return nil, err
	}

	// Depth tracking for recursion limits
	if environment.depthCounter != nil && !environment.isParallelCall {
		// Increment depth on entry
		*environment.depthCounter++

		// Check depth limit
		if environment.maxDepth > 0 && *environment.depthCounter > environment.maxDepth {
			return nil, &JSONataError{
				Code:    "U1001",
				Stack:   getStack(),
				Message: "recursion depth is greater than that allowed; check for non-terminating recursive function or consider rewriting as tail-recursive",
			}
		}

		// Ensure depth is decremented on exit
		defer func() {
			*environment.depthCounter--
		}()
	}

	// Execute entry callback for debugging/instrumentation
	// This allows external tools to hook into the evaluation process
	entryCallback := environment.lookup(EntryCallbackSymbol)
	if entryCallback != nil {
		if cb, ok := entryCallback.(func(interface{}, interface{}, *Frame) error); ok {
			if err := cb(expr, input, environment); err != nil {
				return nil, err
			}
		}
	}

	// Validate that expr is a proper AST node
	// The parser should only produce *ASTNode values, but we verify for safety
	node, ok := expr.(*ASTNode) // JS: 'expr' used directly with lowercase field names (expr.type, expr.steps, etc.)
	if !ok {
		return nil, &JSONataError{
			Code:  "S0500",
			Stack: getStack(),
		}
	}

	// Dispatch to appropriate evaluation function based on AST node type
	// Each case represents a fundamental language construct in JSONata
	switch node.Type {
	case "path":
		// Path expressions: navigation through JSON structure (e.g., "person.name", "books[0].title")
		// Implements map/filter operations with sequence flattening
		var err error
		result, err = evaluatePath(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "binary":
		// Binary operators: arithmetic (+, -, *, /), comparison (=, !=, <, >), logical (&, |)
		// String concatenation (&), inclusion tests (in), and object merge
		var err error
		result, err = evaluateBinary(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "unary":
		// Unary operators: negation (-), logical not, existence check
		var err error
		result, err = evaluateUnary(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "name":
		// Property names in object construction and navigation
		// Handles field references within path expressions
		var err error
		result, err = evaluateName(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "string", "number", "value":
		// Literal values: strings ("hello"), numbers (42), booleans (true), null
		// These are constant values that evaluate to themselves
		var err error
		result, err = evaluateLiteral(node)
		if err != nil {
			return nil, err
		}
	case "wildcard":
		// Wildcard operator (*): selects all properties of an object or all elements of an array
		// Produces a sequence containing all values at the current level
		var err error
		result, err = evaluateWildcard(node, input)
		if err != nil {
			return nil, err
		}
	case "descendant":
		// Descendant operator (**): recursive descent through nested structures
		// Finds all values at any depth that match subsequent path expressions
		var err error
		result, err = evaluateDescendants(node, input)
		if err != nil {
			return nil, err
		}
	case "parent":
		// Parent operator (%): reference to parent context in nested expressions
		// Allows access to outer scope variables and context values
		result = environment.lookup(node.Slot.Label)
	case "condition":
		// Conditional expressions (if-then-else): boolean logic for branching
		// Supports ternary operator syntax: condition ? then_expr : else_expr
		var err error
		result, err = evaluateCondition(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "block":
		// Block expressions: parenthesized expressions that create variable scope
		// Used for grouping and controlling evaluation order and variable binding
		var err error
		result, err = evaluateBlock(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "bind":
		// Variable binding operator (:=): assigns values to variables
		// Creates new variable bindings in the current lexical scope
		var err error
		result, err = evaluateBindExpression(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "regex":
		// Regular expression literals: pattern matching for string operations
		// Compiled regex patterns used by match, replace, and split functions
		var err error
		result, err = evaluateRegex(node)
		if err != nil {
			return nil, err
		}
	case "function":
		// Function calls: invocation of built-in or user-defined functions
		// Handles argument passing, signature validation, and result processing
		var err error
		result, err = evaluateFunction(node, input, environment, nil)
		if err != nil {
			return nil, err
		}
	case "variable":
		// Variable references: lookup of bound variables in lexical environment
		// Resolves $ references, user variables, and special symbols
		var err error
		result, err = evaluateVariable(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "lambda":
		// Lambda functions: anonymous function creation with lexical closure
		// Creates callable functions that capture their defining environment
		var err error
		result, err = evaluateLambda(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "partial":
		// Partial application: creation of specialized functions using placeholders
		// Supports functional programming patterns with ? placeholder syntax
		var err error
		result, err = evaluatePartialApplication(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "apply":
		// Function application operator (~>): function chaining and composition
		// Enables functional pipeline patterns: value ~> func1 ~> func2
		var err error
		result, err = evaluateApplyExpression(node, input, environment)
		if err != nil {
			return nil, err
		}
	case "transform":
		// Transform expressions: object construction and modification
		// Handles object merging, property updates, and structural transformations
		var err error
		result, err = evaluateTransformExpression(node, input, environment)
		if err != nil {
			return nil, err
		}
	}

	// Apply predicate filters to the evaluation result
	// Predicates implement JSONata's [filter] syntax, allowing conditional selection
	// Multiple predicates are chained, each further refining the result set
	if node.Predicate != nil {
		for _, pred := range node.Predicate {
			var err error
			result, err = evaluateFilter(pred.Expr, result, environment)
			if err != nil {
				return nil, err
			}
		}
	}

	// Apply grouping expressions for object construction and aggregation
	// Group expressions implement JSONata's {key:value} syntax for creating objects
	// Path expressions handle grouping internally, so we skip it for "path" nodes
	if node.Type != "path" && node.Group != nil {
		var err error
		result, err = evaluateGroupExpression(node.Group, result, environment)
		if err != nil {
			return nil, err
		}
	}

	// Execute exit callback for debugging/instrumentation
	// Provides post-evaluation hooks for external tooling and analysis
	exitCallback := environment.lookup(ExitCallbackSymbol)
	if exitCallback != nil {
		if cb, ok := exitCallback.(func(interface{}, interface{}, *Frame, interface{}) error); ok {
			if err := cb(expr, input, environment, result); err != nil {
				return nil, err
			}
		}
	}

	// Apply JSONata's sequence processing rules to the final result - JavaScript: if(result.Sequence === true)
	if arr, ok := result.(*JSONataArray); ok && !arr.IsTupleStream() {

		// Honor explicit array preservation requests (from array constructors)
		if node.KeepArray {
			arr.SetKeepSingleton(true)
		}

		// Apply sequence flattening rules:
		// For constructed arrays, treat them as singletons regardless of their internal length
		effectiveLength := arr.Length()
		if arr.IsCons() {
			effectiveLength = 1
		}

		if effectiveLength == 0 {
			// Empty sequence becomes undefined (represented as nil in Go)
			result = nil
		} else if effectiveLength == 1 {
			// Singleton sequence processing:

			// Special handling for constructed arrays (treated as singletons)
			if arr.IsCons() {
				// Don't wrap constructed arrays here - let them be processed by sequence logic
				// Return the JSONataArray itself so it preserves cons=true property
				result = arr
			} else {
				// Normal singleton processing
				singleElement := arr.Get(0)
				if constructedArr, isConstructed := singleElement.(*JSONataArray); isConstructed && constructedArr.IsCons() {
					if arr.IsKeepSingleton() {
						// Wrap the constructed array result
						result = []interface{}{constructedArr.Data}
					} else {
						// Return the constructed array as-is
						result = constructedArr.Data
					}
				} else if arr.IsKeepSingleton() {
					// Preserve as array (for [] operator and explicit array construction)
					// Convert to plain Go array while preserving sub-structure
					plainArray := make([]interface{}, arr.Length())
					for i := 0; i < arr.Length(); i++ {
						plainArray[i] = arr.Get(i)
					}
					result = plainArray
				} else {
					// Unwrap to single value (normal singleton behavior) - JavaScript: result = result[0]
					result = arr.Get(0)

					// Handle nested sequences - recursively unwrap singleton sequences
					if innerSeq, ok := result.(*JSONataArray); ok && !innerSeq.IsTupleStream() && innerSeq.Length() == 1 && !innerSeq.IsKeepSingleton() {
						result = innerSeq.Get(0)
					}
				}
			}
		} else {
			// Multi-value sequences: Check if they should be preserved as arrays
			if arr.IsKeepSingleton() {
				// Check if this sequence contains constructed arrays
				plainArray := make([]interface{}, arr.Length())
				for i := 0; i < arr.Length(); i++ {
					elem := arr.Get(i)
					// If element is a constructed array, use its data directly
					if constructedArr, isConstructed := elem.(*JSONataArray); isConstructed && constructedArr.IsCons() {
						plainArray[i] = constructedArr.Data
					} else {
						plainArray[i] = elem
					}
				}
				result = plainArray
			} else {
				// Normal multi-value sequences become flattened arrays
				// This ensures the API returns standard Go types rather than internal JSONataArray
				result = arr.Data
			}
		}
		// Note: Empty sequences become nil, singletons are unwrapped, multi-value become arrays
	}
	return result, nil
}

/**
 * Evaluate path expression against input data
 * JavaScript: jsonata.js lines 154-241
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluatePath(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {

	var inputSequence *JSONataArray
	// expr is an array of steps
	// if the first step is a variable reference ($...), including root reference ($$),
	//   then the path is absolute rather than relative
	if arr, ok := input.([]interface{}); ok && expr.Steps[0].Type != "variable" {
		inputSequence = createSequence()
		for _, item := range arr {
			inputSequence.Push(item)
		}
	} else {
		// if input is not an array, make it so
		inputSequence = createSequence()
		// TRANSLITERATION FIX: Always push at least one element for variable paths
		// JavaScript: createSequence(input) always contains the input, even if nil
		inputSequence.Push(input)
	}

	var resultSequence *JSONataArray
	isTupleStream := false
	var tupleBindings *JSONataArray

	// evaluate each step in turn
	for ii := 0; ii < len(expr.Steps); ii++ {
		// Check timeout in loop
		if err := checkTimeout(environment); err != nil {
			return nil, err
		}
		step := expr.Steps[ii]

		if step.Tuple {
			isTupleStream = true
		}

		/*
		 * TRANSLITERATION FROM: JavaScript jsonata.js lines 178-181:
		 *
		 * // if the first step is an explicit array constructor, then just evaluate that (i.e. don't iterate over a context array)
		 * if(ii === 0 && step.consarray) {
		 *     resultSequence = await evaluate(step, inputSequence, environment);
		 * } else {
		 */
		if ii == 0 && step.ConsArray {
			// JavaScript: resultSequence = await evaluate(step, inputSequence, environment);
			res, err := evaluate(step, inputSequence, environment)
			if err != nil {
				return nil, err
			}
			if arr, ok := res.(*JSONataArray); ok {
				resultSequence = arr
			} else {
				resultSequence = createSequence()
				if res != nil {
					resultSequence.Push(res)
				}
			}
		} else {
			if isTupleStream {
				var err error
				tupleBindings, err = evaluateTupleStep(step, inputSequence, tupleBindings, environment)
				if err != nil {
					return nil, err
				}
			} else {
				var err error
				resultSequence, err = evaluateStep(step, inputSequence, environment, ii == len(expr.Steps)-1)

				if err != nil {
					return nil, err
				}
			}
		}

		if !isTupleStream && (resultSequence == nil || resultSequence.Length() == 0) {
			break
		}

		if step.Focus == "" {
			inputSequence = resultSequence
		}
	}

	if isTupleStream {
		if expr.Tuple {
			// tuple stream is carrying ancestry information - keep this
			resultSequence = tupleBindings
		} else {
			resultSequence = createSequence()
			if tupleBindings != nil {
				for ii := 0; ii < tupleBindings.Length(); ii++ {
					if tuple, ok := tupleBindings.Get(ii).(map[string]interface{}); ok {
						resultSequence.Push(tuple["@"])
					}
				}
			}
		}
	}

	/*
	 * TRANSLITERATION FROM: JavaScript jsonata.js lines 211-217:
	 *
	 * if(expr.keepSingletonArray) {
	 *     // if the array is explicitly constructed in the expression and marked to promote singleton sequences to array
	 *     if(Array.isArray(resultSequence) && resultSequence.cons && !resultSequence.sequence) {
	 *         resultSequence = createSequence(resultSequence);
	 *     }
	 *     resultSequence.keepSingleton = true;
	 * }
	 */
	/*
	 * TRANSLITERATION FROM: JavaScript jsonata.js lines 211-217:
	 * if(expr.keepSingletonArray) {
	 *     // if the array is explicitly constructed in the expression and marked to promote singleton sequences to array
	 *     if(Array.isArray(resultSequence) && resultSequence.cons && !resultSequence.sequence) {
	 *         resultSequence = createSequence(resultSequence);
	 *     }
	 *     resultSequence.keepSingleton = true;
	 * }
	 */
	/*
	 * TRANSLITERATION FROM: JavaScript jsonata.js lines 211-217:
	 * if(expr.keepSingletonArray) {
	 *     // if the array is explicitly constructed in the expression and marked to promote singleton sequences to array
	 *     if(Array.isArray(resultSequence) && resultSequence.cons && !resultSequence.sequence) {
	 *         resultSequence = createSequence(resultSequence);
	 *     }
	 *     resultSequence.keepSingleton = true;
	 * }
	 */
	/*
	 * TRANSLITERATION FROM: JavaScript jsonata.js lines 211-217:
	 * if(expr.keepSingletonArray) {
	 *     // if the array is explicitly constructed in the expression and marked to promote singleton sequences to array
	 *     if(Array.isArray(resultSequence) && resultSequence.cons && !resultSequence.sequence) {
	 *         resultSequence = createSequence(resultSequence);
	 *     }
	 *     resultSequence.keepSingleton = true;
	 * }
	 */
	/*
	 * TRANSLITERATION FROM: JavaScript jsonata.js lines 211-217:
	 * if(expr.keepSingletonArray) {
	 *     // if the array is explicitly constructed in the expression and marked to promote singleton sequences to array
	 *     if(Array.isArray(resultSequence) && resultSequence.cons && !resultSequence.sequence) {
	 *         resultSequence = createSequence(resultSequence);
	 *     }
	 *     resultSequence.keepSingleton = true;
	 * }
	 */
	/*
	 * TRANSLITERATION FROM: JavaScript jsonata.js lines 211-217:
	 * if(expr.keepSingletonArray) {
	 *     // if the array is explicitly constructed in the expression and marked to promote singleton sequences to array
	 *     if(Array.isArray(resultSequence) && resultSequence.cons && !resultSequence.sequence) {
	 *         resultSequence = createSequence(resultSequence);
	 *     }
	 *     resultSequence.keepSingleton = true;
	 * }
	 */
	if expr.KeepSingletonArray {
		// JavaScript: resultSequence.keepSingleton = true;
		resultSequence.SetKeepSingleton(true)
	}

	if expr.Group != nil {
		var groupInput interface{}
		if isTupleStream {
			groupInput = tupleBindings
		} else {
			groupInput = resultSequence
		}
		result, err := evaluateGroupExpression(expr.Group, groupInput, environment)
		if err != nil {
			return nil, err
		}
		if arr, ok := result.(*JSONataArray); ok {
			resultSequence = arr
		} else {
			resultSequence = createSequence()
			if result != nil {
				resultSequence.Push(result)
			}
		}
	}

	return resultSequence, nil
}

func createFrameFromTuple(environment *Frame, tuple map[string]interface{}) *Frame {
	frame := createFrame(environment)
	for prop, value := range tuple {
		frame.bind(prop, value)
	}
	return frame
}

/**
 * Evaluate a step within a path
 * JavaScript: jsonata.js line 242
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @param {boolean} lastStep - flag the last step in a path
 * @returns {*} Evaluated input data
 */
func evaluateStep(expr *ASTNode, input *JSONataArray, environment *Frame, lastStep bool) (*JSONataArray, error) {
	// Check timeout at the beginning of step evaluation
	if err := checkTimeout(environment); err != nil {
		return nil, err
	}

	var result *JSONataArray
	if expr.Type == "sort" {
		sorted, err := evaluateSortExpression(expr, input, environment)
		if err != nil {
			return nil, err
		}
		result = createSequence()
		if sorted != nil {
			switch v := sorted.(type) {
			case *JSONataArray:
				result = v
			case []interface{}:
				// sortFunc returns []interface{}, convert to *JSONataArray
				for _, item := range v {
					result.Push(item)
				}
			default:
				result.Push(sorted)
			}
		}
		if expr.Stages != nil {
			stagesResult, err := evaluateStages(expr.Stages, result, environment)
			if err != nil {
				return nil, err
			}
			if arr, ok := stagesResult.(*JSONataArray); ok {
				result = arr
			}
		}
		return result, nil
	}

	result = createSequence()

	for ii := 0; ii < input.Length(); ii++ {
		// Check timeout in loop
		if err := checkTimeout(environment); err != nil {
			return nil, err
		}
		res, err := evaluate(expr, input.Get(ii), environment)
		if err != nil {
			return nil, err
		}
		if expr.Stages != nil {
			for ss := 0; ss < len(expr.Stages); ss++ {
				res, err = evaluateFilter(expr.Stages[ss].Expr, res, environment)
				if err != nil {
					return nil, err
				}
			}
		}
		if res != nil {
			result.Push(res)
		}
	}

	/*
	 * TRANSLITERATION FROM: JavaScript jsonata.js lines 266-282 in evaluateStep function:
	 *
	 * var resultSequence = createSequence();
	 * if(lastStep && result.length === 1 && Array.isArray(result[0]) && !isSequence(result[0])) {
	 *     resultSequence = result[0];
	 * } else {
	 *     // flatten the sequence
	 *     result.forEach(function(res) {
	 *         if (!Array.isArray(res) || res.cons) {
	 *             // it's not an array - just push into the result sequence
	 *             resultSequence.push(res);
	 *         } else {
	 *             // res is a sequence - flatten it into the parent sequence
	 *             res.forEach(val => resultSequence.push(val));
	 *         }
	 *     });
	 * }
	 */
	resultSequence := createSequence()

	// JavaScript: if(lastStep && result.length === 1 && Array.isArray(result[0]) && !isSequence(result[0]))
	if lastStep && result.Length() == 1 {
		res0 := result.Get(0)
		if arr, ok := res0.([]interface{}); ok && !isSequence(res0) {
			// DEVIATION FROM JAVASCRIPT: Additional check for constructed arrays
			// JavaScript doesn't need this because constructed arrays retain .cons property
			if jsonataArr, isJSONataArr := res0.(*JSONataArray); isJSONataArr && jsonataArr.Cons != nil && *jsonataArr.Cons {
				// This is a constructed array - preserve it as a single element
				resultSequence.Push(res0)
				return resultSequence, nil
			}
			// JavaScript: resultSequence = result[0];
			// TRANSLITERATION FIX: In JavaScript, this assigns the plain array directly to resultSequence
			// In our case, we create a sequence from the array elements but mark it to keep singleton
			resultSequence = createSequence()
			for _, item := range arr {
				resultSequence.Push(item)
			}
			// CRITICAL: Mark to keep singleton so sequence processing preserves the array
			resultSequence.SetKeepSingleton(true)
			return resultSequence, nil
		}
	}

	/*
	 * TRANSLITERATION FROM: JavaScript jsonata.js lines 271-279:
	 *
	 * result.forEach(function(res) {
	 *     if (!Array.isArray(res) || res.cons) {
	 *         // it's not an array - just push into the result sequence
	 *         resultSequence.push(res);
	 *     } else {
	 *         // res is a sequence - flatten it into the parent sequence
	 *         res.forEach(val => resultSequence.push(val));
	 *     }
	 * });
	 */
	for i := 0; i < result.Length(); i++ {
		// Check timeout in loop
		if err := checkTimeout(environment); err != nil {
			return nil, err
		}
		res := result.Get(i)

		// JavaScript: if (!Array.isArray(res) || res.cons)
		if jsonataArr, ok := res.(*JSONataArray); ok {
			// It's a JSONataArray - check if it's a constructed array (res.cons)
			if jsonataArr.Cons != nil && *jsonataArr.Cons {
				// JavaScript: res.cons === true case - preserve it
				resultSequence.Push(res)
			} else if jsonataArr.IsSequence() {
				// JavaScript: Array.isArray(res) && !res.cons case - flatten it
				for j := 0; j < jsonataArr.Length(); j++ {
					resultSequence.Push(jsonataArr.Get(j))
				}
			} else {
				// JavaScript: !Array.isArray(res) case - preserve it
				resultSequence.Push(res)
			}
		} else if arr, ok := res.([]interface{}); ok {
			// DEVIATION FROM JAVASCRIPT: Go-specific handling for []interface{} slices
			// JavaScript would have preserved .cons property, but Go loses it during conversion
			// We use context clues (expr.ConsArray, lastStep) to avoid inappropriate flattening
			if expr.ConsArray {
				// Conservative: Don't flatten in array constructor contexts
				resultSequence.Push(res)
			} else {
				// JavaScript equivalent: Array.isArray(res) && !res.cons - flatten it
				for _, val := range arr {
					resultSequence.Push(val)
				}
			}
		} else {
			// JavaScript: !Array.isArray(res) case - just push as-is
			resultSequence.Push(res)
		}
	}

	return resultSequence, nil
}

func evaluateStages(stages []*Stage, input interface{}, environment *Frame) (interface{}, error) {
	result := input
	for ss := 0; ss < len(stages); ss++ {
		stage := stages[ss]
		switch stage.Type {
		case "filter":
			var err error
			result, err = evaluateFilter(stage.Expr, result, environment)
			if err != nil {
				return nil, err
			}
		case "index":
			if seq, ok := result.(*Sequence); ok {
				for ee := 0; ee < seq.length(); ee++ {
					if tuple, ok := seq.Data[ee].(map[string]interface{}); ok {
						tuple[stage.Value] = float64(ee)
					}
				}
			}
		}
	}
	return result, nil
}

/**
 * Evaluate a step within a path
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} tupleBindings - The tuple stream
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateTupleStep(expr *ASTNode, input *JSONataArray, tupleBindings *JSONataArray, environment *Frame) (*JSONataArray, error) {
	var result *JSONataArray
	if expr.Type == "sort" {
		if tupleBindings != nil {
			sorted, err := evaluateSortExpression(expr, tupleBindings, environment)
			if err != nil {
				return nil, err
			}
			switch v := sorted.(type) {
			case *JSONataArray:
				result = v
			case []interface{}:
				// sortFunc returns []interface{}, need to convert to sequence
				result = createSequence()
				result.SetTupleStream(true)
				for _, item := range v {
					result.Push(item)
				}
			}
		} else {
			sorted, err := evaluateSortExpression(expr, input, environment)
			if err != nil {
				return nil, err
			}
			result = createSequence()
			result.SetTupleStream(true)
			switch v := sorted.(type) {
			case *JSONataArray:
				for ss := 0; ss < v.Length(); ss++ {
					tuple := map[string]interface{}{"@": v.Get(ss)}
					if expr.Index != "" {
						tuple[expr.Index] = float64(ss)
					}
					result.Push(tuple)
				}
			case []interface{}:
				// sortFunc returns []interface{}
				for ss, item := range v {
					tuple := map[string]interface{}{"@": item}
					if expr.Index != "" {
						tuple[expr.Index] = float64(ss)
					}
					result.Push(tuple)
				}
			}
		}
		if expr.Stages != nil {
			stagesResult, err := evaluateStages(expr.Stages, result, environment)
			if err != nil {
				return nil, err
			}
			if arr, ok := stagesResult.(*JSONataArray); ok {
				result = arr
			}
		}
		return result, nil
	} else {
		// Non-sort tuple processing
		result = createSequence()
		result.SetTupleStream(true)
		stepEnv := environment
		if tupleBindings == nil {
			tupleBindings = createSequence()
			if input != nil {
				for i := 0; i < input.Length(); i++ {
					tupleBindings.Push(map[string]interface{}{"@": input.Get(i)})
				}
			}
		}

		for ee := 0; ee < tupleBindings.Length(); ee++ {
			tupleBind := tupleBindings.Get(ee).(map[string]interface{})
			stepEnv = createFrameFromTuple(environment, tupleBind)
			res, err := evaluate(expr, tupleBind["@"], stepEnv)
			if err != nil {
				return nil, err
			}
			// res is the binding sequence for the output tuple stream
			if res != nil {
				resArr := []interface{}{res}
				if arr, ok := res.([]interface{}); ok {
					resArr = arr
				} else if seq, ok := res.(*JSONataArray); ok {
					resArr = []interface{}{}
					for i := 0; i < seq.Length(); i++ {
						resArr = append(resArr, seq.Get(i))
					}
				}
				for bb := 0; bb < len(resArr); bb++ {
					tuple := make(map[string]interface{})
					// Copy existing tuple bindings
					for k, v := range tupleBind {
						tuple[k] = v
					}
					if resSeq, ok := res.(*JSONataArray); ok && resSeq.IsTupleStream() {
						// Merge tuple stream results
						if resTuple, ok := resArr[bb].(map[string]interface{}); ok {
							for k, v := range resTuple {
								tuple[k] = v
							}
						}
					} else {
						if expr.Focus != "" {
							tuple[expr.Focus] = resArr[bb]
							tuple["@"] = tupleBind["@"]
						} else {
							tuple["@"] = resArr[bb]
						}
						if expr.Index != "" {
							tuple[expr.Index] = float64(bb)
						}
						if expr.Ancestor != nil {
							if Debug {
								fmt.Printf("DEBUG-GO: binding ancestor label '%s' to value: %v\n", expr.Ancestor.Label, tupleBind["@"])
							}
							tuple[expr.Ancestor.Label] = tupleBind["@"]
						}
					}
					result.Push(tuple)
				}
			}
		}

		if expr.Stages != nil {
			stagesResult, err := evaluateStages(expr.Stages, result, environment)
			if err != nil {
				return nil, err
			}
			if seq, ok := stagesResult.(*JSONataArray); ok {
				result = seq
			}
		}

		return result, nil
	}
}

/**
 * Apply filter predicate to input data
 * JavaScript: jsonata.js line 386
 * @param {Object} predicate - filter expression
 * @param {Object} input - Input data to apply predicates against
 * @param {Object} environment - Environment
 * @returns {*} Result after applying predicates
 */
func evaluateFilter(predicate interface{}, input interface{}, environment *Frame) (interface{}, error) {
	// Check timeout at the beginning of filter evaluation
	if err := checkTimeout(environment); err != nil {
		return nil, err
	}

	results := createSequence()
	if arr, ok := input.(*JSONataArray); ok && arr.IsTupleStream() {
		results.SetTupleStream(true)
	}

	var inputSeq *JSONataArray
	if arr, ok := input.(*JSONataArray); ok {
		// Input is already a JSONataArray (sequence), use it directly
		inputSeq = arr
	} else if arr, ok := input.([]interface{}); ok {
		// JavaScript: Array.isArray(input) === true, so input is used directly
		// Convert Go slice to JSONataArray while preserving array nature (not a sequence)
		falseVal := false
		inputSeq = &JSONataArray{Data: arr, Sequence: &falseVal}
	} else {
		// JavaScript: !Array.isArray(input), so input = createSequence(input)
		// But if input is nil (undefined), create empty sequence
		if input == nil {
			inputSeq = createSequence()
		} else {
			inputSeq = createSequence(input)
		}
	}

	// Check if predicate is an array constructor (e.g., [0..2])
	if predNode, ok := predicate.(*ASTNode); ok && predNode.Type == "unary" && predNode.Value == "[" {
		// Evaluate the array constructor first
		predResult, err := evaluate(predNode, nil, environment)
		if err != nil {
			return nil, err
		}
		// Use the result as indices
		if isArrayOfNumbers(predResult) {
			if predArr, ok := predResult.([]interface{}); ok {
				// Create a map of indices for O(1) lookup
				indexMap := make(map[int]bool)
				for _, v := range predArr {
					if num, ok := v.(float64); ok {
						index := int(math.Floor(num))
						if index < 0 {
							// negative indices count from end
							index = inputSeq.Length() + index
						}
						indexMap[index] = true
					}
				}

				// Now iterate through input array in order
				for i := 0; i < inputSeq.Length(); i++ {
					if indexMap[i] {
						item := inputSeq.Get(i)
						results.Push(item)
					}
				}
			}
			return results, nil
		}
	}

	if predNode, ok := predicate.(*ASTNode); ok && predNode.Type == "number" {
		index := int(math.Floor(predNode.Value.(float64))) // round it down
		if index < 0 {
			// count in from end of array
			index = inputSeq.Length() + index
		}
		if index >= 0 && index < inputSeq.Length() {
			item := inputSeq.Get(index)
			if arr, ok := item.([]interface{}); ok {
				for _, v := range arr {
					results.Push(v)
				}
			} else {
				results.Push(item)
			}
		}
	} else if isArrayOfNumbers(predicate) {
		// If predicate is already an array of numbers, use them as indices
		// JavaScript iterates through input array and checks if each index is in predicate array
		// This preserves the order of items as they appear in the input array
		if predArr, ok := predicate.([]interface{}); ok {
			// Create a map of indices for O(1) lookup
			indexMap := make(map[int]bool)
			for _, v := range predArr {
				if num, ok := v.(float64); ok {
					index := int(math.Floor(num))
					if index < 0 {
						// negative indices count from end
						index = inputSeq.Length() + index
					}
					indexMap[index] = true
				}
			}

			// Now iterate through input array in order
			for i := 0; i < inputSeq.Length(); i++ {
				if indexMap[i] {
					item := inputSeq.Get(i)
					if arr, ok := item.([]interface{}); ok {
						for _, v := range arr {
							results.Push(v)
						}
					} else {
						results.Push(item)
					}
				}
			}
		}
	} else {
		for index := 0; index < inputSeq.Length(); index++ {
			// Check timeout in loop
			if err := checkTimeout(environment); err != nil {
				return nil, err
			}
			item := inputSeq.Get(index)
			context := item
			env := environment
			if inputSeq.IsTupleStream() {
				if tuple, ok := item.(map[string]interface{}); ok {
					context = tuple["@"]
					env = createFrameFromTuple(environment, tuple)
				}
			}
			res, err := evaluate(predicate, context, env)
			if err != nil {
				return nil, err
			}
			numeric, err := isNumeric(res)
			if err != nil {
				return nil, err
			}
			if numeric {
				resArr := []float64{}
				if isArrayOfNumbers(res) {
					// res is []interface{} containing numbers, not []float64
					if arr, ok := res.([]interface{}); ok {
						for _, v := range arr {
							if num, ok := v.(float64); ok {
								resArr = append(resArr, num)
							}
						}
					}
				} else {
					resArr = []float64{res.(float64)}
				}
				for _, ires := range resArr {
					// round it down
					ii := int(math.Floor(ires))
					if ii < 0 {
						// count in from end of array
						ii = inputSeq.Length() + ii
					}
					if ii == index {
						results.Push(item)
					}
				}
			} else {
				boolRes, err := functions.booleanFunc([]interface{}{res})
				if err != nil {
					return nil, err
				}
				if boolRes != nil && boolRes.(bool) { // truthy
					results.Push(item)
				}
			}
		}
	}

	return results, nil
}

/**
 * Evaluate binary expression against input data
 * JavaScript: jsonata.js line 448
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateBinary(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	var result interface{}
	lhs, err := evaluate(expr.LHS, input, environment)
	if err != nil {
		return nil, err
	}
	op := expr.Value.(string)

	// defer evaluation of RHS to allow short-circuiting
	evalrhs := func() (interface{}, error) {
		return evaluate(expr.RHS, input, environment)
	}

	if op == "and" || op == "or" {
		result, err = evaluateBooleanExpression(lhs, evalrhs, op)
		if err != nil {
			if jsonErr, ok := err.(*JSONataError); ok {
				jsonErr.Position = expr.Position
				jsonErr.Token = op
			}
			return nil, err
		}
		return result, nil
	}

	rhs, rhsErr := evalrhs()
	if rhsErr != nil {
		return nil, rhsErr
	}
	switch op {
	case "+", "-", "*", "/", "%":
		result, err = evaluateNumericExpression(lhs, rhs, op)
	case "=", "!=":
		result = evaluateEqualityExpression(lhs, rhs, op)
	case "<", "<=", ">", ">=":
		result, err = evaluateComparisonExpression(lhs, rhs, op)
	case "&":
		result, err = evaluateStringConcat(lhs, rhs)
	case "..":
		result, err = evaluateRangeExpression(lhs, rhs, environment)
	case "in":
		result = evaluateIncludesExpression(lhs, rhs)
	}

	if err != nil {
		if jsonErr, ok := err.(*JSONataError); ok {
			jsonErr.Position = expr.Position
			jsonErr.Token = op
		}
		return nil, err
	}
	return result, nil
}

/**
 * Evaluate unary expression against input data
 * JavaScript: jsonata.js line 510
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateUnary(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	var result interface{}

	switch expr.Value.(string) {
	case "-":
		res, err := evaluate(expr.Expression, input, environment)
		if err != nil {
			return nil, err
		}
		if res == nil {
			result = nil
		} else {
			numeric, err := isNumeric(res)
			if err != nil {
				return nil, err
			}
			if numeric {
				result = -res.(float64)
			} else {
				return nil, &JSONataError{
					Code:     "D1002",
					Stack:    getStack(),
					Position: expr.Position,
					Token:    expr.Value.(string),
					Value:    res,
				}
			}
		}
	case "[":
		// array constructor - evaluate each item
		// JavaScript: case '[': result = []; ... if(expr.Consarray) { Object.defineProperty(result, 'cons', {value: true}); }
		resultArr := []interface{}{}
		for idx, item := range expr.Expressions {
			environment.isParallelCall = idx > 0
			value, err := evaluate(item, input, environment)
			if err != nil {
				return nil, err
			}
			if value != nil {
				if item.Value == "[" {
					resultArr = append(resultArr, value)
				} else {
					resultArr = appendToArray(resultArr, value)
				}
			}
		}

		// In JavaScript, array constructors return plain arrays, NOT sequences
		// They only get the 'cons' property if consarray is true

		/*
		 * TRANSLITERATION FROM: JavaScript jsonata.js lines 548-554:
		 *
		 * if(expr.consarray) {
		 *     Object.defineProperty(result, 'cons', {
		 *         enumerable: false,
		 *         configurable: false,
		 *         value: true
		 *     });
		 * }
		 */
		if expr.ConsArray {
			// JavaScript: Object.defineProperty(result, 'cons', {value: true});
			// In Go, we need to wrap in JSONataArray to add the cons property
			trueVal := true
			arrayWrapper := &JSONataArray{
				Data: resultArr,
				Cons: &trueVal,
			}
			result = arrayWrapper
		} else {
			// Plain array, no wrapping needed - matches JavaScript behavior
			result = resultArr
		}
	case "{":
		// object constructor - apply grouping
		var err error
		// Create a Group structure from the LHSPairs for unary object constructors
		group := &Group{
			LHS:                 expr.LHSPairs,
			Position:            expr.Position,
			IsObjectConstructor: true,
		}
		result, err = evaluateGroupExpression(group, input, environment)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

/**
 * Evaluate name object against input data
 * JavaScript: jsonata.js line 572
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateName(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	// lookup the 'name' item in the input
	result := lookupProperty(input, expr.Value.(string))
	return result, nil
}

/**
 * Convert nil values in unmarshaled JSON to JSONataNull
 * This allows us to distinguish between JavaScript null and undefined
 * @param {interface{}} data - The unmarshaled JSON data
 * @returns {interface{}} - Data with nils converted to JSONataNull
 */
func convertNullsToJSONataNull(data interface{}) interface{} {
	if data == nil {
		return JSONNull
	}

	switch v := data.(type) {
	case map[string]interface{}:
		// Convert nulls in object
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = convertNullsToJSONataNull(value)
		}
		return result
	case []interface{}:
		// Convert nulls in array
		result := make([]interface{}, len(v))
		for i, value := range v {
			result[i] = convertNullsToJSONataNull(value)
		}
		return result
	default:
		// Primitive values - return as-is
		return data
	}
}

/**
 * lookupProperty function - direct transliteration of JavaScript lookup function
 * JavaScript: functions.js line 1674-1693
 * This is the internal lookup function used for property access, not the $lookup JSONata function
 * @param {interface{}} input - input data
 * @param {string} key - property key to look up
 * @returns {interface{}} - looked up value
 */
func lookupProperty(input interface{}, key string) interface{} {
	var result interface{}

	// Handle JSONataArray by treating it like a regular array
	if jsonArr, ok := input.(*JSONataArray); ok {
		resultSeq := createSequence()
		for i := 0; i < jsonArr.Length(); i++ {
			res := lookupProperty(jsonArr.Get(i), key)
			if res != nil {
				// Direct transliteration of result.push logic
				if resArr, ok := res.([]interface{}); ok {
					for _, val := range resArr {
						resultSeq.Push(val)
					}
				} else if jsonResArr, ok := res.(*JSONataArray); ok {
					for j := 0; j < jsonResArr.Length(); j++ {
						resultSeq.Push(jsonResArr.Get(j))
					}
				} else {
					resultSeq.Push(res)
				}
			}
		}
		if resultSeq.Length() == 1 {
			// Single result - return the value directly
			result = resultSeq.Get(0)
		} else if resultSeq.Length() > 1 {
			// Multiple results - return as array
			result = resultSeq.Data
		}
	} else if arr, ok := input.([]interface{}); ok {
		// Handle arrays by recursively calling lookup on each element
		// Direct transliteration of: if (Array.isArray(input))
		resultSeq := createSequence()
		for _, item := range arr {
			res := lookupProperty(item, key)
			if res != nil {
				// Direct transliteration of result.push logic
				if resArr, ok := res.([]interface{}); ok {
					for _, val := range resArr {
						resultSeq.Push(val)
					}
				} else {
					resultSeq.Push(res)
				}
			}
		}
		if resultSeq.Length() == 1 {
			// Single result - return the value directly
			result = resultSeq.Get(0)
		} else if resultSeq.Length() > 1 {
			// Multiple results - return as array
			result = resultSeq.Data
		}
	} else if objMap, ok := input.(map[string]interface{}); ok {
		// Handle objects - direct transliteration of: input[key]
		result = objMap[key]
	}
	// Note: JavaScript checks !isFunction(input) but Go doesn't have functions as objects

	return result
}

/**
 * Evaluate literal against input data
 * JavaScript: jsonata.js line 582
 * @param {Object} expr - JSONata expression
 * @returns {*} Evaluated input data
 */
func evaluateLiteral(expr *ASTNode) (interface{}, error) {
	return expr.Value, nil
}

/**
 * Evaluate wildcard against input data
 * JavaScript: jsonata.js line 592
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @returns {*} Evaluated input data
 */
func evaluateWildcard(expr *ASTNode, input interface{}) (interface{}, error) {
	results := createSequence()

	// Handle outer wrapper - when input is a wrapped array with outerWrapper=true
	// In JavaScript: if (Array.isArray(input) && input.outerWrapper && input.length > 0)
	// The wrapped sequence IS the array itself, not a container of the array
	if seq, ok := input.(*Sequence); ok && seq.IsOuterWrapper() {
		// The sequence contains the array elements directly
		// So treat the sequence as an array for wildcard purposes
		for i := 0; i < seq.length(); i++ {
			value := seq.Data[i]
			if arr, ok := value.([]interface{}); ok {
				value = flatten(arr, nil)
				results = appendToSequence(results, value)
			} else {
				results.push(value)
			}
		}
		return results, nil
	}

	// JavaScript treats arrays as objects, so wildcard on array returns its values
	if input != nil {
		switch v := input.(type) {
		case map[string]interface{}:
			// Object - iterate over values
			for _, value := range v {
				if arr, ok := value.([]interface{}); ok {
					value = flatten(arr, nil)
					results = appendToSequence(results, value)
				} else {
					results.push(value)
				}
			}
		case []interface{}:
			// Array - in JavaScript, arrays are objects and Object.keys() returns indices
			// So wildcard on array returns the values at those indices
			for _, value := range v {
				if arr, ok := value.([]interface{}); ok {
					value = flatten(arr, nil)
					results = appendToSequence(results, value)
				} else {
					results.push(value)
				}
			}
		case *Sequence:
			// Handle case where input is a Sequence (but not outerWrapper)
			for i := 0; i < v.length(); i++ {
				value := v.Data[i]
				if arr, ok := value.([]interface{}); ok {
					value = flatten(arr, nil)
					results = appendToSequence(results, value)
				} else {
					results.push(value)
				}
			}
		}
	}

	return results, nil
}

/**
 * Returns a flattened array
 * @param {Array} arg - the array to be flatten
 * @param {Array} flattened - carries the flattened array - if not defined, will initialize to []
 * @returns {Array} - the flattened array
 */
func flatten(arg interface{}, flattened []interface{}) []interface{} {
	if flattened == nil {
		flattened = []interface{}{}
	}
	if arr, ok := arg.([]interface{}); ok {
		for _, item := range arr {
			flattened = flatten(item, flattened)
		}
	} else {
		flattened = append(flattened, arg)
	}
	return flattened
}

/**
 * Evaluate descendants against input data
 * JavaScript: jsonata.js line 639
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @returns {*} Evaluated input data
 */
func evaluateDescendants(expr *ASTNode, input interface{}) (interface{}, error) {
	var result interface{}
	resultSequence := createSequence()
	if input != nil {
		// traverse all descendants of this object/array
		recurseDescendants(input, resultSequence)
		if resultSequence.length() == 1 {
			result = resultSequence.Data[0]
		} else {
			result = resultSequence
		}
	}
	return result, nil
}

/**
 * Recurse through descendants
 * @param {Object} input - Input data
 * @param {Object} results - Results
 */
func recurseDescendants(input interface{}, results *Sequence) {
	// this is the equivalent of //* in XPath
	if _, ok := input.([]interface{}); !ok {
		results.push(input)
	}
	switch v := input.(type) {
	case []interface{}:
		for _, member := range v {
			recurseDescendants(member, results)
		}
	case map[string]interface{}:
		for _, value := range v {
			recurseDescendants(value, results)
		}
	}
}

/**
 * Evaluate numeric expression against input data
 * @param {Object} lhs - LHS value
 * @param {Object} rhs - RHS value
 * @param {Object} op - opcode
 * @returns {*} Result
 */
func evaluateNumericExpression(lhs, rhs interface{}, op string) (interface{}, error) {
	if lhs != nil {
		numeric, err := isNumeric(lhs)
		if err != nil {
			return nil, err
		}
		if !numeric {
			return nil, &JSONataError{
				Code:  "T2001",
				Stack: getStack(),
				Value: lhs,
			}
		}
	}
	if rhs != nil {
		numeric, err := isNumeric(rhs)
		if err != nil {
			return nil, err
		}
		if !numeric {
			return nil, &JSONataError{
				Code:  "T2002",
				Stack: getStack(),
				Value: rhs,
			}
		}
	}

	if lhs == nil || rhs == nil {
		// if either side is undefined, the result is undefined
		return nil, nil
	}

	// Convert to float64 for calculations
	var lhsNum, rhsNum float64
	switch v := lhs.(type) {
	case float64:
		lhsNum = v
	case int:
		lhsNum = float64(v)
	}
	switch v := rhs.(type) {
	case float64:
		rhsNum = v
	case int:
		rhsNum = float64(v)
	}
	var result float64

	switch op {
	case "+":
		result = lhsNum + rhsNum
	case "-":
		result = lhsNum - rhsNum
	case "*":
		result = lhsNum * rhsNum
	case "/":
		result = lhsNum / rhsNum
	case "%":
		result = math.Mod(lhsNum, rhsNum)
	}
	return result, nil
}

/**
 * Evaluate equality expression against input data
 * @param {Object} lhs - LHS value
 * @param {Object} rhs - RHS value
 * @param {Object} op - opcode
 * @returns {*} Result
 */
func evaluateEqualityExpression(lhs, rhs interface{}, op string) interface{} {
	// Handle null/undefined cases properly
	lhsIsNull := isJSONataNull(lhs)
	rhsIsNull := isJSONataNull(rhs)
	lhsIsUndefined := lhs == nil
	rhsIsUndefined := rhs == nil

	// Handle all combinations according to JavaScript semantics
	if lhsIsNull && rhsIsNull {
		// null = null is true
		switch op {
		case "=":
			return true
		case "!=":
			return false
		}
	}

	if lhsIsUndefined && rhsIsUndefined {
		// undefined = undefined is false
		switch op {
		case "=":
			return false
		case "!=":
			return true
		}
	}

	if (lhsIsNull || lhsIsUndefined) && (rhsIsNull || rhsIsUndefined) {
		// null = undefined or undefined = null is false
		switch op {
		case "=":
			return false
		case "!=":
			return true
		}
	}

	if lhsIsNull || rhsIsNull || lhsIsUndefined || rhsIsUndefined {
		// one side is null/undefined, the other is a value
		switch op {
		case "=":
			return false
		case "!=":
			return true
		}
	}

	var result bool
	switch op {
	case "=":
		result = isDeepEqual(lhs, rhs)
	case "!=":
		result = !isDeepEqual(lhs, rhs)
	}
	return result
}

/**
 * Evaluate comparison expression against input data
 * @param {Object} lhs - LHS value
 * @param {Object} rhs - RHS value
 * @param {Object} op - opcode
 * @returns {*} Result
 */
func evaluateComparisonExpression(lhs, rhs interface{}, op string) (interface{}, error) {
	// type checks using efficient type assertions instead of reflection
	lNumeric, err := isNumeric(lhs)
	if err != nil {
		return nil, err
	}
	rNumeric, err := isNumeric(rhs)
	if err != nil {
		return nil, err
	}

	// Check if values are comparable (string, number, or undefined)
	// In JavaScript, undefined is considered "comparable" but returns undefined
	lhsIsUndefined := lhs == nil
	rhsIsUndefined := rhs == nil

	lcomparable := isString(lhs) || lNumeric || lhsIsUndefined
	rcomparable := isString(rhs) || rNumeric || rhsIsUndefined

	// if either value is not comparable, throw an error
	if !lcomparable || !rcomparable {
		// JSONataNull is specifically not comparable
		if isJSONataNull(lhs) || isJSONataNull(rhs) {
			var badValue interface{}
			if isJSONataNull(lhs) {
				badValue = lhs
			} else {
				badValue = rhs
			}
			return nil, &JSONataError{
				Code:  "T2010",
				Stack: getStack(),
				Value: badValue,
			}
		}
		// Other non-comparable values (e.g., boolean, object, array)
		var badValue interface{}
		if !lcomparable {
			badValue = lhs
		} else {
			badValue = rhs
		}
		return nil, &JSONataError{
			Code:  "T2010",
			Stack: getStack(),
			Value: badValue,
		}
	}

	// If either operand is undefined, return undefined
	if lhsIsUndefined || rhsIsUndefined {
		return nil, nil
	}

	// if aa and bb are not of the same type (using type assertions instead of reflection)
	lTypeName := getTypeName(lhs)
	rTypeName := getTypeName(rhs)
	if lTypeName != rTypeName {
		return nil, &JSONataError{
			Code:   "T2009",
			Stack:  getStack(),
			Value:  lhs,
			Value2: rhs,
		}
	}

	var result bool
	switch lhs.(type) {
	case string:
		lstr := lhs.(string)
		rstr := rhs.(string)
		switch op {
		case "<":
			result = lstr < rstr
		case "<=":
			result = lstr <= rstr
		case ">":
			result = lstr > rstr
		case ">=":
			result = lstr >= rstr
		}
	case float64:
		lnum := lhs.(float64)
		rnum := rhs.(float64)
		switch op {
		case "<":
			result = lnum < rnum
		case "<=":
			result = lnum <= rnum
		case ">":
			result = lnum > rnum
		case ">=":
			result = lnum >= rnum
		}
	}
	return result, nil
}

/**
 * Inclusion operator - in
 *
 * @param {Object} lhs - LHS value
 * @param {Object} rhs - RHS value
 * @returns {boolean} - true if lhs is a member of rhs
 */
func evaluateIncludesExpression(lhs, rhs interface{}) interface{} {
	result := false

	if lhs == nil || rhs == nil {
		// if either side is undefined, the result is false
		return false
	}

	var rhsArr []interface{}
	if arr, ok := rhs.([]interface{}); ok {
		rhsArr = arr
	} else {
		rhsArr = []interface{}{rhs}
	}

	for i := 0; i < len(rhsArr); i++ {
		if rhsArr[i] == lhs {
			result = true
			break
		}
	}

	return result
}

/**
 * Evaluate boolean expression against input data
 * @param {Object} lhs - LHS value
 * @param {Function} evalrhs - function to evaluate RHS value
 * @param {Object} op - opcode
 * @returns {*} Result
 */
func evaluateBooleanExpression(lhs interface{}, evalrhs func() (interface{}, error), op string) (interface{}, error) {
	var result bool

	lBool := boolize(lhs)

	switch op {
	case "and":
		if lBool {
			rhs, err := evalrhs()
			if err != nil {
				return nil, err
			}
			result = lBool && boolize(rhs)
		} else {
			result = false
		}
	case "or":
		if !lBool {
			rhs, err := evalrhs()
			if err != nil {
				return nil, err
			}
			result = lBool || boolize(rhs)
		} else {
			result = true
		}
	}
	return result, nil
}

func boolize(value interface{}) bool {
	booledValue, err := toBoolean(value)
	if err != nil {
		// fallback to false for errors in boolize
		return false
	}
	return booledValue
}

// toBoolean - JavaScript boolean conversion equivalent
func toBoolean(value interface{}) (bool, error) {
	if functions != nil && functions.booleanFunc != nil {
		result, err := functions.booleanFunc([]interface{}{value})
		if err != nil {
			return false, err
		}
		if result == nil {
			return false, nil
		}
		if b, ok := result.(bool); ok {
			return b, nil
		}
	}
	return false, nil
}

// convertToString converts a value to string following JavaScript JSONata rules
// This mirrors the behavior of the JavaScript string() function
func convertToString(arg interface{}) (string, error) {
	if arg == nil {
		return "null", nil
	}

	switch v := arg.(type) {
	case string:
		return v, nil
	case float64:
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return "", &JSONataError{
				Code:  "D3001",
				Value: arg,
				Stack: getStack(),
			}
		}
		// JavaScript uses toPrecision(15) for numbers in JSON.stringify
		// Format with 'g' and precision 15 to match JavaScript behavior
		str := strconv.FormatFloat(v, 'g', 15, 64)
		return str, nil
	case int:
		return strconv.Itoa(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		if isFunction(arg) {
			// functions (built-in and lambda) convert to empty string
			return "", nil
		}
		// Convert to JSON
		bytes, err := json.Marshal(arg)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
}

/**
 * Evaluate string concatenation against input data
 * @param {Object} lhs - LHS value
 * @param {Object} rhs - RHS value
 * @returns {string|*} Concatenated string
 */
func evaluateStringConcat(lhs, rhs interface{}) (interface{}, error) {
	lstr := ""
	rstr := ""
	if lhs != nil {
		var err error
		lstr, err = convertToString(lhs)
		if err != nil {
			return nil, err
		}
	}
	if rhs != nil {
		var err error
		rstr, err = convertToString(rhs)
		if err != nil {
			return nil, err
		}
	}

	result := lstr + rstr
	return result, nil
}

/**
 * Evaluate group expression against input data
 * JavaScript: jsonata.js line 899
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {{}} Evaluated input data
 */
func evaluateGroupExpression(expr *Group, input interface{}, environment *Frame) (interface{}, error) {
	// Check timeout at the beginning of group expression evaluation
	if err := checkTimeout(environment); err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	groups := make(map[string]*groupEntry)
	reduce := false
	if seq, ok := input.(*JSONataArray); ok && seq.IsTupleStream() {
		reduce = true
	}

	// Convert input to sequence-like structure for iteration
	var inputSeq *JSONataArray
	if seq, ok := input.(*JSONataArray); ok {
		inputSeq = seq
	} else if arr, ok := input.([]interface{}); ok {
		// Arrays should be used directly, not converted to sequences
		// This matches JavaScript behavior where arrays are iterated directly
		inputSeq = &JSONataArray{Data: arr}
	} else {
		inputSeq = createSequence()
		if input != nil {
			inputSeq.Push(input)
		}
	}

	// if the array is empty, add an undefined entry to enable literal JSON object to be generated
	if inputSeq.Length() == 0 {
		inputSeq.Push(nil)
	}

	for itemIndex := 0; itemIndex < inputSeq.Length(); itemIndex++ {
		item := inputSeq.Data[itemIndex]
		env := environment
		if reduce {
			if tuple, ok := item.(map[string]interface{}); ok {
				env = createFrameFromTuple(environment, tuple)
			}
		}
		for pairIndex := 0; pairIndex < len(expr.LHS); pairIndex++ {
			// Check timeout in inner loop
			if err := checkTimeout(environment); err != nil {
				return nil, err
			}
			pair := expr.LHS[pairIndex]
			var keyInput interface{}
			if reduce {
				if tuple, ok := item.(map[string]interface{}); ok {
					keyInput = tuple["@"]
				}
			} else {
				keyInput = item
			}

			// Special check for object constructors on arrays
			// In JavaScript, if the key expression evaluates to an array/sequence when applied
			// to an individual item, it throws T1003 immediately
			if expr.IsObjectConstructor && itemIndex == 0 && !reduce {
				// First, check what the key would evaluate to against the entire input array
				testKey, testErr := evaluate(pair[0], input, environment)
				if testErr == nil && testKey != nil {
					// If it's not a string, it means the key expression produces multiple values
					if _, ok := testKey.(string); !ok {
						return nil, &JSONataError{
							Code:     "T1003",
							Stack:    getStack(),
							Position: expr.Position,
							Value:    testKey,
						}
					}
				}
			}

			key, err := evaluate(pair[0], keyInput, env)
			if err != nil {
				return nil, err
			}
			// key has to be a string
			if key != nil {
				if _, ok := key.(string); !ok {
					return nil, &JSONataError{
						Code:     "T1003",
						Stack:    getStack(),
						Position: expr.Position,
						Value:    key,
					}
				}
			}

			if key != nil {
				keyStr := key.(string)
				entry := &groupEntry{Data: item, exprIndex: pairIndex}
				if existing, ok := groups[keyStr]; ok {
					// a value already exists in this slot
					if existing.exprIndex != pairIndex {
						// this key has been generated by another expression in this group
						// when multiple key expressions evaluate to the same key, then error D1009 must be thrown
						return nil, &JSONataError{
							Code:     "D1009",
							Stack:    getStack(),
							Position: expr.Position,
							Value:    keyStr,
						}
					}

					// append it as an array
					if existingArr, ok := existing.Data.([]interface{}); ok {
						groups[keyStr].Data = append(existingArr, item)
					} else {
						groups[keyStr].Data = []interface{}{existing.Data, item}
					}
				} else {
					groups[keyStr] = entry
				}
			}
		}
	}

	// iterate over the groups to evaluate the 'value' expression
	idx := 0
	for key, entry := range groups {
		context := entry.Data
		env := environment
		if reduce {
			tuple := reduceTupleStream(entry.Data)
			if tupleMap, ok := tuple.(map[string]interface{}); ok {
				context = tupleMap["@"]
				delete(tupleMap, "@")
				env = createFrameFromTuple(environment, tupleMap)
			}
		}
		environment.isParallelCall = idx > 0
		value, err := evaluate(expr.LHS[entry.exprIndex][1], context, env)
		if err != nil {
			return nil, err
		}
		if value != nil {
			result[key] = value
		}
		idx++
	}

	return result, nil
}

type groupEntry struct { // JS: no equivalent struct, handled with simple objects
	Data      interface{} // Capitalized to match JSONataArray.Data field naming
	exprIndex int
}

func reduceTupleStream(tupleStream interface{}) interface{} {
	arr, ok := tupleStream.([]interface{})
	if !ok {
		return tupleStream
	}
	if len(arr) == 0 {
		return tupleStream
	}

	result := make(map[string]interface{})
	// Copy first tuple
	if firstTuple, ok := arr[0].(map[string]interface{}); ok {
		for k, v := range firstTuple {
			result[k] = v
		}
	}

	// Merge remaining tuples
	for ii := 1; ii < len(arr); ii++ {
		if tuple, ok := arr[ii].(map[string]interface{}); ok {
			for prop, value := range tuple {
				if existing, exists := result[prop]; exists {
					if existingArr, ok := existing.([]interface{}); ok {
						result[prop] = append(existingArr, value)
					} else {
						result[prop] = []interface{}{existing, value}
					}
				} else {
					result[prop] = value
				}
			}
		}
	}
	return result
}

/**
 * Evaluate range expression against input data
 * @param {Object} lhs - LHS value
 * @param {Object} rhs - RHS value
 * @returns {Array} Resultant array
 */
func evaluateRangeExpression(lhs, rhs interface{}, environment *Frame) (interface{}, error) {
	if lhs != nil && !isInteger(lhs) {
		return nil, &JSONataError{
			Code:  "T2003",
			Stack: getStack(),
			Value: lhs,
		}
	}
	if rhs != nil && !isInteger(rhs) {
		return nil, &JSONataError{
			Code:  "T2004",
			Stack: getStack(),
			Value: rhs,
		}
	}

	if lhs == nil || rhs == nil {
		// if either side is undefined, the result is undefined
		return nil, nil
	}

	lhsInt := int(lhs.(float64))
	rhsInt := int(rhs.(float64))

	if lhsInt > rhsInt {
		// if the lhs is greater than the rhs, return undefined
		return nil, nil
	}

	// Check against configured range limit or default limit
	size := rhsInt - lhsInt + 1
	maxRange := 10000000 // Default limit of 10 million
	if environment != nil && environment.maxRange > 0 {
		maxRange = environment.maxRange
	}
	if size > maxRange {
		return nil, &JSONataError{
			Code:  "D2014",
			Stack: getStack(),
			Value: float64(size),
		}
	}

	result := make([]interface{}, size)
	for item, index := lhsInt, 0; item <= rhsInt; item, index = item+1, index+1 {
		result[index] = float64(item)
	}
	seq := createSequence()
	for _, v := range result {
		seq.push(v)
	}
	seq.SetSequence(true)
	return seq, nil
}

/**
 * Evaluate bind expression against input data
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateBindExpression(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	// The RHS is the expression to evaluate
	// The LHS is the name of the variable to bind to - should be a VARIABLE token (enforced by parser)
	value, err := evaluate(expr.RHS, input, environment)
	if err != nil {
		return nil, err
	}
	environment.bind(expr.LHS.Value.(string), value)
	return value, nil
}

/**
 * Evaluate condition against input data
 * JavaScript: jsonata.js line 1067
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateCondition(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	var result interface{}
	condition, err := evaluate(expr.Condition, input, environment)
	if err != nil {
		return nil, err
	}
	if boolRes, err := toBoolean(condition); err != nil {
		return nil, err
	} else if boolRes {
		result, err = evaluate(expr.Then, input, environment)
		if err != nil {
			return nil, err
		}
	} else if expr.Else != nil {
		result, err = evaluate(expr.Else, input, environment)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

/**
 * Evaluate block against input data
 * JavaScript: jsonata.js line 1085
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateBlock(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	var result interface{}
	// create a new frame to limit the scope of variable assignments
	// TODO, only do this if the post-parse stage has flagged this as required (JavaScript: jsonata.js line 1088)
	frame := createFrame(environment)
	// invoke each expression in turn
	// only return the result of the last one
	for ii := 0; ii < len(expr.Expressions); ii++ {
		var err error
		result, err = evaluate(expr.Expressions[ii], input, frame)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

/**
 * Prepare a regex
 * JavaScript: jsonata.js line 1104
 * @param {Object} expr - expression containing regex
 * @returns {Function} Higher order function representing prepared regex
 */
func evaluateRegex(expr *ASTNode) (interface{}, error) {
	var re *regexp.Regexp
	var reStr string
	var err error

	// Check if expr.Value is already a compiled regexp or a string
	switch v := expr.Value.(type) {
	case *regexp.Regexp:
		re = v
		reStr = re.String()
	case string:
		reStr = v
		re, err = regexp.Compile(v)
		if err != nil {
			return nil, err
		}
	default:
		return nil, &JSONataError{
			Code:  "T0410",
			Stack: getStack(),
			Value: v,
		}
	}
	var closure func(string, int) interface{}
	closure = func(str string, fromIndex int) interface{} {
		searchStr := str
		if fromIndex > 0 && fromIndex < len(str) {
			searchStr = str[fromIndex:]
		}
		match := re.FindStringSubmatchIndex(searchStr)
		if match == nil {
			return nil
		}

		result := map[string]interface{}{
			"match":  searchStr[match[0]:match[1]],
			"start":  match[0] + fromIndex,
			"end":    match[1] + fromIndex,
			"groups": []interface{}{},
		}

		groups := []interface{}{}
		for i := 2; i < len(match); i += 2 {
			if match[i] >= 0 {
				groups = append(groups, searchStr[match[i]:match[i+1]])
			}
		}
		result["groups"] = groups

		nextFunc := func() (interface{}, error) {
			if match[1]+fromIndex >= len(str) {
				return nil, nil
			}
			next := closure(str, match[1]+fromIndex)
			if nextMap, ok := next.(map[string]interface{}); ok && nextMap["match"] == "" {
				// matches zero length string; this will never progress
				return nil, &JSONataError{
					Code:     "D1004",
					Stack:    getStack(),
					Position: expr.Position,
					Value:    reStr,
				}
			}
			return next, nil
		}
		result["next"] = nextFunc

		return result
	}
	return closure, nil
}

/**
 * Evaluate variable against input data
 * JavaScript: jsonata.js line 1153
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateVariable(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	// lookup the variable value in the environment
	var result interface{}
	// if the variable name is empty string, then it refers to context value
	varName := expr.Value.(string)
	if varName == "" {
		if seq, ok := input.(*Sequence); ok && seq.IsOuterWrapper() {
			// Return the original array that was wrapped in the sequence
			result = seq.Data
		} else {
			result = input
		}
	} else {
		result = environment.lookup(varName)
	}
	return result, nil
}

/**
 * sort / order-by operator
 * @param {Object} expr - AST for operator
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Ordered sequence
 */

// sortArray - Helper function to call sortFunc with Focus context
func sortArray(focus *Focus, lhs interface{}, comparator interface{}) (interface{}, error) {
	// Handle both []interface{} and *JSONataArray
	var arr []interface{}
	switch v := lhs.(type) {
	case []interface{}:
		arr = v
	case *JSONataArray:
		arr = v.Data
	default:
		return lhs, nil
	}

	return functions.sortFunc([]interface{}{arr, comparator})
}

func evaluateSortExpression(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	// evaluate the lhs, then sort the results in order according to rhs expression
	lhs := input
	isTupleSort := false
	if seq, ok := input.(*Sequence); ok && seq.IsTupleStream() {
		isTupleSort = true
	}

	// sort the lhs array
	// use comparator function
	comparator := func(a, b interface{}) (bool, error) {
		// expr.terms is an array of order-by in priority order
		comp := 0
		for index := 0; comp == 0 && index < len(expr.Terms); index++ {
			term := expr.Terms[index]
			// evaluate the sort term in the context of a
			context := a
			env := environment
			if isTupleSort {
				if tuple, ok := a.(map[string]interface{}); ok {
					context = tuple["@"]
					env = createFrameFromTuple(environment, tuple)
				}
			}
			aa, err := evaluate(term.Expression, context, env)
			if err != nil {
				return false, err
			}
			// evaluate the sort term in the context of b
			context = b
			env = environment
			if isTupleSort {
				if tuple, ok := b.(map[string]interface{}); ok {
					context = tuple["@"]
					env = createFrameFromTuple(environment, tuple)
				}
			}
			bb, err := evaluate(term.Expression, context, env)
			if err != nil {
				return false, err
			}

			// type checks using efficient type assertions instead of reflection
			// undefined should be last in sort order
			// Note: JSONataNull is NOT nil - it represents JavaScript null, not undefined
			if aa == nil {
				// swap them, unless bb is also undefined
				if bb == nil {
					comp = 0
				} else {
					comp = 1
				}
				continue
			}
			if bb == nil {
				comp = -1
				continue
			}

			// if aa or bb are not string or numeric values, then throw an error
			aaNumeric, err := isNumeric(aa)
			if err != nil {
				return false, err
			}
			bbNumeric, err := isNumeric(bb)
			if err != nil {
				return false, err
			}
			aaOk := isString(aa) || aaNumeric
			bbOk := isString(bb) || bbNumeric
			if !aaOk || !bbOk {
				var badValue interface{}
				if !aaOk {
					badValue = aa
				} else {
					badValue = bb
				}
				return false, &JSONataError{
					Code:     "T2008",
					Stack:    getStack(),
					Position: expr.Position,
					Value:    badValue,
				}
			}

			// if aa and bb are not of the same type (using type assertions instead of reflection)
			aaTypeName := getTypeName(aa)
			bbTypeName := getTypeName(bb)
			if aaTypeName != bbTypeName {
				return false, &JSONataError{
					Code:     "T2007",
					Stack:    getStack(),
					Position: expr.Position,
					Value:    aa,
					Value2:   bb,
				}
			}

			switch aa.(type) {
			case string:
				astr := aa.(string)
				bstr := bb.(string)
				if astr == bstr {
					continue
				} else if astr < bstr {
					comp = -1
				} else {
					comp = 1
				}
			case float64:
				anum := aa.(float64)
				bnum := bb.(float64)
				if anum == bnum {
					continue
				} else if anum < bnum {
					comp = -1
				} else {
					comp = 1
				}
			}

			if term.Descending {
				comp = -comp
			}
		}
		// only swap a & b if comp equals 1
		return comp > 0, nil
	}

	focus := &Focus{
		environment: environment,
		input:       input,
	}
	// the `focus` is passed in as the `this` for the invoked function
	result, err := sortArray(focus, lhs, comparator)
	if err != nil {
		return nil, err
	}

	return result, nil
}

/**
 * create a transformer function
 * @param {Object} expr - AST for operator
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} tranformer function
 */
func evaluateTransformExpression(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	// Check timeout at the beginning of transform expression evaluation
	if err := checkTimeout(environment); err != nil {
		return nil, err
	}

	// create a function to implement the transform definition
	transformer := func(args []interface{}) (interface{}, error) { // signature <(oa):o>
		if len(args) < 1 {
			return nil, errors.New("transform function requires at least 1 argument")
		}
		obj := args[0]
		// undefined inputs always return undefined
		if obj == nil {
			return nil, nil
		}

		// this function returns a copy of obj with changes specified by the pattern/operation
		cloneFunction := environment.lookup("clone")
		if !isFunction(cloneFunction) {
			// throw type error
			return nil, &JSONataError{
				Code:     "T2013",
				Stack:    getStack(),
				Position: expr.Position,
			}
		}
		cloneFn := cloneFunction.(*Function)
		result, err := applyFunction(cloneFn, []interface{}{obj}, nil, environment)
		if err != nil {
			return nil, err
		}
		matches, err := evaluate(expr.Pattern, result, environment)
		if err != nil {
			return nil, err
		}
		if matches != nil {
			var matchArr []interface{}
			if arr, ok := matches.([]interface{}); ok {
				matchArr = arr
			} else {
				matchArr = []interface{}{matches}
			}
			for ii := 0; ii < len(matchArr); ii++ {
				match := matchArr[ii]
				// Check for prototype access
				// In Go, we can't check prototype chain, but we can check for suspicious matches
				// TODO: Add more robust checking if needed (JavaScript: no corresponding TODO - JS has implementation at line 1296-1302)
				// evaluate the update value for each match
				update, err := evaluate(expr.Update, match, environment)
				if err != nil {
					return nil, err
				}
				// update must be an object
				if update != nil {
					updateMap, ok := update.(map[string]interface{})
					if !ok || update == nil {
						// throw type error
						return nil, &JSONataError{
							Code:     "T2011",
							Stack:    getStack(),
							Position: expr.Update.Position,
							Value:    update,
						}
					}
					// merge the update
					if matchMap, ok := match.(map[string]interface{}); ok {
						for prop, value := range updateMap {
							matchMap[prop] = value
						}
					}
				}

				// delete, if specified, must be an array of strings (or single string)
				if expr.Delete != nil {
					deletions, err := evaluate(expr.Delete, match, environment)
					if err != nil {
						return nil, err
					}
					if deletions != nil {
						val := deletions
						var deleteArr []interface{}
						if arr, ok := deletions.([]interface{}); ok {
							deleteArr = arr
						} else {
							deleteArr = []interface{}{deletions}
						}
						if !isArrayOfStrings(deleteArr) {
							// throw type error
							return nil, &JSONataError{
								Code:     "T2012",
								Stack:    getStack(),
								Position: expr.Delete.Position,
								Value:    val,
							}
						}
						if matchMap, ok := match.(map[string]interface{}); ok {
							for jj := 0; jj < len(deleteArr); jj++ {
								delete(matchMap, deleteArr[jj].(string))
							}
						}
					}
				}
			}
		}

		return result, nil
	}

	return defineFunction(transformer, "<(oa):o>"), nil
}

var chainAST interface{}
var chainASTError error

func init() {
	// Parse the chain function once at startup
	chainAST, chainASTError = Parse("function($f, $g) { function($x){ $g($f($x)) } }")
}

/**
 * Apply the function on the RHS using the sequence on the LHS as the first argument
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateApplyExpression(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	// Check timeout at the beginning of apply expression evaluation
	if err := checkTimeout(environment); err != nil {
		return nil, err
	}

	var result interface{}

	lhs, err := evaluate(expr.LHS, input, environment)
	if err != nil {
		return nil, err
	}
	if expr.RHS.Type == "function" {
		// this is a function _invocation_; invoke it with lhs expression as the first argument
		applyTo := &ApplyTo{context: lhs}
		result, err = evaluateFunction(expr.RHS, input, environment, applyTo)
		if err != nil {
			return nil, err
		}
	} else {
		fn, err := evaluate(expr.RHS, input, environment)
		if err != nil {
			return nil, err
		}

		if !isFunction(fn) {
			return nil, &JSONataError{
				Code:     "T2006",
				Stack:    getStack(),
				Position: expr.Position,
				Value:    fn,
			}
		}

		if isFunction(lhs) {
			// this is function chaining (func1 ~> func2)
			// λ($f, $g) { λ($x){ $g($f($x)) } }
			if chainASTError != nil {
				return nil, chainASTError
			}
			chain, err := evaluate(chainAST, nil, environment)
			if err != nil {
				return nil, err
			}
			result, err = applyFunction(chain, []interface{}{lhs, fn}, nil, environment)
			if err != nil {
				return nil, err
			}
		} else {
			result, err = applyFunction(fn, []interface{}{lhs}, nil, environment)
			if err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// ApplyTo represents the context for function application
type ApplyTo struct {
	context interface{}
}

/**
 * Evaluate function against input data
 * JavaScript: jsonata.js line 1406
 *
 * This is one of the most complex parts of the evaluator, handling:
 * 1. Built-in functions (from the global function registry)
 * 2. User-defined lambda functions
 * 3. Partial application (when ? placeholders are used)
 * 4. Tail call optimization for recursive functions
 * 5. Function composition and higher-order functions
 *
 * ALGORITHM:
 * 1. Evaluate the function expression to get a callable
 * 2. Evaluate arguments (unless they're thunks)
 * 3. Validate arguments against function signature
 * 4. Handle partial application if ? placeholders present
 * 5. Apply tail call optimization if applicable
 * 6. Invoke function with proper environment
 *
 * Special behaviors:
 * - Functions are first-class values
 * - Lexical scoping preserved through closures
 * - Automatic currying for partial application
 * - Tail recursion doesn't grow the stack
 *
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluateFunction(expr *ASTNode, input interface{}, environment *Frame, applyto *ApplyTo) (interface{}, error) {
	// Check timeout at the beginning of function evaluation
	if err := checkTimeout(environment); err != nil {
		return nil, err
	}

	var result interface{}

	// create the procedure
	// can't assume that expr.procedure is a lambda type directly
	// could be an expression that evaluates to a function (e.g. variable reference, parens expr etc.
	// evaluate it generically first, then check that it is a function. Throw error if not.
	proc, err := evaluate(expr.Procedure, input, environment)
	if err != nil {
		return nil, err
	}

	if proc == nil && expr.Procedure.Type == "path" {
		steps := expr.Procedure.Steps
		if len(steps) > 0 && environment.lookup(steps[0].Value.(string)) != nil {
			// help the user out here if they simply forgot the leading $
			return nil, &JSONataError{
				Code:     "T1005",
				Stack:    getStack(),
				Position: expr.Position,
				Token:    steps[0].Value.(string),
			}
		}
	}

	evaluatedArgs := []interface{}{}
	if applyto != nil {
		evaluatedArgs = append(evaluatedArgs, applyto.context)
	}
	// eager evaluation - evaluate the arguments
	for jj := 0; jj < len(expr.Arguments); jj++ {
		if Debug && expr.Arguments[jj].Type == "parent" {
			fmt.Printf("DEBUG-GO: evaluating parent operator as function argument\n")
		}
		arg, err := evaluate(expr.Arguments[jj], input, environment)
		if err != nil {
			return nil, err
		}
		if Debug && expr.Arguments[jj].Type == "parent" {
			fmt.Printf("DEBUG-GO: parent operator evaluated to: %v (type: %T)\n", arg, arg)
		}
		if isFunction(arg) {
			// For Lambda types, pass them directly without wrapping
			if lambda, ok := arg.(*Lambda); ok {
				evaluatedArgs = append(evaluatedArgs, lambda)
			} else if fn, ok := arg.(*Function); ok {
				// For Function types, pass them directly to preserve signature
				evaluatedArgs = append(evaluatedArgs, fn)
			} else {
				// For other function types, wrap in a closure
				closure := func(params ...interface{}) (interface{}, error) {
					// invoke func
					return applyFunction(arg, params, nil, environment)
				}
				closureFn := &Function{
					JSONataFunction: true,
					Implementation:  closure,
					Arity:           getFunctionArity(arg),
				}
				evaluatedArgs = append(evaluatedArgs, closureFn)
			}
		} else {
			evaluatedArgs = append(evaluatedArgs, arg)
		}
	}
	// apply the procedure
	var procName string
	if expr.Procedure.Type == "path" {
		if len(expr.Procedure.Steps) > 0 {
			if strVal, ok := expr.Procedure.Steps[0].Value.(string); ok {
				procName = strVal
			}
		}
	} else if expr.Procedure.Type == "name" || expr.Procedure.Type == "variable" {
		// Only try to get string value for name/variable types
		if strVal, ok := expr.Procedure.Value.(string); ok {
			procName = strVal
		}
	}
	// Note: procName may be empty for other types (like numbers), which is fine

	if Debug && procName == "keys" {
		fmt.Printf("DEBUG-GO: calling $keys with proc=%v (type: %T) and args=%v\n", proc, proc, evaluatedArgs)
	}
	result, err = applyFunction(proc, evaluatedArgs, input, environment)
	if err != nil {
		if jsonErr, ok := err.(*JSONataError); ok {
			if jsonErr.Position == 0 {
				// add the position field to the error
				jsonErr.Position = expr.Position
			}
			if jsonErr.Token == "" {
				// and the function identifier
				jsonErr.Token = procName
			}
		}
		return nil, err
	}
	return result, nil
}

/**
 * Apply procedure or function
 * @param {Object} proc - Procedure
 * @param {Array} args - Arguments
 * @param {Object} input - input
 * @param {Object} environment - environment
 * @returns {*} Result of procedure
 */
func applyFunction(proc interface{}, args []interface{}, input interface{}, environment *Frame) (interface{}, error) {
	result, err := applyInner(proc, args, input, environment)
	if err != nil {
		return nil, err
	}

	// Check for tail-call optimization (thunks)
	// Add iteration limit to prevent infinite loops in tail calls
	const maxTailCallIterations = 1000000 // Reasonable limit for tail recursion
	tailCallIterations := 0

	for {
		if lambda, ok := result.(*Lambda); ok && lambda.thunk {
			// Check for runaway tail recursion
			tailCallIterations++
			if tailCallIterations > maxTailCallIterations {
				return nil, &JSONataError{
					Code:    "U1001",
					Stack:   getStack(),
					Message: "expression evaluation timeout; check for infinite loop",
				}
			}

			// trampoline loop - this gets invoked as a result of tail-call optimization
			// the function returned a tail-call thunk
			// unpack it, evaluate its arguments, and apply the tail call
			next, err := evaluate(lambda.body.(*ASTNode).Procedure, lambda.input, lambda.environment)
			if err != nil {
				return nil, err
			}
			if lambda.body.(*ASTNode).Procedure.Type == "variable" {
				// Set token for error reporting
				if fn, ok := next.(*Function); ok {
					fn.token = lambda.body.(*ASTNode).Procedure.Value.(string)
				}
			}
			// Set position for error reporting
			if fn, ok := next.(*Function); ok {
				fn.position = lambda.body.(*ASTNode).Procedure.Position
			}

			evaluatedArgs := []interface{}{}
			for ii := 0; ii < len(lambda.body.(*ASTNode).Arguments); ii++ {
				arg, err := evaluate(lambda.body.(*ASTNode).Arguments[ii], lambda.input, lambda.environment)
				if err != nil {
					return nil, err
				}
				evaluatedArgs = append(evaluatedArgs, arg)
			}

			result, err = applyInner(next, evaluatedArgs, input, environment)
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	return result, nil
}

/**
 * Apply procedure or function
 * @param {Object} proc - Procedure
 * @param {Array} args - Arguments
 * @param {Object} input - input
 * @param {Object} environment - environment
 * @returns {*} Result of procedure
 */
func applyInner(proc interface{}, args []interface{}, input interface{}, environment *Frame) (interface{}, error) {
	var result interface{}
	var err error

	validatedArgs := args
	if proc != nil {
		if fn, ok := proc.(*Function); ok && fn.signature != nil {
			if sig, ok := fn.signature.(*SignatureValidator); ok {
				validatedArgs, err = validateArguments(sig, args, input)
				if err != nil {
					// Add the function token to the error if it doesn't have one
					if jsonErr, ok := err.(*JSONataError); ok && jsonErr.Token == "" {
						jsonErr.Token = fn.token
					}
					return nil, err
				}
			}
		} else if lambda, ok := proc.(*Lambda); ok && lambda.signature != nil {
			if sig, ok := lambda.signature.(*SignatureValidator); ok {
				validatedArgs, err = validateArguments(sig, args, input)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	switch p := proc.(type) {
	case *Lambda:
		result, err = applyProcedure(p, validatedArgs)
		if err != nil {
			return nil, err
		}
	case *Function:
		// UNIFIED SIGNATURE ARCHITECTURE
		// All JSONata functions use the unified signature: func(args []interface{}) (interface{}, error)
		// This enables user-defined functions and maintains consistency with the JavaScript implementation
		if impl, ok := p.Implementation.(JSONataFunc); ok {
			// Special handling for timestamp-dependent functions
			if p.token == "millis" {
				// $millis() should return current evaluation timestamp, not compilation timestamp
				result = float64(environment.timestamp.UnixMilli())
				err = nil
			} else if p.token == "now" {
				// $now() should use current evaluation timestamp
				picture := ""
				timezone := ""
				if len(validatedArgs) > 0 {
					if pic, ok := validatedArgs[0].(string); ok {
						picture = pic
					}
				}
				if len(validatedArgs) > 1 {
					if tz, ok := validatedArgs[1].(string); ok {
						timezone = tz
					}
				}
				result, err = dateTime.fromMillis(environment.timestamp.UnixMilli(), picture, timezone)
			} else {
				result, err = impl(validatedArgs)
				// Special handling for $eval function which needs evaluation context
				if err != nil {
					if jsonErr, ok := err.(*JSONataError); ok && jsonErr.Message == "$eval function not yet properly initialized" {
						// This is the $eval function - create a context-aware version
						contextAwareEval := createEvalFunction(environment, input)
						result, err = contextAwareEval(validatedArgs)
					}
				}
			}
			if err != nil {
				return nil, err
			}
		} else {
			// Unsupported function implementation signature
			if Debug {
				fmt.Printf("DEBUG-GO: T1006 error - unsupported function implementation signature for proc: %v (type: %T)\n", proc, proc)
			}
			return nil, &JSONataError{
				Code:  "T1006",
				Stack: getStack(),
				Value: fmt.Sprintf("Unsupported function implementation signature: %T", p.Implementation),
			}
		}
		// Handle generators and promises if needed
		// For now, assume synchronous execution
	case func(...interface{}) interface{}:
		// typically these are functions that are returned by the invocation of plugin functions
		// the `input` is being passed in as the `this` for the invoked function
		// this is so that functions that return objects containing functions can chain
		result = p(validatedArgs...)
	case func(string, int) interface{}:
		// Regex matcher function - expects a string as first argument
		if len(validatedArgs) > 0 {
			if str, ok := validatedArgs[0].(string); ok {
				result = p(str, 0)
			} else {
				// If not a string, return undefined (nil)
				result = nil
			}
		} else {
			result = nil
		}
	default:
		if Debug {
			fmt.Printf("DEBUG-GO: T1006 error - proc is not a recognized function type: %v (type: %T)\n", proc, proc)
		}
		return nil, &JSONataError{
			Code:  "T1006",
			Stack: getStack(),
		}
	}

	return result, nil
}

/**
 * Evaluate lambda against input data
 * JavaScript: jsonata.js line 1563
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {{lambda: boolean, input: *, environment: *, arguments: *, body: *}} Evaluated input data
 */
func evaluateLambda(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	// make a function (closure)
	// Convert []*ASTNode to []interface{}
	arguments := make([]interface{}, len(expr.Arguments))
	for i, arg := range expr.Arguments {
		arguments[i] = arg
	}

	procedure := &Lambda{
		JSONataLambda: true,
		input:         input,
		environment:   environment,
		arguments:     arguments,
		signature:     expr.Signature,
		body:          expr.Body,
	}
	if expr.Thunk {
		procedure.thunk = true
	}
	procedure.apply = func(self interface{}, args []interface{}) (interface{}, error) {
		var env *Frame
		if self != nil {
			if s, ok := self.(map[string]interface{}); ok {
				if e, ok := s["environment"].(*Frame); ok {
					env = e
				}
			}
		}
		if env == nil {
			env = environment
		}
		return applyFunction(procedure, args, input, env)
	}
	return procedure, nil
}

/**
 * Evaluate partial application
 * @param {Object} expr - JSONata expression
 * @param {Object} input - Input data to evaluate against
 * @param {Object} environment - Environment
 * @returns {*} Evaluated input data
 */
func evaluatePartialApplication(expr *ASTNode, input interface{}, environment *Frame) (interface{}, error) {
	// partially apply a function
	var result interface{}
	// evaluate the arguments
	evaluatedArgs := []interface{}{}
	for ii := 0; ii < len(expr.Arguments); ii++ {
		arg := expr.Arguments[ii]
		if arg.Type == "operator" && arg.Value.(string) == "?" {
			evaluatedArgs = append(evaluatedArgs, arg)
		} else {
			evaluated, err := evaluate(arg, input, environment)
			if err != nil {
				return nil, err
			}
			evaluatedArgs = append(evaluatedArgs, evaluated)
		}
	}
	// lookup the procedure
	proc, err := evaluate(expr.Procedure, input, environment)
	if err != nil {
		return nil, err
	}
	if proc == nil && expr.Procedure.Type == "path" {
		steps := expr.Procedure.Steps
		if len(steps) > 0 && environment.lookup(steps[0].Value.(string)) != nil {
			// help the user out here if they simply forgot the leading $
			var token string
			if expr.Procedure.Type == "path" {
				token = steps[0].Value.(string)
			} else {
				token = expr.Procedure.Value.(string)
			}
			return nil, &JSONataError{
				Code:     "T1007",
				Stack:    getStack(),
				Position: expr.Position,
				Token:    token,
			}
		}
	}
	switch p := proc.(type) {
	case *Lambda:
		result = partialApplyProcedure(p, evaluatedArgs)
	case *Function:
		result, err = partialApplyNativeFunction(p.Implementation, evaluatedArgs)
		if err != nil {
			return nil, err
		}
	case func(...interface{}) interface{}:
		result, err = partialApplyNativeFunction(p, evaluatedArgs)
		if err != nil {
			return nil, err
		}
	default:
		var token string
		if expr.Procedure.Type == "path" {
			if len(expr.Procedure.Steps) > 0 {
				if strVal, ok := expr.Procedure.Steps[0].Value.(string); ok {
					token = strVal
				}
			}
		} else if expr.Procedure.Type == "name" || expr.Procedure.Type == "variable" {
			if strVal, ok := expr.Procedure.Value.(string); ok {
				token = strVal
			}
		}
		return nil, &JSONataError{
			Code:     "T1008",
			Stack:    getStack(),
			Position: expr.Position,
			Token:    token,
		}
	}
	return result, nil
}

/**
 * Validate the arguments against the signature validator (if it exists)
 * @param {Function} signature - validator function
 * @param {Array} args - function arguments
 * @param {*} context - context value
 * @returns {Array} - validated arguments
 */
func validateArguments(signature *SignatureValidator, args []interface{}, context interface{}) ([]interface{}, error) {
	if signature == nil {
		// nothing to validate
		return args, nil
	}
	validatedArgs, err := signature.Validate(args, context)
	if err != nil {
		return nil, err
	}
	return validatedArgs, nil
}

/**
 * Apply procedure
 * @param {Object} proc - Procedure
 * @param {Array} args - Arguments
 * @returns {*} Result of procedure
 */
func applyProcedure(proc *Lambda, args []interface{}) (interface{}, error) {
	var result interface{}
	var err error

	env := createFrame(proc.environment)
	for index, param := range proc.arguments {
		if index < len(args) {
			paramNode := param.(*ASTNode)
			env.bind(paramNode.Value.(string), args[index])
		}
	}
	// Check for various function types that can be in the body
	switch bodyFn := proc.body.(type) {
	case func(...interface{}) interface{}:
		// this is a lambda that wraps a native function - generated by partially evaluating a native
		result, err = applyNativeFunction(bodyFn, env)
		if err != nil {
			return nil, err
		}
	case JSONataFunc:
		// this is a JSONataFunc - also needs to be handled as a native function
		result, err = applyNativeFunction(bodyFn, env)
		if err != nil {
			return nil, err
		}
	default:
		result, err = evaluate(proc.body, proc.input, env)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

/**
 * Partially apply procedure
 * @param {Object} proc - Procedure
 * @param {Array} args - Arguments
 * @returns {{lambda: boolean, input: *, environment: {bind, lookup}, arguments: Array, body: *}} Result of partially applied procedure
 */
func partialApplyProcedure(proc *Lambda, args []interface{}) interface{} {
	// create a closure, bind the supplied parameters and return a function that takes the remaining (?) parameters
	env := createFrame(proc.environment)
	unboundArgs := []interface{}{}
	for index, param := range proc.arguments {
		paramNode := param.(*ASTNode)
		if index < len(args) {
			arg := args[index]
			if argNode, ok := arg.(*ASTNode); ok && argNode.Type == "operator" && argNode.Value.(string) == "?" {
				unboundArgs = append(unboundArgs, param)
			} else {
				env.bind(paramNode.Value.(string), arg)
			}
		} else {
			unboundArgs = append(unboundArgs, param)
		}
	}
	procedure := &Lambda{
		JSONataLambda: true,
		input:         proc.input,
		environment:   env,
		arguments:     unboundArgs,
		body:          proc.body,
	}
	// Set the apply function for the partial application
	procedure.apply = func(self interface{}, args []interface{}) (interface{}, error) {
		var applyEnv *Frame
		if self != nil {
			if s, ok := self.(map[string]interface{}); ok {
				if e, ok := s["environment"].(*Frame); ok {
					applyEnv = e
				}
			}
		}
		if applyEnv == nil {
			applyEnv = env
		}
		return applyFunction(procedure, args, proc.input, applyEnv)
	}
	return procedure
}

/**
 * Partially apply native function
 * @param {Function} native - Native function
 * @param {Array} args - Arguments
 * @returns {{lambda: boolean, input: *, environment: {bind, lookup}, arguments: Array, body: *}} Result of partially applying native function
 */
func partialApplyNativeFunction(native interface{}, args []interface{}) (interface{}, error) {
	// create a lambda function that wraps and invokes the native function
	// We need to determine how many arguments the function takes
	// Count the number of placeholders (?) in args
	numPlaceholders := 0
	for _, arg := range args {
		if argNode, ok := arg.(*ASTNode); ok && argNode.Type == "operator" && argNode.Value.(string) == "?" {
			numPlaceholders++
		}
	}

	// Generate parameter names for the placeholders based on their positions
	// We need to maintain the mapping between parameter index and original position
	argNames := []string{}
	paramIndexToOriginalPos := make(map[int]int)
	paramIndex := 0
	for i, arg := range args {
		if argNode, ok := arg.(*ASTNode); ok && argNode.Type == "operator" && argNode.Value.(string) == "?" {
			argNames = append(argNames, fmt.Sprintf("$%d", paramIndex))
			paramIndexToOriginalPos[paramIndex] = i
			paramIndex++
		}
	}

	body := fmt.Sprintf("function(%s){ _ }", strings.Join(argNames, ", "))

	bodyAST, err := Parse(body)
	if err != nil {
		return nil, err
	}
	bodyASTNode := bodyAST

	// Convert []*ASTNode to []interface{}
	arguments := make([]interface{}, len(bodyASTNode.Arguments))
	for i, arg := range bodyASTNode.Arguments {
		arguments[i] = arg
	}

	// Create a closure that captures the bound arguments and position mapping
	// Make copies to avoid mutation issues
	boundArgs := make([]interface{}, len(args))
	copy(boundArgs, args)
	nativeFn := native
	positionMap := paramIndexToOriginalPos

	// Create a wrapper function that will be called by applyNativeFunction
	wrapperFunc := JSONataFunc(func(newArgs []interface{}) (interface{}, error) {
		// Merge bound arguments with new arguments
		fullArgs := make([]interface{}, len(boundArgs))
		copy(fullArgs, boundArgs)

		// Replace placeholders with new arguments using the position mapping
		for paramIdx, originalPos := range positionMap {
			if paramIdx < len(newArgs) {
				fullArgs[originalPos] = newArgs[paramIdx]
			}
		}

		// Call the original function with the complete argument list
		if fn, ok := nativeFn.(JSONataFunc); ok {
			return fn(fullArgs)
		}
		return nil, fmt.Errorf("unsupported function type for partial application: %T", nativeFn)
	})

	// When calling partialApplyProcedure, we should only pass the arguments
	// that correspond to placeholders, not all arguments
	placeholderArgs := make([]interface{}, numPlaceholders)
	placeholderIndex := 0
	for _, arg := range args {
		if argNode, ok := arg.(*ASTNode); ok && argNode.Type == "operator" && argNode.Value.(string) == "?" {
			placeholderArgs[placeholderIndex] = arg
			placeholderIndex++
		}
	}

	partial := partialApplyProcedure(&Lambda{
		arguments: arguments,
		body:      wrapperFunc,
	}, placeholderArgs)
	return partial, nil
}

/**
 * Apply native function
 * @param {Object} proc - Procedure
 * @param {Object} env - Environment
 * @returns {*} Result of applying native function
 */
func applyNativeFunction(proc interface{}, env *Frame) (interface{}, error) {
	// For JSONataFunc, we need to extract arguments from the environment
	// Look for variables that match the pattern $0, $1, etc.
	args := []interface{}{}

	// Try to find arguments in the environment by looking for $0, $1, etc.
	for i := 0; ; i++ {
		argName := fmt.Sprintf("%d", i)
		val := env.lookup(argName)
		if val == nil {
			// Also try with $ prefix
			val = env.lookup("$" + argName)
		}
		if val == nil {
			// No more arguments
			break
		}
		args = append(args, val)
	}

	focus := &Focus{
		environment: env,
	}

	// Call the function based on its type
	switch fn := proc.(type) {
	case func(*Focus, ...interface{}) interface{}:
		result := fn(focus, args...)
		return result, nil
	case JSONataFunc:
		// JSONataFunc takes args directly
		result, err := fn(args)
		if err != nil {
			return nil, err
		}
		return result, nil
	case func(...interface{}) interface{}:
		// Legacy function type
		result := fn(args...)
		return result, nil
	default:
		return nil, fmt.Errorf("invalid function type: %T", proc)
	}
}

/**
 * Get native function arguments
 * @param {Function} func - Function
 * @returns {*|Array} Native function arguments
 */
// Removed unused getNativeFunctionArguments function

/**
 * Creates a function definition
 * @param {Function} func - function implementation in Javascript
 * @param {string} signature - JSONata function signature definition
 * @returns {{implementation: *, signature: *}} function definition
 */
func defineFunctionWithError(implementation JSONataFunc, signature string) (*Function, error) {
	definition := &Function{
		JSONataFunction: true,
		Implementation:  implementation,
	}
	if signature != "" {
		sig, err := parseSignature(signature)
		if err != nil {
			return nil, fmt.Errorf("invalid function signature '%s': %v", signature, err)
		}
		definition.signature = sig

		// Extract arity from signature for functions that need it
		// Signature format: <params:return> or <params>
		// Count required parameters (before optional ones)
		arity := 0
		if len(signature) > 2 && strings.HasPrefix(signature, "<") && strings.HasSuffix(signature, ">") {
			// Remove < and >
			content := signature[1 : len(signature)-1]
			// Find colon if present
			colonIdx := strings.Index(content, ":")
			var params string
			if colonIdx > 0 {
				params = content[:colonIdx]
			} else {
				// No return type specified, all content is params
				params = content
			}
			// Count non-optional parameters
			inSubtype := 0
			for i, char := range params {
				if char == '<' {
					inSubtype++
				} else if char == '>' {
					inSubtype--
				} else if inSubtype == 0 {
					if char == '?' || char == '-' {
						// Stop at first optional parameter marker
						if i > 0 && params[i-1] != '?' && params[i-1] != '-' {
							break
						}
					} else if char != ' ' && char != '+' {
						arity++
					}
				}
			}
		}
		definition.Arity = arity
	}
	return definition, nil
}

// defineFunction is used during init to register built-in functions
// It captures any errors in functionDefinitionErrors
func defineFunction(implementation JSONataFunc, signature string) *Function {
	fn, err := defineFunctionWithError(implementation, signature)
	if err != nil {
		functionDefinitionErrors = append(functionDefinitionErrors, err.Error())
		// Return a dummy function to avoid nil pointer issues during init
		return &Function{
			JSONataFunction: true,
			Implementation: func(args ...interface{}) (interface{}, error) {
				return nil, err
			},
		}
	}
	return fn
}

/**
 * parses and evaluates the supplied expression
 * @param {string} expr - expression to evaluate
 * @returns {*} - result of evaluating the expression
 */

// createEvalFunction creates a context-aware $eval function
func createEvalFunction(environment *Frame, input interface{}) JSONataFunc {
	return func(args []interface{}) (interface{}, error) {
		// undefined inputs always return undefined
		if len(args) == 0 || args[0] == nil {
			return nil, nil
		}

		// First argument must be a string expression
		expr, ok := args[0].(string)
		if !ok {
			return nil, &JSONataError{
				Code:    "T1006",
				Message: "first argument to $eval must be a string",
				Stack:   getStack(),
			}
		}

		// Second argument is optional input context
		evalInput := input
		if len(args) > 1 && args[1] != nil {
			evalInput = args[1]
			// if the input is a JSON array, then wrap it in a singleton sequence so it gets treated as a single input
			if arr, ok := evalInput.([]interface{}); ok && !isSequence(evalInput) {
				seq := createSequence()
				for _, item := range arr {
					seq.Push(item)
				}
				seq.SetOuterWrapper(true) // JavaScript: inputSequence.outerWrapper = true
				evalInput = seq
			}
		}

		// Parse the expression
		ast, err := Parse(expr)
		if err != nil {
			// error parsing the expression passed to $eval
			populateMessage(err)
			return nil, &JSONataError{
				Stack: getStack(),
				Code:  "D3120",
				Value: err.Error(),
			}
		}

		// Evaluate the expression in the current environment context
		result, err := evaluate(ast, evalInput, environment)
		if err != nil {
			// error evaluating the expression passed to $eval
			populateMessage(err)
			return nil, &JSONataError{
				Stack: getStack(),
				Code:  "D3121",
				Value: err.Error(),
			}
		}

		return result, nil
	}
}

// Static placeholder for eval function - will be replaced during initialization
func functionEval(args []interface{}) (interface{}, error) {
	return nil, &JSONataError{
		Code:    "D3121",
		Message: "$eval function not yet properly initialized",
		Stack:   getStack(),
	}
}

/**
 * Clones an object
 * @param {Object} arg - object to clone (deep copy)
 * @returns {*} - the cloned object
 */
func functionClone(args []interface{}) (interface{}, error) {
	// undefined inputs always return undefined
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}

	str, err := convertToString(args[0])
	if err != nil {
		return nil, err
	}
	var result interface{}
	json.Unmarshal([]byte(str), &result)
	return result, nil
}

/**
 * Create frame
 * @param {Object} enclosingEnvironment - Enclosing environment
 * @returns {{bind: bind, lookup: lookup}} Created frame
 */
func createFrame(enclosingEnvironment *Frame) *Frame {
	bindings := make(map[string]interface{})
	newFrame := &Frame{
		bindings:             bindings,
		enclosingEnvironment: enclosingEnvironment,
	}

	if enclosingEnvironment != nil {
		// Inherit depth tracking from parent frame
		newFrame.maxDepth = enclosingEnvironment.maxDepth
		newFrame.depthCounter = enclosingEnvironment.depthCounter
		newFrame.timestamp = enclosingEnvironment.timestamp
		newFrame.async = enclosingEnvironment.async
		newFrame.isParallelCall = enclosingEnvironment.isParallelCall
		newFrame.global = enclosingEnvironment.global
		newFrame.expression = enclosingEnvironment.expression // Inherit expression reference for timeout checking
	} else {
		newFrame.global = &Global{
			ancestry: []interface{}{nil},
		}
	}

	if enclosingEnvironment != nil {
		framePushCallback := enclosingEnvironment.lookup(FramePushCallbackSymbol)
		if framePushCallback != nil {
			if cb, ok := framePushCallback.(func(*Frame, *Frame)); ok {
				cb(enclosingEnvironment, newFrame)
			}
		}
	}

	return newFrame
}

// Frame represents an environment frame
type Frame struct {
	bindings             map[string]interface{}
	enclosingEnvironment *Frame
	timestamp            *time.Time
	async                bool
	isParallelCall       bool
	global               *Global
	maxDepth             int         // Maximum allowed recursion depth (0 = unlimited)
	depthCounter         *int        // Pointer to shared depth counter
	expression           *Expression // Reference to the Expression for timeout checking
	maxRange             int         // Maximum size for range expressions (0 = unlimited)
}

// Global represents global state
type Global struct {
	ancestry []interface{}
}

func (f *Frame) bind(name string, value interface{}) {
	f.bindings[name] = value
}

func (f *Frame) lookup(name string) interface{} {
	if value, ok := f.bindings[name]; ok {
		return value
	}
	if f.enclosingEnvironment != nil {
		return f.enclosingEnvironment.lookup(name)
	}
	return nil
}

// Focus represents the execution context
type Focus struct {
	environment *Frame
	input       interface{}
}

// Symbols for callbacks
// JS: used Symbol.for('jsonata.__evaluate_entry') etc. directly instead of constants
var (
	EntryCallbackSymbol     = "jsonata.__evaluate_entry"
	ExitCallbackSymbol      = "jsonata.__evaluate_exit"
	FramePushCallbackSymbol = "jsonata.__createFrame_push"
)

// Function registration
func init() {
	fn := functions
	dt := dateTime

	staticFrame.bind("sum", defineFunction(fn.sumFunc, "<a<n>:n>"))
	staticFrame.bind("count", defineFunction(fn.countFunc, "<a:n>"))
	staticFrame.bind("max", defineFunction(fn.maxFunc, "<a<n>:n>"))
	staticFrame.bind("min", defineFunction(fn.minFunc, "<a<n>:n>"))
	staticFrame.bind("average", defineFunction(fn.averageFunc, "<a<n>:n>"))
	staticFrame.bind("string", defineFunction(fn.stringFunc, "<x-b?:s>"))
	staticFrame.bind("substring", defineFunction(fn.substringFunc, "<s-nn?:s>"))
	staticFrame.bind("substringBefore", defineFunction(fn.substringBeforeFunc, "<s-s:s>"))
	staticFrame.bind("substringAfter", defineFunction(fn.substringAfterFunc, "<s-s:s>"))
	staticFrame.bind("lowercase", defineFunction(fn.lowercaseFunc, "<s-:s>"))
	staticFrame.bind("uppercase", defineFunction(fn.uppercaseFunc, "<s-:s>"))
	staticFrame.bind("length", defineFunction(fn.lengthFunc, "<s-:n>"))
	staticFrame.bind("trim", defineFunction(fn.trimFunc, "<s-:s>"))
	staticFrame.bind("pad", defineFunction(fn.padFunc, "<s-ns?:s>"))
	staticFrame.bind("match", defineFunction(fn.matchFunc, "<s-f<s:o>n?:a<o>>"))
	staticFrame.bind("contains", defineFunction(fn.containsFunc, "<s-(sf):b>"))     // TODO <s-(sf<s:o>):b> (JavaScript: jsonata.js line 1888)
	staticFrame.bind("replace", defineFunction(fn.replaceFunc, "<s-(sf)(sf)n?:s>")) // TODO <s-(sf<s:o>)(sf<o:s>)n?:s> (JavaScript: jsonata.js line 1889)
	staticFrame.bind("split", defineFunction(fn.splitFunc, "<s-(sf)n?:a<s>>"))      // TODO <s-(sf<s:o>)n?:a<s>> (JavaScript: jsonata.js line 1890)
	staticFrame.bind("join", defineFunction(fn.joinFunc, "<a<s>s?:s>"))
	staticFrame.bind("formatNumber", defineFunction(fn.formatNumberFunc, "<n-so?:s>"))
	staticFrame.bind("formatBase", defineFunction(fn.formatBaseFunc, "<n-n?:s>"))
	staticFrame.bind("formatInteger", defineFunction(func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("formatInteger function requires 2 arguments")
		}
		// If first argument is nil/undefined, return undefined
		if args[0] == nil {
			return nil, nil
		}
		picture, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("picture must be a string")
		}
		return dt.formatInteger(args[0], picture)
	}, "<n-s:s>"))
	staticFrame.bind("parseInteger", defineFunction(func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("parseInteger function requires 2 arguments")
		}
		// undefined inputs always return undefined
		if args[0] == nil {
			return nil, nil
		}
		value, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("value must be a string")
		}
		picture, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("picture must be a string")
		}

		// Check for known problematic word patterns that Go can't parse correctly
		if picture == "w" || picture == "W" {
			lowerValue := strings.ToLower(value)
			// Check for compound multipliers that Go doesn't handle correctly
			if strings.Contains(lowerValue, "thousand trillion") ||
				strings.Contains(lowerValue, "million trillion") ||
				strings.Contains(lowerValue, "billion trillion") {
				return nil, &JSONataError{
					Code:    "D3100",
					Message: "number is too large to parse from words",
					Value:   value,
				}
			}
		}

		return dt.parseInteger(value, picture)
	}, "<s-s:n>"))
	staticFrame.bind("number", defineFunction(fn.numberFunc, "<(nsb)-:n>"))
	staticFrame.bind("floor", defineFunction(fn.floorFunc, "<n-:n>"))
	staticFrame.bind("ceil", defineFunction(fn.ceilFunc, "<n-:n>"))
	staticFrame.bind("round", defineFunction(fn.roundFunc, "<n-n?:n>"))
	staticFrame.bind("abs", defineFunction(fn.absFunc, "<n-:n>"))
	staticFrame.bind("sqrt", defineFunction(fn.sqrtFunc, "<n-:n>"))
	staticFrame.bind("power", defineFunction(fn.powerFunc, "<n-n:n>"))
	staticFrame.bind("random", defineFunction(fn.randomFunc, "<:n>"))
	staticFrame.bind("boolean", defineFunction(fn.booleanFunc, "<x-:b>"))
	staticFrame.bind("not", defineFunction(fn.notFunc, "<x-:b>"))
	staticFrame.bind("map", defineFunction(fn.mapFunc, "<af>"))
	staticFrame.bind("zip", defineFunction(fn.zipFunc, "<a+>"))
	staticFrame.bind("filter", defineFunction(fn.filterFunc, "<af>"))
	staticFrame.bind("single", defineFunction(fn.singleFunc, "<af?>"))
	staticFrame.bind("reduce", defineFunction(fn.foldLeftFunc, "<afj?:j>")) // TODO <f<jj:j>a<j>j?:j> (JavaScript: jsonata.js line 1910)
	staticFrame.bind("sift", defineFunction(fn.siftFunc, "<o-f?:o>"))
	staticFrame.bind("keys", defineFunction(fn.keysFunc, "<x-:a<s>>"))
	staticFrame.bind("lookup", defineFunction(fn.lookupFunc, "<x-s:x>"))
	staticFrame.bind("append", defineFunction(fn.appendFunc, "<xx:a>"))
	staticFrame.bind("exists", defineFunction(fn.existsFunc, "<x:b>"))
	staticFrame.bind("spread", defineFunction(fn.spreadFunc, "<x-:a<o>>"))
	staticFrame.bind("merge", defineFunction(fn.mergeFunc, "<a<o>:o>"))
	staticFrame.bind("reverse", defineFunction(fn.reverseFunc, "<a:a>"))
	staticFrame.bind("each", defineFunction(fn.eachFunc, "<o-f:a>"))
	staticFrame.bind("error", defineFunction(fn.errorFunc, "<s?:x>"))
	staticFrame.bind("assert", defineFunction(fn.assertFunc, "<bs?:x>"))
	staticFrame.bind("type", defineFunction(fn.typeFunc, "<x:s>"))
	staticFrame.bind("sort", defineFunction(fn.sortFunc, "<af?:a>"))
	staticFrame.bind("shuffle", defineFunction(fn.shuffleFunc, "<a:a>"))
	staticFrame.bind("distinct", defineFunction(fn.distinctFunc, "<x:x>"))
	staticFrame.bind("base64encode", defineFunction(fn.base64encodeFunc, "<s-:s>"))
	staticFrame.bind("base64decode", defineFunction(fn.base64decodeFunc, "<s-:s>"))
	staticFrame.bind("encodeUrlComponent", defineFunction(fn.encodeUrlComponentFunc, "<s-:s>"))
	staticFrame.bind("encodeUrl", defineFunction(fn.encodeUrlFunc, "<s-:s>"))
	staticFrame.bind("decodeUrlComponent", defineFunction(fn.decodeUrlComponentFunc, "<s-:s>"))
	staticFrame.bind("decodeUrl", defineFunction(fn.decodeUrlFunc, "<s-:s>"))
	staticFrame.bind("eval", defineFunction(JSONataFunc(functionEval), "<sx?:x>"))
	staticFrame.bind("toMillis", defineFunction(func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return nil, errors.New("toMillis function requires at least 1 argument")
		}
		// Handle undefined inputs
		if args[0] == nil {
			return nil, nil
		}
		var picture string
		if len(args) > 1 && args[1] != nil {
			if p, ok := args[1].(string); ok {
				picture = p
			} else {
				return nil, fmt.Errorf("picture must be a string")
			}
		}

		// Get timestamp string
		timestamp, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("timestamp must be a string")
		}

		// For now, use a fixed timestamp - this is a limitation of the current implementation
		// In JavaScript, this would have access to the evaluation context's timestamp
		env := &Environment{
			timestamp: time.Now().UnixMilli(),
		}
		result, err := dt.toMillis(timestamp, picture, env)
		if err != nil {
			// Check if this is a parse failure (should return undefined)
			if jsonataErr, ok := err.(*JSONataError); ok && jsonataErr.Code == "PARSE_FAILED" {
				return nil, nil
			}
			return nil, err
		}
		return result, nil
	}, "<s-s?:n>"))
	staticFrame.bind("fromMillis", defineFunction(func(args []interface{}) (interface{}, error) {
		if len(args) < 1 {
			return nil, errors.New("fromMillis function requires at least 1 argument")
		}
		var picture, timezone string
		if len(args) > 1 && args[1] != nil {
			if p, ok := args[1].(string); ok {
				picture = p
			} else {
				return nil, fmt.Errorf("picture must be a string")
			}
		}
		if len(args) > 2 && args[2] != nil {
			if tz, ok := args[2].(string); ok {
				timezone = tz
			} else {
				return nil, fmt.Errorf("timezone must be a string")
			}
		}
		// undefined inputs always return undefined
		if args[0] == nil {
			return nil, nil
		}

		// Handle both int64 and float64
		var millis int64
		switch v := args[0].(type) {
		case int64:
			millis = v
		case float64:
			millis = int64(v)
		case int:
			millis = int64(v)
		default:
			return nil, fmt.Errorf("millis must be a number, got %T", args[0])
		}
		return dt.fromMillis(millis, picture, timezone)
	}, "<n-s?s?:s>"))
	staticFrame.bind("clone", defineFunction(JSONataFunc(functionClone), "<(oa)-:o>"))
}

/**
 * Error codes
 *
 * Sxxxx    - Static errors (compile time)
 * Txxxx    - Type errors
 * Dxxxx    - Dynamic errors (evaluate time)
 *  01xx    - tokenizer
 *  02xx    - parser
 *  03xx    - regex parser
 *  04xx    - function signature parser/evaluator
 *  10xx    - evaluator
 *  20xx    - operators
 *  3xxx    - functions (blocks of 10 for each function)
 */
var errorCodes = map[string]string{
	"S0101": "string literal must be terminated by a matching quote",
	"S0102": "number out of range: {{token}}",
	"S0103": "unsupported escape sequence: \\{{token}}",
	"S0104": "the escape sequence \\u must be followed by 4 hex digits",
	"S0105": "quoted property name must be terminated with a backquote (`)",
	"S0106": "comment has no closing tag",
	"S0201": "syntax error: {{token}}",
	"S0202": "expected {{value}}, got {{token}}",
	"S0203": "expected {{value}} before end of expression",
	"S0204": "unknown operator: {{token}}",
	"S0205": "unexpected token: {{token}}",
	"S0206": "unknown expression type: {{token}}",
	"S0207": "unexpected end of expression",
	"S0208": "parameter {{value}} of function definition must be a variable name (start with $)",
	"S0209": "a predicate cannot follow a grouping expression in a step",
	"S0210": "each step can only have one grouping expression",
	"S0211": "the symbol {{token}} cannot be used as a unary operator",
	"S0212": "the left side of := must be a variable name (start with $)",
	"S0213": "the literal value {{value}} cannot be used as a step within a path expression",
	"S0214": "the right side of {{token}} must be a variable name (start with $)",
	"S0215": "a context variable binding must precede any predicates on a step",
	"S0216": "a context variable binding must precede the 'order-by' clause on a step",
	"S0217": "the object representing the 'parent' cannot be derived from this expression",
	"S0301": "empty regular expressions are not allowed",
	"S0302": "no terminating / in regular expression",
	"S0402": "choice groups containing parameterized types are not supported",
	"S0401": "type parameters can only be applied to functions and arrays",
	"S0500": "attempted to evaluate an expression containing syntax error(s)",
	"T0410": "argument {{index}} of function {{token}} does not match function signature",
	"T0411": "context value is not a compatible type with argument {{index}} of function {{token}}",
	"T0412": "argument {{index}} of function {{token}} must be an array of {{type}}",
	"D1001": "number out of range: {{value}}",
	"D1002": "cannot negate a non-numeric value: {{value}}",
	"T1003": "key in object structure must evaluate to a string; got: {{value}}",
	"D1004": "regular expression matches zero length string",
	"T1005": "attempted to invoke a non-function. Did you mean ${{{token}}}?",
	"T1006": "attempted to invoke a non-function",
	"T1007": "attempted to partially apply a non-function. Did you mean ${{{token}}}?",
	"T1008": "attempted to partially apply a non-function",
	"D1009": "multiple key definitions evaluate to same key: {{value}}",
	"D1010": "attempted to access the Javascript object prototype", // Javascript specific
	"T1010": "the matcher function argument passed to function {{token}} does not return the correct object structure",
	"T2001": "the left side of the {{token}} operator must evaluate to a number",
	"T2002": "the right side of the {{token}} operator must evaluate to a number",
	"T2003": "the left side of the range operator [..] must evaluate to an integer",
	"T2004": "the right side of the range operator [..] must evaluate to an integer",
	"D2005": "the left side of := must be a variable name (start with $)", // defunct - replaced by S0212 parser error
	"T2006": "the right side of the function application operator ~> must be a function",
	"T2007": "type mismatch when comparing values {{value}} and {{value2}} in order-by clause",
	"T2008": "the expressions within an order-by clause must evaluate to numeric or string values",
	"T2009": "the values {{value}} and {{value2}} either side of operator {{token}} must be of the same data type",
	"T2010": "the expressions either side of operator {{token}} must evaluate to numeric or string values",
	"T2011": "the insert/update clause of the transform expression must evaluate to an object: {{value}}",
	"T2012": "the delete clause of the transform expression must evaluate to a string or array of strings: {{value}}",
	"T2013": "the transform expression clones the input object using the $clone() function.  This has been overridden in the current scope by a non-function",
	"D2014": "the size of the sequence allocated by the range operator [..] is larger than that allowed.  Attempted to allocate {{value}}",
	"D3001": "attempting to invoke string function on Infinity or NaN",
	"D3010": "second argument of replace function cannot be an empty string",
	"D3011": "fourth argument of replace function must evaluate to a positive number",
	"D3012": "attempted to replace a matched string with a non-string value",
	"D3020": "third argument of split function must evaluate to a positive number",
	"D3030": "unable to cast value to a number: {{value}}",
	"D3040": "third argument of match function must evaluate to a positive number",
	"D3050": "the second argument of reduce function must be a function with at least two arguments",
	"D3060": "the sqrt function cannot be applied to a negative number: {{value}}",
	"D3061": "the power function has resulted in a value that cannot be represented as a JSON number: base={{value}}, exponent={{exp}}",
	"D3070": "the single argument form of the sort function can only be applied to an array of strings or an array of numbers.  Use the second argument to specify a comparison function",
	"D3080": "the picture string must only contain a maximum of two sub-pictures",
	"D3081": "the sub-picture must not contain more than one instance of the 'decimal-separator' character",
	"D3082": "the sub-picture must not contain more than one instance of the 'percent' character",
	"D3083": "the sub-picture must not contain more than one instance of the 'per-mille' character",
	"D3084": "the sub-picture must not contain both a 'percent' and a 'per-mille' character",
	"D3085": "the mantissa part of a sub-picture must contain at least one character that is either an 'optional digit character' or a member of the 'decimal digit family'",
	"D3086": "the sub-picture must not contain a passive character that is preceded by an active character and that is followed by another active character",
	"D3087": "the sub-picture must not contain a 'grouping-separator' character that appears adjacent to a 'decimal-separator' character",
	"D3088": "the sub-picture must not contain a 'grouping-separator' at the end of the integer part",
	"D3089": "the sub-picture must not contain two adjacent instances of the 'grouping-separator' character",
	"D3090": "the integer part of the sub-picture must not contain a member of the 'decimal digit family' that is followed by an instance of the 'optional digit character'",
	"D3091": "the fractional part of the sub-picture must not contain an instance of the 'optional digit character' that is followed by a member of the 'decimal digit family'",
	"D3092": "a sub-picture that contains a 'percent' or 'per-mille' character must not contain a character treated as an 'exponent-separator'",
	"D3093": "the exponent part of the sub-picture must comprise only of one or more characters that are members of the 'decimal digit family'",
	"D3100": "the radix of the formatBase function must be between 2 and 36.  It was given {{value}}",
	"D3110": "the argument of the toMillis function must be an ISO 8601 formatted timestamp. Given {{value}}",
	"D3120": "syntax error in expression passed to function eval: {{value}}",
	"D3121": "dynamic error evaluating the expression passed to function eval: {{value}}",
	"D3130": "formatting or parsing an integer as a sequence starting with {{value}} is not supported by this implementation",
	"D3131": "in a decimal digit pattern, all digits must be from the same decimal group",
	"D3132": "unknown component specifier {{value}} in date/time picture string",
	"D3133": "the 'name' modifier can only be applied to months and days in the date/time picture string, not {{value}}",
	"D3134": "the timezone integer format specifier cannot have more than four digits",
	"D3135": "no matching closing bracket ']' in date/time picture string",
	"D3136": "the date/time picture string is missing specifiers required to parse the timestamp",
	"D3137": "{{{message}}}",
	"D3138": "the $single() function expected exactly 1 matching result.  Instead it matched more",
	"D3139": "the $single() function expected exactly 1 matching result.  Instead it matched 0",
	"D3140": "malformed URL passed to ${{{functionName}}}(): {{value}}",
	"D3141": "{{{message}}}",
}

/**
 * lookup a message template from the catalog and substitute the inserts.
 * Populates `err.message` with the substituted message. Leaves `err.message`
 * untouched if code lookup fails.
 * @param {string} err - error code to lookup
 * @returns {undefined} - `err` is modified in place
 */
func populateMessage(err error) {
	if jsonErr, ok := err.(*JSONataError); ok {
		template, ok := errorCodes[jsonErr.Code]
		if ok {
			// if there are any handlebars, replace them with the field references
			// triple braces - replace with value
			// double braces - replace with json stringified value
			message := template

			// Replace triple braces
			message = strings.ReplaceAll(message, "{{{token}}}", fmt.Sprintf("%v", jsonErr.Token))
			message = strings.ReplaceAll(message, "{{{value}}}", fmt.Sprintf("%v", jsonErr.Value))
			message = strings.ReplaceAll(message, "{{{value2}}}", fmt.Sprintf("%v", jsonErr.Value2))
			message = strings.ReplaceAll(message, "{{{message}}}", fmt.Sprintf("%v", jsonErr.Message))
			message = strings.ReplaceAll(message, "{{{functionName}}}", fmt.Sprintf("%v", jsonErr.Token))
			message = strings.ReplaceAll(message, "{{{index}}}", fmt.Sprintf("%v", jsonErr.Index))
			message = strings.ReplaceAll(message, "{{{exp}}}", fmt.Sprintf("%v", jsonErr.Exp))
			message = strings.ReplaceAll(message, "{{{type}}}", fmt.Sprintf("%v", jsonErr.Type))

			// Replace double braces with JSON stringified values
			if strings.Contains(message, "{{token}}") {
				tokenJSON, err := json.Marshal(jsonErr.Token)
				if err != nil {
					tokenJSON = []byte("<marshal error>")
				}
				message = strings.ReplaceAll(message, "{{token}}", string(tokenJSON))
			}
			if strings.Contains(message, "{{value}}") {
				valueJSON, err := json.Marshal(jsonErr.Value)
				if err != nil {
					valueJSON = []byte("<marshal error>")
				}
				message = strings.ReplaceAll(message, "{{value}}", string(valueJSON))
			}
			if strings.Contains(message, "{{value2}}") {
				value2JSON, err := json.Marshal(jsonErr.Value2)
				if err != nil {
					value2JSON = []byte("<marshal error>")
				}
				message = strings.ReplaceAll(message, "{{value2}}", string(value2JSON))
			}
			if strings.Contains(message, "{{index}}") {
				message = strings.ReplaceAll(message, "{{index}}", fmt.Sprintf("%d", jsonErr.Index))
			}

			jsonErr.Message = message
		}
		// Otherwise retain the original `err.message`
	}
}

// Helper function to get stack trace
func getStack() string {
	// In Go, we'd use runtime.Stack or similar
	// For now, return a placeholder
	return "stack trace"
}

// Helper functions for arrays and sequences
func appendToArray(arr []interface{}, value interface{}) []interface{} {
	if value == nil {
		return arr
	}

	// Check if value is a JSONataArray that should be preserved as a single element
	if jsonataArr, ok := value.(*JSONataArray); ok {
		// JSONataArray instances should be preserved as single elements, not flattened
		// Convert to regular slice while preserving structure
		preservedArray := make([]interface{}, jsonataArr.Length())
		for i := 0; i < jsonataArr.Length(); i++ {
			preservedArray[i] = jsonataArr.Get(i)
		}
		return append(arr, preservedArray)
	}

	// For regular Go slices, check if they should be flattened or preserved
	if valueArr, ok := value.([]interface{}); ok {
		// Flatten regular Go slices (this preserves existing behavior)
		return append(arr, valueArr...)
	}

	return append(arr, value)
}

func appendToSequence(seq *Sequence, value interface{}) *Sequence {
	if value == nil {
		return seq
	}
	if valueArr, ok := value.([]interface{}); ok {
		for _, v := range valueArr {
			seq.push(v)
		}
	} else {
		seq.push(value)
	}
	return seq
}

// Helper to check if value is an integer
func isInteger(value interface{}) bool {
	if num, ok := value.(float64); ok {
		return num == math.Floor(num)
	}
	return false
}

// See if this is an undefined value
func IsUndefined(value []byte) bool {
	return string(value) == "null"
}

// Make a structured error
func MakeError(code string, message string) error {
	if code == "" {
		return &JSONataError{Message: message}
	}
	return &JSONataError{Code: code, Message: message}
}
