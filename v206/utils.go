// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

/*
JSONata Utility Functions and Core Data Structures
==================================================

Portions Copyright IBM Corp. 2016, 2018 All Rights Reserved
Project name: JSONata
This project is licensed under the MIT License, see LICENSE

Overview:
--------
This module provides essential utility functions and data structures that support
JSONata's core functionality. These utilities handle type checking, value conversion,
sequence management, and error handling that are fundamental to JSONata's operation.

Key Components:
--------------

1. **Error System**
   - JSONataError: Structured error type with codes, context, and stack traces
   - Error codes matching JavaScript implementation for consistency
   - Rich error context for debugging and user feedback

2. **Type Checking Utilities**
   - isNumeric: Validates numeric values with proper handling of edge cases
   - isArrayOfNumbers/Strings: Type validation for homogeneous arrays
   - Type conversion and validation functions

3. **Sequence Management**
   - Sequence: Core data structure representing JSONata's sequence types
   - Sequence manipulation functions (creation, flattening, wrapping)
   - Array vs sequence distinction handling

4. **Value Processing**
   - Deep equality comparison for JSONata semantics
   - Value conversion between Go types and JSONata's type system
   - Null/undefined handling following JSONata's graceful degradation

5. **String and Array Utilities**
   - String to character array conversion
   - Array manipulation functions
   - Unicode and internationalization support

6. **Frame and Environment**
   - Frame: Lexical environment for variable bindings
   - Environment chain management for closures and scoping
   - Variable lookup and binding functions

Critical Concepts:
----------------

**Sequence vs Array Distinction**
The Sequence type is fundamental to JSONata's behavior:
- Sequences are collections that follow flattening rules
- Arrays are preserved structures from input JSON
- The `outerWrapper` flag distinguishes input arrays from computed sequences
- Sequence processing implements JSONata's core evaluation semantics

**Graceful Degradation**
Utility functions implement JSONata's principle of graceful degradation:
- Undefined inputs typically produce undefined outputs
- Type mismatches don't cause hard errors when possible
- Functions continue operating on valid parts of invalid input

**Type Coercion**
Go's strong typing requires careful conversion to JSONata's dynamic semantics:
- Numeric types are normalized to float64 for consistency
- String/number conversions follow JSONata rules
- Boolean conversion matches JavaScript truthiness semantics
*/

package jsonata

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
)

/*
JSONata Error Codes
==================

Error codes follow the JavaScript implementation's taxonomy for consistency
across JSONata implementations. Each code category represents a different
class of error:

- D1xxx: Data errors (type mismatches, invalid values)
- S0xxx: Syntax errors (parsing failures)
- T1xxx, T2xxx: Type errors (function signature mismatches)
- U0xxx: User errors (undefined functions, variable references)

This ensures that error handling code can work across different JSONata
implementations with consistent error identification.
*/
const (
	ErrorCodeD1001 = "D1001" // Example data error code
)

// JSONataNull represents an explicit null value (as opposed to undefined)
// This allows us to distinguish between JavaScript's null and undefined
type JSONataNull struct{}

// Singleton instance of JSONataNull
var JSONNull = &JSONataNull{}

// String representation for debugging
func (n *JSONataNull) String() string {
	return "null"
}

// MarshalJSON implements json.Marshaler to serialize as JSON null
func (n *JSONataNull) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

// isJSONataNull checks if a value is the JSONataNull singleton
func isJSONataNull(v interface{}) bool {
	_, ok := v.(*JSONataNull)
	return ok
}

/*
JSONataError - Structured Error Type with Rich Context
=====================================================

JSONataError provides comprehensive error information for debugging and
user feedback. It extends Go's standard error interface with JSONata-specific
context information that matches the JavaScript implementation's error model.

Fields:
- Code: Standardized error code (e.g., "T2001", "D3020")
- Value: The problematic value that caused the error
- Value2: Secondary value for binary operation errors
- Stack: Go stack trace for debugging
- Position: Character position in source expression
- Token: The token that caused a parsing error
- Index: Parameter index for function argument errors
- Message: Human-readable error description
- Exp: Expected value or type information
- Type: Actual type that caused the mismatch
- Remaining: Additional context for complex errors

This structure enables detailed error reporting that helps users understand
both what went wrong and where in their expression the error occurred.
*/
type JSONataError struct {
	Code      string      // Standardized JSONata error code
	Value     interface{} // Primary problematic value
	Value2    interface{} // Secondary value (for binary operations)
	Stack     string      // Go stack trace for debugging
	Position  int         // Source position where error occurred
	Token     string      // Token that caused parsing error
	Index     int         // Parameter index for function errors
	Message   string      // Human-readable error description
	Exp       interface{} // Expected value or type
	Type      string      // Actual type that caused error
	Remaining interface{} // Additional error context
}

/*
Error - Standard Go Error Interface Implementation
===============================================

Provides string representation of JSONataError for compatibility with
Go's standard error handling. Formats the error code and primary value
in a consistent, readable format.
*/
func (e *JSONataError) Error() string {

	// Build error string with available information
	result := "unknown error"
	if e.Code != "" && e.Message != "" {
		result = e.Code + ": " + e.Message
	} else if e.Code == "" && e.Message != "" {
		result = e.Message
	} else if e.Code != "" && e.Message == "" {
		result = e.Code
	}

	// Add value if available and no message
	if e.Value != nil && e.Message == "" {
		result = fmt.Sprintf("%s: %v", result, e.Value)
	}

	// Add index information for function parameter errors
	if e.Index > 0 {
		result = fmt.Sprintf("%s (argument %d)", result, e.Index)
	}

	return result
}

/*
isNumeric - Finite Number Validation
===================================

Determines whether a given value represents a finite numeric value according
to JSONata's numeric semantics. This function is critical for mathematical
operations and type validation throughout JSONata.

Validation Rules:
- Accepts all Go numeric types (int variants, float variants)
- Rejects infinite values (positive/negative infinity)
- Rejects NaN (Not a Number) values
- Non-numeric types are rejected
- Follows JavaScript's Number.isFinite() semantics

This function ensures that mathematical operations work reliably by filtering
out edge cases that could produce unexpected results or errors in calculations.

JavaScript equivalent: utils.js lines 15-28

Parameters:
- n: Value to test for numeric validity

Returns:
- bool: true if n is a finite number, false otherwise
- error: Currently unused but maintained for interface compatibility
*/
func isNumeric(n interface{}) (bool, error) {
	_ = errors.New // currently unused import
	isNum := false
	switch v := n.(type) {
	case float64:
		isNum = !math.IsNaN(v)
		if isNum && math.IsInf(v, 0) {
			return false, &JSONataError{
				Code:  ErrorCodeD1001,
				Value: n,
				Stack: "", // Go doesn't have direct stack trace like JS
			}
		}
	case float32:
		v64 := float64(v)
		isNum = !math.IsNaN(v64)
		if isNum && math.IsInf(v64, 0) {
			return false, &JSONataError{
				Code:  ErrorCodeD1001,
				Value: n,
				Stack: "",
			}
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		isNum = true
	}
	return isNum, nil
}

/*
isString - Type checking function for string values

Efficiently determines if a value is a string using type assertions
instead of reflection, providing better performance.

Parameters:
- val: The value to check

Returns:
- bool: true if val is a string, false otherwise
*/
func isString(val interface{}) bool {
	_, ok := val.(string)
	return ok
}

/*
getTypeName - Get type name for comparison without reflection

Efficiently determines the type name of a value using type assertions
instead of reflection, providing better performance.

Parameters:
- val: The value to get type name for

Returns:
- string: Type name as a string
*/
func getTypeName(val interface{}) string {
	switch val.(type) {
	case nil:
		return "nil"
	case string:
		return "string"
	case int, int8, int16, int32, int64:
		return "int"
	case uint, uint8, uint16, uint32, uint64:
		return "uint"
	case float32, float64:
		return "float"
	case bool:
		return "bool"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}

// isArrayOfStrings returns true if the arg is an array of strings
// JavaScript: utils.js lines 35-42
// @param {interface{}} arg - the item to test
// @returns {bool} True if arg is an array of strings
func isArrayOfStrings(arg interface{}) bool {
	result := false
	/* istanbul ignore else */
	if arr, ok := arg.([]interface{}); ok {
		result = true
		for _, item := range arr {
			if _, ok := item.(string); !ok {
				result = false
				break
			}
		}
	} else if arr, ok := arg.([]string); ok {
		_ = arr // currently unused
		result = true
	}
	return result
}

// isArrayOfNumbers returns true if the arg is an array of numbers
// JavaScript: utils.js lines 49-55
// @param {interface{}} arg - the item to test
// @returns {bool} True if arg is an array of numbers
func isArrayOfNumbers(arg interface{}) bool {
	result := false
	if arr, ok := arg.([]interface{}); ok {
		result = true
		for _, item := range arr {
			isNum, err := isNumeric(item)
			if err != nil {
				return false
			}
			if !isNum {
				result = false
				break
			}
		}
	}
	return result
}

// JSONataArray represents a property-enhanced array identical to JavaScript
// In JavaScript: var arr = [1,2,3]; arr.sequence = true; arr.outerWrapper = true;
// This struct contains a slice and adds the JSONata-specific properties as fields
type JSONataArray struct {
	// Array data - equivalent to the JavaScript array's indexed elements
	Data []interface{} `json:"-"`

	// JSONata-specific properties that can be attached to arrays, just like JavaScript
	Sequence      *bool `json:"sequence,omitempty"`      // JavaScript: array.sequence = true
	TupleStream   *bool `json:"tupleStream,omitempty"`   // JavaScript: array.tupleStream = true
	KeepSingleton *bool `json:"keepSingleton,omitempty"` // JavaScript: array.keepSingleton = true
	OuterWrapper  *bool `json:"outerWrapper,omitempty"`  // JavaScript: array.outerWrapper = true
	Cons          *bool `json:"cons,omitempty"`          // JavaScript: array.cons = true
}

// MarshalJSON implements json.Marshaler for JSONataArray
// This ensures that when a JSONataArray is serialized to JSON, it serializes as its Data array
// rather than as an object with the JSONata metadata fields
func (a *JSONataArray) MarshalJSON() ([]byte, error) {
	if a == nil {
		return []byte("null"), nil
	}
	return json.Marshal(a.Data)
}

// Array-like operations that work exactly like JavaScript
func (a *JSONataArray) Length() int {
	if a == nil {
		return 0
	}
	return len(a.Data)
}

func (a *JSONataArray) Get(i int) interface{} {
	if a == nil || i >= len(a.Data) {
		return nil
	}
	return a.Data[i]
}

func (a *JSONataArray) Push(item interface{}) {
	if a == nil {
		return
	}
	a.Data = append(a.Data, item)
}

// SetSequence sets the sequence property like JavaScript: array.sequence = true
func (a *JSONataArray) SetSequence(val bool) {
	if a != nil {
		a.Sequence = &val
	}
}

// IsSequence checks if sequence property is true, like JavaScript: array.sequence === true
func (a *JSONataArray) IsSequence() bool {
	return a != nil && a.Sequence != nil && *a.Sequence
}

// SetOuterWrapper sets the outerWrapper property like JavaScript: array.outerWrapper = true
func (a *JSONataArray) SetOuterWrapper(val bool) {
	if a != nil {
		a.OuterWrapper = &val
	}
}

// IsOuterWrapper checks if outerWrapper property is true
func (a *JSONataArray) IsOuterWrapper() bool {
	return a != nil && a.OuterWrapper != nil && *a.OuterWrapper
}

// SetTupleStream sets the tupleStream property
func (a *JSONataArray) SetTupleStream(val bool) {
	if a != nil {
		a.TupleStream = &val
	}
}

// IsTupleStream checks if tupleStream property is true
func (a *JSONataArray) IsTupleStream() bool {
	return a != nil && a.TupleStream != nil && *a.TupleStream
}

// SetKeepSingleton sets the keepSingleton property
func (a *JSONataArray) SetKeepSingleton(val bool) {
	if a != nil {
		a.KeepSingleton = &val
	}
}

// IsKeepSingleton checks if keepSingleton property is true
func (a *JSONataArray) IsKeepSingleton() bool {
	return a != nil && a.KeepSingleton != nil && *a.KeepSingleton
}

// SetCons sets the cons property
func (a *JSONataArray) SetCons(val bool) {
	if a != nil {
		a.Cons = &val
	}
}

// IsCons checks if cons property is true
func (a *JSONataArray) IsCons() bool {
	return a != nil && a.Cons != nil && *a.Cons
}

// Compatibility methods for old Sequence interface
func (a *JSONataArray) length() int           { return a.Length() }
func (a *JSONataArray) push(item interface{}) { a.Push(item) }

// Sequence - alias for compatibility
type Sequence = JSONataArray

// createSequence creates an array with sequence property like JavaScript
// JavaScript: utils.js lines 61-68: var sequence = []; sequence.sequence = true; if (arguments.length === 1) sequence.push(arguments[0]);
func createSequence(args ...interface{}) *JSONataArray {
	trueVal := true
	sequence := &JSONataArray{
		Data:     make([]interface{}, 0),
		Sequence: &trueVal, // JavaScript: sequence.sequence = true
	}
	if len(args) == 1 {
		sequence.Push(args[0])
	}
	return sequence
}

// isSequence tests if a value is a sequence
// JavaScript: utils.js lines 75-77: return value.sequence === true && Array.isArray(value);
// @param {interface{}} value the value to test
// @returns {bool} true if it's a sequence
func isSequence(value interface{}) bool {
	// Check if it's a JSONataArray with sequence property set to true
	if arr, ok := value.(*JSONataArray); ok {
		return arr.IsSequence() // This checks if Sequence != nil && *Sequence == true
	}
	return false
}

// Lambda represents a JSONata lambda function
type Lambda struct {
	JSONataLambda  bool
	input          interface{}
	environment    *Frame
	arguments      []interface{}
	signature      interface{}
	body           interface{}
	thunk          bool
	apply          func(interface{}, []interface{}) (interface{}, error)
	Arity          int
	Implementation func(...interface{}) (interface{}, error)
}

// Function represents a JSONata function
type Function struct {
	JSONataFunction bool
	JSONataLambda   bool
	Implementation  interface{}
	signature       interface{}
	token           string
	position        interface{}
	Arity           int
}

// isFunction checks if arg is a function (lambda or built-in)
// JavaScript: utils.js lines 84-86
// @param {interface{}} arg - expression to test
// @returns {bool} - true if it is a function (lambda or built-in)
func isFunction(arg interface{}) bool {
	switch v := arg.(type) {
	case *Function:
		return v != nil && v.JSONataFunction
	case *Lambda:
		return v != nil && v.JSONataLambda
	case func(...interface{}) (interface{}, error):
		return true
	case func(string, int) interface{}:
		// Regex matcher function
		return true
	default:
		return false
	}
}

// getFunctionArity returns the arity (number of arguments) of the function
// JavaScript: utils.js lines 93-98
// @param {interface{}} funcArg - the function (JS: 'func')
// @returns {int} - the arity
func getFunctionArity(funcArg interface{}) int { // JS: parameter was named 'func'
	switch f := funcArg.(type) {
	case *Function:
		if f.Arity != 0 {
			return f.Arity
		}
		// In Go, we can't easily get function length like in JS
		return 0
	case *Lambda:
		if f.Arity != 0 {
			return f.Arity
		}
		return len(f.arguments)
	default:
		// For regular Go functions, we can't determine arity dynamically
		return 0
	}
}

// Removed unused isLambda, iteratorSymbol, and isIterable
// They can be restored if needed in the future

// isDeepEqual compares two values for equality
// JavaScript: utils.js lines 132-175
// @param {interface{}} lhs first value
// @param {interface{}} rhs second value
// @returns {bool} true if they are deep equal
func isDeepEqual(lhs, rhs interface{}) bool {
	// Handle nil cases first
	if lhs == nil && rhs == nil {
		return true
	}
	if lhs == nil || rhs == nil {
		return false
	}

	lhsValue := reflect.ValueOf(lhs)
	rhsValue := reflect.ValueOf(rhs)

	// Both must be same type
	if lhsValue.Type() != rhsValue.Type() {
		return false
	}

	switch lhsValue.Kind() {
	case reflect.Slice, reflect.Array:
		// both arrays (or sequences)
		// must be the same length
		if lhsValue.Len() != rhsValue.Len() {
			return false
		}
		// must contain same values in same order
		for ii := 0; ii < lhsValue.Len(); ii++ { // JS: same variable name 'ii'
			if !isDeepEqual(lhsValue.Index(ii).Interface(), rhsValue.Index(ii).Interface()) {
				return false
			}
		}
		return true

	case reflect.Map:
		// both objects
		// must have the same set of keys (in any order)
		lkeys := lhsValue.MapKeys()
		rkeys := rhsValue.MapKeys()
		if len(lkeys) != len(rkeys) {
			return false
		}

		// Convert keys to strings and sort
		var lkeyStrings []string // JS: lkeys (reused after sort)
		var rkeyStrings []string // JS: rkeys (reused after sort)
		for _, k := range lkeys {
			lkeyStrings = append(lkeyStrings, fmt.Sprintf("%v", k.Interface()))
		}
		for _, k := range rkeys {
			rkeyStrings = append(rkeyStrings, fmt.Sprintf("%v", k.Interface()))
		}
		sort.Strings(lkeyStrings)
		sort.Strings(rkeyStrings)

		for ii := 0; ii < len(lkeyStrings); ii++ {
			if lkeyStrings[ii] != rkeyStrings[ii] {
				return false
			}
		}

		// must have the same values
		for _, key := range lkeys {
			lval := lhsValue.MapIndex(key)
			rval := rhsValue.MapIndex(key)
			if !rval.IsValid() {
				return false
			}
			if !isDeepEqual(lval.Interface(), rval.Interface()) {
				return false
			}
		}
		return true

	case reflect.Struct:
		// Handle struct comparison if both are objects
		// This is a simplified version - in real JSONata we'd need to handle JSON objects properly
		return reflect.DeepEqual(lhs, rhs)

	default:
		return reflect.DeepEqual(lhs, rhs)
	}
}

// promise interface to mimic JavaScript promises (not implemented)
type promise interface {
	Then(func(interface{}) (interface{}, error)) promise
}

// isPromise checks if arg is a promise
// JavaScript: utils.js lines 181-188
// @param {interface{}} arg - expression to test
// @returns {bool} - true if it is a promise
func isPromise(arg interface{}) bool {
	if arg == nil {
		return false
	}
	_, ok := arg.(promise)
	return ok
}

// stringToArray converts a string to an array of characters
// JavaScript: utils.js lines 195-201
// @param {string} str - the input string
// @returns {[]string} - the array of characters
func stringToArray(str string) []string {
	arr := []string{}
	for _, char := range str {
		arr = append(arr, string(char))
	}
	return arr
}

// Module exports equivalent - in Go we export by capitalizing
// These are already exported as they start with lowercase in the original
// but are exported through the module.exports in JS
