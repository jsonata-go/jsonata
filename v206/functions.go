// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

/*
JSONata Built-in Functions Implementation
========================================

Portions Copyright IBM Corp. 2016, 2018 All Rights Reserved
Project name: JSONata
This project is licensed under the MIT License, see LICENSE

Overview:
--------
This file implements JSONata's comprehensive built-in function library, providing
the functional programming primitives that make JSONata a powerful transformation
language. These functions operate on JSONata's sequence-based type system and
follow the language's core principles.

Function Categories:
------------------

1. **Numeric Functions**
   - Mathematical operations: $sqrt, $power, $abs, $floor, $ceil, $round
   - Statistical functions: $sum, $average, $max, $min, $count
   - Random number generation: $random
   - Number formatting: $formatNumber, $formatBase, $formatInteger

2. **String Functions**
   - Text manipulation: $substring, $length, $trim, $pad
   - Case conversion: $lowercase, $uppercase
   - Pattern matching: $match, $contains, $replace, $split
   - String joining: $join
   - URL encoding/decoding: $encodeUrl, $decodeUrl, $encodeUrlComponent, $decodeUrlComponent

3. **Array Functions**
   - Higher-order functions: $map, $filter, $reduce, $sort
   - Array manipulation: $append, $reverse, $zip
   - Set operations: $distinct
   - Utility functions: $shuffle, $single

4. **Object Functions**
   - Property introspection: $keys, $lookup, $spread
   - Object merging: $merge
   - Property testing: $exists
   - Object iteration: $each, $sift

5. **Type and Utility Functions**
   - Type conversion: $string, $number, $boolean
   - Type inspection: $type
   - Existence testing: $exists
   - Error handling: $error, $assert
   - Base64 encoding: $base64encode, $base64decode

6. **Higher-Order Programming**
   - Function application and composition
   - Partial application support
   - Closure capture and lexical scoping

Implementation Principles:
-------------------------
- All functions handle JSONata's sequence types correctly
- Undefined inputs typically produce undefined outputs (graceful degradation)
- Functions follow JSONata's type coercion rules
- Error handling uses JSONata's structured error system
- Performance is optimized for common use cases while maintaining semantic correctness
*/

package jsonata

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Global function registry containing all JSONata built-in functions
// This is initialized once at package load time and provides the static
// function environment that all JSONata expressions inherit from
var functions = initFunctions()

/*
createJSONataError - Enhanced Error Creation with Stack Traces
============================================================

Creates JSONata errors with full stack trace information for debugging.
This provides much richer error context than simple error messages,
allowing developers to trace the execution path that led to an error.

Parameters:
- code: JSONata error code (e.g., "T2001", "D3001") following the standard error taxonomy
- value: The problematic value that caused the error
- index: Optional parameter index for function argument errors

Returns:
- Fully populated JSONataError with stack trace and context information

The stack trace helps identify:
- Which function call chain led to the error
- The exact line in the Go code where the error occurred
- The evaluation context when the error was encountered
*/
func createJSONataError(code string, value interface{}, index ...int) *JSONataError {
	// Capture current goroutine stack trace for debugging
	// This provides the exact execution path that led to the error
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	// Construct error with full context information
	err := &JSONataError{
		Code:  code,  // Standard JSONata error code
		Value: value, // The problematic value
		Stack: stack, // Go stack trace
	}

	// Add parameter index for function argument errors
	if len(index) > 0 {
		err.Index = index[0]
	}

	return err
}

/*
initFunctions - Initialize JSONata Built-in Function Library
===========================================================

This function creates and configures the complete set of JSONata built-in functions.
It serves as the central registry for all function implementations, establishing
the static function environment that expressions inherit from.

The initialization process:
1. Captures utility functions for reuse across implementations
2. Defines each built-in function with proper error handling
3. Configures function signatures for type validation
4. Returns a FunctionsModule containing all function implementations

Function Implementation Patterns:
- Undefined inputs generally return undefined (graceful degradation)
- Type checking validates arguments according to JSONata semantics
- Sequence handling respects JSONata's flattening rules
- Error codes follow JSONata's standard error taxonomy
- Performance optimizations while maintaining semantic correctness

The resulting FunctionsModule is used to populate the static frame,
making these functions available to all JSONata expressions.
*/

func initFunctions() *FunctionsModule {
	// This function creates the static function implementations
	// For functions that need environment context, they will be replaced
	// with environment-aware versions when called from applyInner

	// Capture utility functions for reuse in function implementations
	// These provide common patterns used across multiple built-in functions
	isNumeric := isNumeric
	isArrayOfStrings := isArrayOfStrings
	isArrayOfNumbers := isArrayOfNumbers
	createSequence := createSequence
	isSequence := isSequence
	isFunction := isFunction
	isPromise := isPromise
	getFunctionArity := getFunctionArity
	deepEquals := isDeepEqual // JS: var deepEquals = utils.isDeepEqual
	stringToArray := stringToArray

	// Currently unused but keeping for consistency with JS
	_ = isNumeric
	_ = isArrayOfStrings
	_ = isArrayOfNumbers
	_ = isPromise

	// Dummy references for unused imports
	_ = sort.Strings

	/**
	 * Sum function
	 * JavaScript equivalent: functions.js lines 29-40
	 *
	 * Aggregates numeric values from sequences, handling JSONata's type system:
	 * - Flattens nested arrays to find all numeric values
	 * - Ignores non-numeric values silently (no error)
	 * - Returns undefined for empty sequences or no numeric values
	 * - Handles both integer and floating-point addition
	 * - Maintains numeric precision as float64
	 *
	 * Examples:
	 * $sum([1, 2, 3]) => 6
	 * $sum([[1, 2], [3, 4]]) => 10 (flattened)
	 * $sum([1, "two", 3]) => 4 (non-numbers ignored)
	 * $sum([]) => undefined
	 * $sum(5) => 5 (singleton treated as array)
	 *
	 * @param {[]interface{}} args - Arguments
	 * @returns {float64} Total value of arguments
	 */
	/*
		$sum - Mathematical Sum Aggregation Function
		===========================================

		Calculates the sum of numeric values in an array, following JSONata's
		graceful degradation principles and type coercion rules.

		Behavior:
		- Undefined/nil input returns undefined (follows JSONata semantics)
		- Non-numeric values are ignored (graceful handling)
		- Supports both integer and floating-point arithmetic
		- Returns floating-point result for mathematical consistency

		Example usage:
		$sum([1, 2, 3, 4, 5])        → 15
		$sum([1.5, 2.5, 3.0])        → 7.0
		$sum([1, "hello", 3])        → 4 (ignores non-numeric)
		$sum([])                     → 0
		$sum(undefined)              → undefined

		JavaScript equivalent: functions.js lines 40-46
	*/
	sumFunc := func(args []interface{}) (interface{}, error) {
		// undefined inputs always return undefined
		// This follows JSONata's principle of graceful degradation
		if args == nil {
			return nil, nil
		}

		// Check if the first argument is nil (undefined)
		if len(args) == 1 && args[0] == nil {
			return nil, nil
		}

		// Handle the case where we receive an array as the first argument
		// This happens when JSONata passes [1,2,3,4,5] as args[0] instead of as separate args
		var numbersToSum []interface{}
		if len(args) == 1 {
			if arr, ok := args[0].([]interface{}); ok {
				numbersToSum = arr
			} else {
				numbersToSum = args
			}
		} else {
			numbersToSum = args
		}

		// Accumulate sum using floating-point arithmetic for precision
		total := 0.0
		for _, arg := range numbersToSum {
			// Handle both native Go numeric types
			if num, ok := arg.(float64); ok {
				total += num
			} else if num, ok := arg.(int); ok {
				total += float64(num)
			}
			// Non-numeric values are silently ignored (graceful degradation)
		}
		return total, nil
	}

	/**
	 * Count function
	 * JavaScript equivalent: functions.js lines 47-54
	 *
	 * Counts the number of values in a sequence:
	 * - Handles nested arrays by flattening
	 * - Counts all values regardless of type
	 * - Returns 0 for undefined input
	 * - Singleton values count as 1
	 *
	 * This is different from $length which counts:
	 * - Characters in a string
	 * - Elements in an array (without flattening)
	 *
	 * Examples:
	 * $count([1, 2, 3]) => 3
	 * $count([[1, 2], [3, 4]]) => 4 (flattened)
	 * $count("hello") => 1 (string is one value)
	 * $count(undefined) => 0
	 * @param {[]interface{}} args - Arguments
	 * @returns {int} Number of elements in the array
	 */
	countFunc := func(args []interface{}) (interface{}, error) {
		// JavaScript returns 0 for undefined inputs (lines 49-51)
		if args == nil {
			return 0, nil
		}

		// Check if the first argument is nil (undefined)
		if len(args) == 1 && args[0] == nil {
			return 0, nil
		}

		// Handle the case where we receive an array as the first argument
		var elementsToCount []interface{}
		if len(args) == 1 {
			if arr, ok := args[0].([]interface{}); ok {
				elementsToCount = arr
			} else {
				elementsToCount = args
			}
		} else {
			elementsToCount = args
		}

		return float64(len(elementsToCount)), nil
	}

	/**
	 * Max function
	 * JavaScript: functions.js line 61
	 * @param {[]interface{}} args - Arguments
	 * @returns {float64} Max element in the array
	 */
	maxFunc := func(args []interface{}) (interface{}, error) {
		// undefined inputs always return undefined
		if len(args) == 0 {
			return nil, nil
		}

		// Handle the case where we receive an array as the first argument
		var numbersToMax []interface{}
		if len(args) == 1 {
			if arr, ok := args[0].([]interface{}); ok {
				numbersToMax = arr
			} else {
				numbersToMax = args
			}
		} else {
			numbersToMax = args
		}

		if len(numbersToMax) == 0 {
			return nil, nil
		}

		maxVal := math.Inf(-1)
		for _, arg := range numbersToMax {
			if num, ok := arg.(float64); ok && num > maxVal {
				maxVal = num
			} else if num, ok := arg.(int); ok && float64(num) > maxVal {
				maxVal = float64(num)
			}
		}
		if math.IsInf(maxVal, -1) {
			return nil, nil
		}
		return maxVal, nil
	}

	/**
	 * Min function
	 * JavaScript: functions.js line 75
	 * @param {[]interface{}} args - Arguments
	 * @returns {float64} Min element in the array
	 */
	minFunc := func(args []interface{}) (interface{}, error) {
		// undefined inputs always return undefined
		if len(args) == 0 {
			return nil, nil
		}

		// Handle the case where we receive an array as the first argument
		var numbersToMin []interface{}
		if len(args) == 1 {
			if arr, ok := args[0].([]interface{}); ok {
				numbersToMin = arr
			} else {
				numbersToMin = args
			}
		} else {
			numbersToMin = args
		}

		if len(numbersToMin) == 0 {
			return nil, nil
		}

		minVal := math.Inf(1)
		for _, arg := range numbersToMin {
			if num, ok := arg.(float64); ok && num < minVal {
				minVal = num
			} else if num, ok := arg.(int); ok && float64(num) < minVal {
				minVal = float64(num)
			}
		}
		if math.IsInf(minVal, 1) {
			return nil, nil
		}
		return minVal, nil
	}

	/**
	 * Average function
	 * JavaScript: functions.js line 89
	 * @param {[]interface{}} args - Arguments
	 * @returns {float64} Average element in the array
	 */
	averageFunc := func(args []interface{}) (interface{}, error) {
		// undefined inputs always return undefined
		if len(args) == 0 {
			return nil, nil
		}

		// Check if the first argument is nil (undefined)
		if len(args) == 1 && args[0] == nil {
			return nil, nil
		}

		// Handle the case where we receive an array as the first argument
		var numbersToAverage []interface{}
		if len(args) == 1 {
			if arr, ok := args[0].([]interface{}); ok {
				numbersToAverage = arr
			} else {
				numbersToAverage = args
			}
		} else {
			numbersToAverage = args
		}

		if len(numbersToAverage) == 0 {
			return nil, nil
		}

		total := 0.0
		count := 0
		for _, arg := range numbersToAverage {
			if num, ok := arg.(float64); ok {
				total += num
				count++
			} else if num, ok := arg.(int); ok {
				total += float64(num)
				count++
			}
		}
		if count == 0 {
			return nil, nil
		}
		return total / float64(count), nil
	}

	// preprocessForJSON applies JavaScript's JSON.stringify replacer logic
	var preprocessForJSON func(interface{}) (interface{}, error)
	preprocessForJSON = func(val interface{}) (interface{}, error) {
		switch v := val.(type) {
		case float64:
			// Check for Infinity/NaN before processing
			if math.IsInf(v, 0) || math.IsNaN(v) {
				// JavaScript throws D1001 when stringifying objects with Infinity/NaN
				return nil, createJSONataError("D1001", v)
			}
			// JavaScript: Number(val.toPrecision(15))
			// Format with precision 15, then parse back
			str := strconv.FormatFloat(v, 'g', 15, 64)
			if parsed, err := strconv.ParseFloat(str, 64); err == nil {
				return parsed, nil
			}
			return v, nil
		case map[string]interface{}:
			// Recursively process object values
			result := make(map[string]interface{})
			for k, v := range v {
				if isFunction(v) {
					result[k] = ""
				} else {
					processed, err := preprocessForJSON(v)
					if err != nil {
						return nil, err
					}
					result[k] = processed
				}
			}
			return result, nil
		case []interface{}:
			// Recursively process array values
			result := make([]interface{}, len(v))
			for i, item := range v {
				if isFunction(item) {
					result[i] = ""
				} else {
					processed, err := preprocessForJSON(item)
					if err != nil {
						return nil, err
					}
					result[i] = processed
				}
			}
			return result, nil
		default:
			if isFunction(val) {
				return "", nil
			}
			return val, nil
		}
	}

	/**
	 * Stringify arguments
	 * JavaScript: functions.js line 108
	 * @param {interface{}} arg - Arguments
	 * @param {bool} prettify - Pretty print the result
	 * @returns {string} String from arguments
	 */
	stringFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first is the value, second is optional prettify boolean
		if len(args) == 0 {
			// JavaScript: string() with no args should use context, but this is handled by signature
			// This shouldn't be reached if signature is correct
			return nil, nil
		}

		arg := args[0]

		// undefined inputs always return undefined
		if arg == nil {
			// nil represents undefined, return undefined
			return nil, nil
		}

		// Check for explicit null (JSONataNull)
		if _, isNull := arg.(*JSONataNull); isNull {
			// JavaScript returns "null" as a string when stringifying null
			return "null", nil
		}

		prettify := false
		if len(args) > 1 {
			if p, ok := args[1].(bool); ok {
				prettify = p
			}
		}

		var str string

		switch v := arg.(type) {
		case string:
			// already a string
			str = v
		case float64:
			if !math.IsInf(v, 0) && !math.IsNaN(v) {
				// Fall through to use JSON marshaling for consistency
			} else {
				return nil, createJSONataError("D3001", arg)
			}
		case int:
			// Fall through to use JSON marshaling
		case bool:
			// Fall through to use JSON marshaling
		}

		// For non-string values, use JSON marshaling
		if str == "" {
			if isFunction(arg) {
				// functions (built-in and lambda convert to empty string
				str = ""
			} else {
				// Custom JSON marshaling to match JavaScript behavior
				preprocessed, err := preprocessForJSON(arg)
				if err != nil {
					return nil, err
				}
				var bytes []byte
				if prettify {
					bytes, err = json.MarshalIndent(preprocessed, "", "  ")
				} else {
					bytes, err = json.Marshal(preprocessed)
				}
				if err != nil {
					return nil, err
				}
				str = string(bytes)
			}
		}
		return str, nil
	}

	/**
	 * Create substring based on character number and length
	 * JavaScript: functions.js line 148
	 * @param {string} str - String to evaluate
	 * @param {int} start - Character number to start substring
	 * @param {int} length - Number of characters in substring
	 * @returns {string} Substring
	 */
	substringFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: string, start index, optional length
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		start := 0
		if len(args) > 1 {
			if s, ok := args[1].(float64); ok {
				start = int(s)
			} else if s, ok := args[1].(int); ok {
				start = s
			}
		}

		var length *int
		if len(args) > 2 {
			if l, ok := args[2].(float64); ok {
				val := int(l)
				length = &val
			} else if l, ok := args[2].(int); ok {
				length = &l
			}
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		strArray := stringToArray(str)
		strLength := len(strArray)

		if strLength+start < 0 {
			start = 0
		}

		if length != nil {
			if *length <= 0 {
				return "", nil
			}
			var end int
			if start >= 0 {
				end = start + *length
			} else {
				end = strLength + start + *length
			}
			if end > strLength {
				end = strLength
			}
			if start < 0 {
				start = strLength + start
			}
			if start < 0 {
				start = 0
			}
			return strings.Join(strArray[start:end], ""), nil
		}

		if start < 0 {
			start = strLength + start
		}
		if start < 0 {
			start = 0
		}
		return strings.Join(strArray[start:], ""), nil
	}

	/**
	 * Create substring up until a character
	 * @param {string} str - String to evaluate
	 * @param {string} chars - Character to define substring boundary
	 * @returns {string} Substring
	 */
	substringBeforeFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: string, chars
		if len(args) < 2 {
			return nil, nil
		}

		// undefined inputs always return undefined
		if args[0] == nil {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		} else {
			// Non-string input - the signature should have caught this,
			// but if not, convert to string
			str = fmt.Sprintf("%v", args[0])
		}

		chars := ""
		if c, ok := args[1].(string); ok {
			chars = c
		}

		pos := strings.Index(str, chars)
		if pos > -1 {
			return str[:pos], nil
		}
		return str, nil
	}

	/**
	 * Create substring after a character
	 * @param {string} str - String to evaluate
	 * @param {string} chars - Character to define substring boundary
	 * @returns {string} Substring
	 */
	substringAfterFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: string, chars
		if len(args) < 2 {
			return nil, nil
		}

		// undefined inputs always return undefined
		if args[0] == nil {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		} else {
			// Non-string input - the signature should have caught this,
			// but if not, convert to string
			str = fmt.Sprintf("%v", args[0])
		}

		chars := ""
		if c, ok := args[1].(string); ok {
			chars = c
		}

		pos := strings.Index(str, chars)
		if pos > -1 {
			return str[pos+len(chars):], nil
		}
		return str, nil
	}

	/**
	 * Lowercase a string
	 * JavaScript: functions.js line 217
	 * @param {string} str - String to evaluate
	 * @returns {string} Lowercase string
	 */
	lowercaseFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		return strings.ToLower(str), nil
	}

	/**
	 * Uppercase a string
	 * JavaScript: functions.js line 231
	 * @param {string} str - String to evaluate
	 * @returns {string} Uppercase string
	 */
	uppercaseFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		return strings.ToUpper(str), nil
	}

	/**
	 * length of a string
	 * JavaScript equivalent: functions.js lines 245-252
	 * @param {string} str - string
	 * @returns {int} The number of characters in the string
	 */
	lengthFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		// undefined inputs always return undefined
		if args[0] == nil {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		return float64(len(stringToArray(str))), nil
	}

	/**
	 * Normalize and trim whitespace within a string
	 * JavaScript equivalent: functions.js lines 259-276
	 * @param {string} str - string to be trimmed
	 * @returns {string} - trimmed string
	 */
	trimFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		// normalize whitespace
		re := regexp.MustCompile(`[ \t\n\r]+`)
		result := re.ReplaceAllString(str, " ")
		// strip leading and trailing space
		result = strings.TrimSpace(result)
		return result, nil
	}

	/**
	 * Pad a string to a minimum width by adding characters to the start or end
	 * JavaScript equivalent: functions.js lines 285-312
	 * @param {string} str - string to be padded
	 * @param {int} width - the minimum width; +ve pads to the right, -ve pads to the left
	 * @param {string} char - the pad character(s); defaults to ' '
	 * @returns {string} - padded string
	 */
	padFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: string, width (int), optional char (string)
		if len(args) < 2 {
			return nil, nil
		}

		// undefined inputs always return undefined
		if args[0] == nil {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		width := 0
		if w, ok := args[1].(float64); ok {
			width = int(w)
		} else if w, ok := args[1].(int); ok {
			width = w
		}

		char := " " // default
		if len(args) > 2 && args[2] != nil {
			if c, ok := args[2].(string); ok {
				char = c
			}
		}

		if char == "" {
			char = " "
		}

		var result string
		width = int(width)
		strLen := len(stringToArray(str))
		padLength := int(math.Abs(float64(width))) - strLen
		if padLength > 0 {
			// JavaScript: new Array(padLength + 1).join(char)
			// This creates padLength repetitions of char
			charArray := stringToArray(char)
			charLen := len(charArray)

			padding := ""
			if charLen == 1 {
				// Single character padding is simple
				padding = strings.Repeat(char, padLength)
			} else {
				// Multi-character padding
				fullRepeats := padLength / charLen
				partialChars := padLength % charLen

				padding = strings.Repeat(char, fullRepeats)
				// Add partial characters if needed
				for i := 0; i < partialChars; i++ {
					padding += charArray[i]
				}
			}

			if width > 0 {
				result = str + padding
			} else {
				result = padding + str
			}
		} else {
			result = str
		}
		return result, nil
	}

	// Forward declaration for recursive function
	var evaluateMatcher func(interface{}, string, *Environment) (*MatchResult, error)
	var containsFunc JSONataFunc
	var matchFunc JSONataFunc
	var replaceFunc JSONataFunc
	var splitFunc JSONataFunc
	var mapFunc JSONataFunc
	var filterFunc JSONataFunc
	var singleFunc JSONataFunc
	var foldLeftFunc JSONataFunc
	var siftFunc JSONataFunc
	var eachFunc JSONataFunc

	/**
	 * Helper function to invoke a function (Lambda or native)
	 * @param {interface{}} fn - the function to invoke
	 * @param {[]interface{}} args - the arguments to pass
	 * @returns {interface{}} result of function invocation
	 */
	invokeFunction := func(fn interface{}, args []interface{}) (interface{}, error) {
		switch f := fn.(type) {
		case *Lambda:
			if f.apply != nil {
				return f.apply(f, args)
			}
			return nil, errors.New("lambda function has no apply method")
		case *Function:
			if f.Implementation != nil {
				if impl, ok := f.Implementation.(JSONataFunc); ok {
					return impl(args)
				} else if impl, ok := f.Implementation.(func(...interface{}) (interface{}, error)); ok {
					return impl(args...)
				}
			}
			return nil, errors.New("function has no implementation")
		case func(...interface{}) (interface{}, error):
			return f(args...)
		default:
			return nil, errors.New("not a function")
		}
	}

	// Helper to convert map result to MatchResult
	var convertMapToMatchResult func(map[string]interface{}) *MatchResult
	convertMapToMatchResult = func(resultMap map[string]interface{}) *MatchResult {
		match, _ := resultMap["match"].(string)

		// Handle both int and float64 for start/end (JavaScript numbers come as float64)
		var start, end int
		if s, ok := resultMap["start"].(int); ok {
			start = s
		} else if s, ok := resultMap["start"].(float64); ok {
			start = int(s)
		}
		if e, ok := resultMap["end"].(int); ok {
			end = e
		} else if e, ok := resultMap["end"].(float64); ok {
			end = int(e)
		}

		groupsInterface, _ := resultMap["groups"].([]interface{})

		groups := []string{}
		for _, g := range groupsInterface {
			if gs, ok := g.(string); ok {
				groups = append(groups, gs)
			}
		}

		var nextFunc func() *MatchResult
		if nextFn, ok := resultMap["next"].(func() (interface{}, error)); ok {
			nextFunc = func() *MatchResult {
				nextResult, err := nextFn()
				if err != nil || nextResult == nil {
					return nil
				}
				if nextMap, ok := nextResult.(map[string]interface{}); ok {
					return convertMapToMatchResult(nextMap)
				}
				return nil
			}
		} else if next := resultMap["next"]; next != nil && isFunction(next) {
			// Handle other function types (like Lambda from custom matchers)
			nextFunc = func() *MatchResult {
				nextResult, err := invokeFunction(next, []interface{}{})
				if err != nil || nextResult == nil {
					return nil
				}
				if nextMap, ok := nextResult.(map[string]interface{}); ok {
					return convertMapToMatchResult(nextMap)
				}
				return nil
			}
		}

		return &MatchResult{
			Match:  match,
			Start:  start,
			End:    end,
			Groups: groups,
			Next:   nextFunc,
		}
	}

	/**
	 * Evaluate the matcher function against the str arg
	 *
	 * @param {interface{}} matcher - matching function (native or lambda)
	 * @param {string} str - the string to match against
	 * @param {*Environment} env - the environment
	 * @returns {*MatchResult} - structure that represents the match(es)
	 */
	evaluateMatcher = func(matcher interface{}, str string, env *Environment) (*MatchResult, error) {
		// For Go implementation, we'll handle regexp.Regexp directly
		if re, ok := matcher.(*regexp.Regexp); ok {
			// Find all matches at once to avoid closure issues
			allMatches := re.FindAllStringSubmatchIndex(str, -1)
			if len(allMatches) == 0 {
				return nil, nil
			}

			// Create a chain of MatchResults
			var buildResult func(int) *MatchResult
			buildResult = func(idx int) *MatchResult {
				if idx >= len(allMatches) {
					return nil
				}

				match := allMatches[idx]
				matchStr := str[match[0]:match[1]]
				groups := []string{}
				// match[2:] contains the capture group indices (pairs of start,end)
				for i := 2; i < len(match); i += 2 {
					if match[i] >= 0 && match[i+1] >= 0 {
						groups = append(groups, str[match[i]:match[i+1]])
					}
				}

				return &MatchResult{
					Match:  matchStr,
					Start:  match[0],
					End:    match[1],
					Groups: groups,
					Next: func() *MatchResult {
						return buildResult(idx + 1)
					},
				}
			}

			return buildResult(0), nil
		}

		// Handle regex closure from evaluateRegex
		if closure, ok := matcher.(func(string, int) interface{}); ok {
			result := closure(str, 0)
			if result == nil {
				return nil, nil
			}

			if resultMap, ok := result.(map[string]interface{}); ok {
				return convertMapToMatchResult(resultMap), nil
			}

			return nil, createJSONataError("T1010", nil)
		}

		// For lambda functions and other callable matchers
		if isFunction(matcher) {
			result, err := invokeFunction(matcher, []interface{}{str})
			if err != nil {
				return nil, err
			}

			// Check if result has the correct structure
			if result == nil {
				return nil, nil
			}

			if resultMap, ok := result.(map[string]interface{}); ok {
				// Validate structure
				var start, end int
				var match string
				var groups []string
				var hasStart, hasEnd bool

				if s, ok := resultMap["start"].(int); ok {
					start = s
					hasStart = true
				} else if s, ok := resultMap["start"].(float64); ok {
					start = int(s)
					hasStart = true
				}
				if e, ok := resultMap["end"].(int); ok {
					end = e
					hasEnd = true
				} else if e, ok := resultMap["end"].(float64); ok {
					end = int(e)
					hasEnd = true
				}
				if m, ok := resultMap["match"].(string); ok {
					match = m
				}
				if g, ok := resultMap["groups"].([]interface{}); ok {
					for _, group := range g {
						if groupStr, ok := group.(string); ok {
							groups = append(groups, groupStr)
						}
					}
				}

				if !hasStart && !hasEnd && len(groups) == 0 && !isFunction(resultMap["next"]) {
					return nil, createJSONataError("T1010", nil)
				}

				return &MatchResult{
					Match:  match,
					Start:  start,
					End:    end,
					Groups: groups,
					Next: func() *MatchResult {
						if nextFunc, ok := resultMap["next"]; ok {
							// Check if it's a function that returns (interface{}, error)
							if fn, ok := nextFunc.(func() (interface{}, error)); ok {
								nextResult, err := fn()
								if err != nil || nextResult == nil {
									return nil
								}
								// Convert the result to MatchResult
								if nextMap, ok := nextResult.(map[string]interface{}); ok {
									return convertMapToMatchResult(nextMap)
								}
							} else if isFunction(nextFunc) {
								// The next function returns a matcher result
								// We need to invoke it with no arguments
								nextResult, err := invokeFunction(nextFunc, []interface{}{})
								if err != nil || nextResult == nil {
									return nil
								}
								// Convert the result to MatchResult
								if nextMap, ok := nextResult.(map[string]interface{}); ok {
									return convertMapToMatchResult(nextMap)
								}
							}
						}
						return nil
					},
				}, nil
			}

			return nil, createJSONataError("T1010", nil)
		}

		return nil, errors.New("matcher must be a regexp or function")
	}

	/**
	 * Tests if the str contains the token
	 * JavaScript equivalent: functions.js lines 342-358
	 * @param {string} str - string to test
	 * @param {interface{}} token - substring or regex to find
	 * @param {*Environment} env - the environment
	 * @returns {bool} - true if str contains token
	 */
	containsFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("contains function requires at least 2 arguments")
		}

		// Extract arguments based on signature "<s-(sf):b>"
		// args[0] = string
		// args[1] = token (string or function)
		str, ok1 := args[0].(string)
		if !ok1 {
			str = ""
		}

		token := args[1]

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		var result bool

		if tokenStr, ok := token.(string); ok {
			result = strings.Contains(str, tokenStr)
		} else {
			// For regex patterns and lambda functions - matches JavaScript: evaluateMatcher(token, str)
			matches, err := evaluateMatcher(token, str, nil)
			if err != nil {
				return nil, err
			}
			result = matches != nil
		}

		return result, nil
	}

	/**
	 * Match a string with a regex returning an array of object containing details of each match
	 * JavaScript equivalent: functions.js lines 367-402
	 * @param {string} str - string
	 * @param {interface{}} regex - the regex applied to the string
	 * @param {int} limit - max number of matches to return
	 * @param {*Environment} env - the environment
	 * @returns {[]interface{}} The array of match objects
	 */
	matchFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("match function requires at least 2 arguments")
		}

		// Extract arguments based on signature "<s-f<s:o>n?:a<o>>"
		// args[0] = string
		// args[1] = regex function
		// args[2] = limit (optional)
		str, ok1 := args[0].(string)
		if !ok1 {
			str = ""
		}

		regex := args[1]

		var limit *int
		if len(args) >= 3 {
			if args[2] != nil {
				if limitVal, ok := args[2].(int); ok {
					limit = &limitVal
				} else if limitVal, ok := args[2].(float64); ok {
					intVal := int(limitVal)
					limit = &intVal
				}
			}
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		// limit, if specified, must be a non-negative number
		if limit != nil && *limit < 0 {
			return nil, &JSONataError{
				Code:  "D3040",
				Value: *limit,
				Index: 3,
			}
		}

		result := createSequence()

		if limit == nil || *limit > 0 {
			count := 0
			matches, err := evaluateMatcher(regex, str, nil)
			if err != nil {
				return nil, err
			}
			if matches != nil {
				for matches != nil && (limit == nil || count < *limit) {
					groups := matches.Groups
					if groups == nil {
						groups = []string{}
					}
					result.Data = append(result.Data, map[string]interface{}{
						"match":  matches.Match,
						"index":  matches.Start,
						"groups": groups,
					})
					count++
					if matches.Next != nil {
						matches = matches.Next()
					} else {
						matches = nil
					}
				}
			}
		}

		return result.Data, nil
	}

	/**
	 * Process replacement string handling $0, $1, $2, etc. substitutions
	 * @param {string} replacement - the replacement string with $n patterns
	 * @param {*MatchResult} matches - the match result containing groups
	 * @returns {string} processed replacement string
	 */
	processReplacementString := func(replacement string, matches *MatchResult) string {
		substitute := ""
		position := 0
		index := strings.Index(replacement[position:], "$")
		for index != -1 && position < len(replacement) {
			index += position
			substitute += replacement[position:index]
			position = index + 1
			if position < len(replacement) {
				dollarVal := replacement[position]
				if dollarVal == '$' {
					// literal $
					substitute += "$"
					position++
				} else if dollarVal >= '0' && dollarVal <= '9' {
					if len(matches.Groups) == 0 {
						// No sub-matches; any $ followed by a digit will be replaced by an empty string
						position++ // Skip the digit
					} else {
						// JavaScript's maxDigits logic: Math.floor(Math.log(groups.length) * Math.LOG10E) + 1
						maxDigits := len(strconv.Itoa(len(matches.Groups)))

						// Parse up to maxDigits
						digitStart := position
						digitEnd := position
						for i := 0; i < maxDigits && digitEnd < len(replacement) && replacement[digitEnd] >= '0' && replacement[digitEnd] <= '9'; i++ {
							digitEnd++
						}

						groupNum, _ := strconv.Atoi(replacement[digitStart:digitEnd])
						// If maxDigits > 1 and index > groups.length, try with one less digit
						if maxDigits > 1 && groupNum > len(matches.Groups) && digitEnd > digitStart+1 {
							digitEnd--
							groupNum, _ = strconv.Atoi(replacement[digitStart:digitEnd])
						}

						if groupNum == 0 {
							substitute += matches.Match
							position = digitEnd
						} else if groupNum > 0 && groupNum <= len(matches.Groups) {
							substitute += matches.Groups[groupNum-1]
							position = digitEnd
						} else {
							// Group doesn't exist, JavaScript outputs empty string
							// Still advance position to consume the digits
							position = digitEnd
						}
					}
				} else {
					// not a capture group, treat the $ as literal
					substitute += "$"
				}
			} else {
				substitute += "$"
			}
			if position < len(replacement) {
				index = strings.Index(replacement[position:], "$")
			} else {
				break
			}
		}
		substitute += replacement[position:]
		return substitute
	}

	/**
	 * Replace a string with a regex
	 * JavaScript equivalent: functions.js lines 412-543
	 * @param {string} str - string
	 * @param {interface{}} pattern - the substring/regex applied to the string
	 * @param {interface{}} replacement - text to replace the matched substrings
	 * @param {int} limit - max number of matches to replace
	 * @param {*Environment} env - the environment
	 * @returns {string} The replaced string
	 */
	replaceFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 3 {
			return nil, errors.New("replace function requires at least 3 arguments")
		}

		// Extract arguments based on signature "<s-(sf)(sf)n?:s>"
		// args[0] = string (input)
		// args[1] = pattern (string or function)
		// args[2] = replacement (string or function)
		// args[3] = limit (optional number)
		str, ok1 := args[0].(string)
		if !ok1 {
			str = ""
		}

		pattern := args[1]
		replacement := args[2]

		var limit *int
		if len(args) >= 4 && args[3] != nil {
			if limitVal, ok := args[3].(int); ok {
				limit = &limitVal
			} else if limitVal, ok := args[3].(float64); ok {
				intVal := int(limitVal)
				limit = &intVal
			}
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		// pattern cannot be an empty string
		if patternStr, ok := pattern.(string); ok && patternStr == "" {
			return nil, &JSONataError{
				Code:  "D3010",
				Value: pattern,
				Index: 2,
			}
		}

		// limit, if specified, must be a non-negative number
		if limit != nil && *limit < 0 {
			return nil, &JSONataError{
				Code:  "D3011",
				Value: *limit,
				Index: 4,
			}
		}

		result := ""
		position := 0

		if limit == nil || *limit > 0 {
			count := 0
			if patternStr, ok := pattern.(string); ok {
				// Simple string replacement
				if replacementStr, ok := replacement.(string); ok {
					for {
						index := strings.Index(str[position:], patternStr)
						if index == -1 || (limit != nil && count >= *limit) {
							break
						}
						index += position
						result += str[position:index]
						result += replacementStr
						position = index + len(patternStr)
						count++
					}
					result += str[position:]
				}
			} else {
				// Regex replacement - matches JavaScript: evaluateMatcher(pattern, str)
				// First, we need to test if this regex would match zero-length strings
				// by invoking it and checking the first match
				testResult, err := invokeFunction(pattern, []interface{}{str})
				if err != nil {
					return nil, err
				}

				// Check if we got a match result map
				if testResultMap, ok := testResult.(map[string]interface{}); ok {
					// Check if it's a zero-length match
					if match, hasMatch := testResultMap["match"].(string); hasMatch && match == "" {
						// Try to get the next match to see if it would progress
						if nextFn, hasNext := testResultMap["next"].(func() (interface{}, error)); hasNext {
							nextResult, nextErr := nextFn()
							// If we get a D1004 error, propagate it
							if nextErr != nil {
								return nil, nextErr
							}
							// If the next match is also zero-length at the same position, it would loop forever
							if nextMap, ok := nextResult.(map[string]interface{}); ok {
								if nextMatch, hasMatch := nextMap["match"].(string); hasMatch && nextMatch == "" {
									if nextStart, hasStart := nextMap["start"].(int); hasStart && nextStart == 0 {
										// This would cause infinite loop
										return nil, createJSONataError("D1004", pattern)
									}
								}
							}
						}
					}
				}

				// Now do the actual replacement
				matches, err := evaluateMatcher(pattern, str, nil)
				if err != nil {
					return nil, err
				}
				if matches != nil {

					for matches != nil && (limit == nil || count < *limit) {
						result += str[position:matches.Start]
						if replacementStr, ok := replacement.(string); ok {
							// Process replacement string with $0, $1, $2, etc.
							replacedWith := processReplacementString(replacementStr, matches)
							result += replacedWith
						} else if isFunction(replacement) {
							// Lambda replacement functions - matches JavaScript: replacement.apply(this, [matches])
							// Convert MatchResult to the format expected by JavaScript
							// Convert []string to []interface{} for groups
							groups := make([]interface{}, len(matches.Groups))
							for i, g := range matches.Groups {
								groups[i] = g
							}
							matchObj := map[string]interface{}{
								"match":  matches.Match,
								"index":  matches.Start,
								"groups": groups,
							}
							res, err := invokeFunction(replacement, []interface{}{matchObj})
							if err != nil {
								return nil, err
							}
							if resStr, ok := res.(string); ok {
								result += resStr
							} else {
								return nil, createJSONataError("D3012", res)
							}
						}
						position = matches.End
						count++
						matches = matches.Next()
					}
					result += str[position:]
				} else {
					result = str
				}
			}
		} else {
			result = str
		}

		return result, nil
	}

	/**
	 * Base64 encode a string
	 * JavaScript equivalent: functions.js lines 550-566
	 * @param {string} str - string
	 * @returns {string} Base 64 encoding of the binary data
	 */
	base64encodeFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}
		return base64.StdEncoding.EncodeToString([]byte(str)), nil
	}

	/**
	 * Base64 decode a string
	 * JavaScript equivalent: functions.js lines 573-588
	 * @param {string} str - string
	 * @returns {string} Base 64 decoding of the string
	 */
	base64decodeFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}
		bytes, err := base64.StdEncoding.DecodeString(str)
		if err != nil {
			return nil, err
		}
		return string(bytes), nil
	}

	/**
	 * Encode a string into a component for a url
	 * JavaScript equivalent: functions.js lines 595-614
	 * @param {string} str - String to encode
	 * @returns {string} Encoded string
	 */
	encodeUrlComponentFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		return url.QueryEscape(str), nil
	}

	/**
	 * Encode a string into a url
	 * JavaScript equivalent: functions.js lines 621-640
	 * @param {string} str - String to encode
	 * @returns {string} Encoded string
	 */
	encodeUrlFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		// JavaScript's encodeURI has specific behavior:
		// - It encodes all non-ASCII characters
		// - It preserves certain URL delimiters like :/?#[]@!$&'()*+,;=
		// We'll implement a custom encoder that matches JavaScript's behavior

		result := ""
		for _, r := range str {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
				r == '-' || r == '_' || r == '.' || r == '~' || r == '/' || r == ':' ||
				r == '?' || r == '#' || r == '[' || r == ']' || r == '@' || r == '!' ||
				r == '$' || r == '&' || r == '\'' || r == '(' || r == ')' || r == '*' ||
				r == '+' || r == ',' || r == ';' || r == '=' {
				// These characters are not encoded by encodeURI
				result += string(r)
			} else {
				// Encode the character
				bytes := []byte(string(r))
				for _, b := range bytes {
					result += fmt.Sprintf("%%%02X", b)
				}
			}
		}

		return result, nil
	}

	/**
	 * Decode a string from a component for a url
	 * JavaScript equivalent: functions.js lines 647-666
	 * @param {string} str - String to decode
	 * @returns {string} Decoded string
	 */
	decodeUrlComponentFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		decoded, err := url.QueryUnescape(str)
		if err != nil {
			return nil, &JSONataError{
				Code:  "D3140",
				Value: str,
			}
		}
		return decoded, nil
	}

	/**
	 * Decode a string from a url
	 * JavaScript equivalent: functions.js lines 673-692
	 * @param {string} str - String to decode
	 * @returns {string} Decoded string
	 */
	decodeUrlFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the string
		if len(args) == 0 {
			return nil, nil
		}

		str := ""
		if s, ok := args[0].(string); ok {
			str = s
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		// This is a simplified implementation
		decoded, err := url.QueryUnescape(str)
		if err != nil {
			return nil, &JSONataError{
				Code:  "D3140",
				Value: str,
			}
		}
		return decoded, nil
	}

	/**
	 * Split a string into an array of substrings
	 * JavaScript equivalent: functions.js lines 701-743
	 * @param {string} str - string
	 * @param {interface{}} separator - the token or regex that splits the string
	 * @param {int} limit - max number of substrings
	 * @param {*Environment} env - the environment
	 * @returns {[]string} The array of strings
	 */
	splitFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("split function requires at least 2 arguments")
		}

		// Extract arguments based on signature "<s-(sf)n?:a<s>>"
		// args[0] = string (input)
		// args[1] = separator (string or function)
		// args[2] = limit (optional number)
		str, ok1 := args[0].(string)
		if !ok1 {
			str = ""
		}

		separator := args[1]

		var limit *int
		if len(args) >= 3 && args[2] != nil {
			if limitVal, ok := args[2].(int); ok {
				limit = &limitVal
			} else if limitVal, ok := args[2].(float64); ok {
				intVal := int(limitVal)
				limit = &intVal
			}
		}

		// undefined inputs always return undefined
		if str == "" {
			return nil, nil
		}

		// limit, if specified, must be a non-negative number
		if limit != nil && *limit < 0 {
			return nil, &JSONataError{
				Code:  "D3020",
				Value: *limit,
				Index: 3,
			}
		}

		result := []interface{}{}

		if limit == nil || *limit > 0 {
			if sepStr, ok := separator.(string); ok {
				if limit != nil {
					// JavaScript split with limit returns first N elements, not N splits
					// Use Split to get all parts, then take first limit elements
					parts := strings.Split(str, sepStr)
					for i, part := range parts {
						if i >= *limit {
							break
						}
						result = append(result, part)
					}
				} else {
					parts := strings.Split(str, sepStr)
					for _, part := range parts {
						result = append(result, part)
					}
				}
			} else {
				// Regex split - matches JavaScript: evaluateMatcher(separator, str)
				count := 0
				matches, err := evaluateMatcher(separator, str, nil)
				if err != nil {
					return nil, err
				}
				if matches != nil {
					start := 0
					for matches != nil && (limit == nil || count < *limit) {
						result = append(result, str[start:matches.Start])
						start = matches.End
						matches = matches.Next()
						count++
					}
					if limit == nil || count < *limit {
						result = append(result, str[start:])
					}
				} else {
					result = append(result, str)
				}
			}
		}

		return result, nil
	}

	/**
	 * Join an array of strings
	 * JavaScript equivalent: functions.js lines 751-763
	 * @param {[]interface{}} strs - array of string
	 * @param {string} separator - the token that splits the string
	 * @returns {string} The concatenated string
	 */
	joinFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}

		// First argument should be the array to join
		strs, ok := args[0].([]interface{})
		if !ok {
			if args[0] == nil {
				return nil, nil
			}
			// If not an array, treat as single element
			strs = []interface{}{args[0]}
		}

		// Second argument is separator (optional, defaults to empty string)
		separator := ""
		if len(args) > 1 && args[1] != nil {
			separator = fmt.Sprintf("%v", args[1])
		}

		strSlice := []string{}
		for _, s := range strs {
			if str, ok := s.(string); ok {
				strSlice = append(strSlice, str)
			}
		}
		return strings.Join(strSlice, separator), nil
	}

	/**
	 * Formats a number into a decimal string representation using XPath 3.1 F&O fn:format-number spec
	 * JavaScript equivalent: functions.js lines 772-1152
	 * @param {float64} value - number to format
	 * @param {string} picture - picture string definition
	 * @param {map[string]string} options - override locale defaults
	 * @returns {string} The formatted string
	 */
	formatNumberFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}

		// Extract value (required)
		// undefined inputs always return undefined
		if args[0] == nil {
			return nil, nil
		}

		var value float64
		if v, ok := args[0].(float64); ok {
			value = v
		} else if v, ok := args[0].(int); ok {
			value = float64(v)
		} else {
			return nil, fmt.Errorf("value must be a number")
		}

		// Extract picture (required)
		var picture string
		if len(args) > 1 && args[1] != nil {
			picture = fmt.Sprintf("%v", args[1])
		} else {
			return nil, fmt.Errorf("picture string is required")
		}

		// Extract options (optional)
		var options map[string]string
		if len(args) > 2 && args[2] != nil {
			// Handle both map[string]string and map[string]interface{}
			switch opts := args[2].(type) {
			case map[string]string:
				options = opts
			case map[string]interface{}:
				options = make(map[string]string)
				for k, v := range opts {
					if str, ok := v.(string); ok {
						options[k] = str
					} else {
						options[k] = fmt.Sprintf("%v", v)
					}
				}
			}
		}
		// undefined inputs always return undefined
		if math.IsNaN(value) {
			return nil, nil
		}

		// Default properties matching JavaScript implementation (lines 778-790)
		defaults := map[string]string{
			"decimal-separator":  ".",
			"grouping-separator": ",",
			"exponent-separator": "e",
			"infinity":           "Infinity",
			"minus-sign":         "-",
			"NaN":                "NaN",
			"percent":            "%",
			"per-mille":          "\u2030",
			"zero-digit":         "0",
			"digit":              "#",
			"pattern-separator":  ";",
		}

		// if `options` is specified, then its entries override defaults (lines 792-798)
		properties := make(map[string]string)
		for k, v := range defaults {
			properties[k] = v
		}
		for k, v := range options {
			properties[k] = v
		}

		// Build decimal digit family (lines 800-804)
		var decimalDigitFamily []string
		zeroCharCode := int([]rune(properties["zero-digit"])[0])
		for ii := zeroCharCode; ii < zeroCharCode+10; ii++ {
			decimalDigitFamily = append(decimalDigitFamily, string(rune(ii)))
		}

		// Build active chars (line 806)
		activeChars := append(decimalDigitFamily,
			properties["decimal-separator"],
			properties["exponent-separator"],
			properties["grouping-separator"],
			properties["digit"],
			properties["pattern-separator"])

		// Split sub-pictures (line 808)
		subPictures := strings.Split(picture, properties["pattern-separator"])

		// Validate sub-pictures length (lines 810-815)
		if len(subPictures) > 2 {
			return nil, &JSONataError{
				Code: "D3080",
			}
		}

		// Picture part structure
		type PictureParts struct {
			prefix         string
			suffix         string
			activePart     string
			mantissaPart   string
			exponentPart   string
			integerPart    string
			fractionalPart string
			subpicture     string
		}

		// splitParts function (lines 817-864)
		splitParts := func(subpicture string) *PictureParts {
			// Find prefix
			prefix := ""
			runes := []rune(subpicture)
			for ii := 0; ii < len(runes); ii++ {
				ch := string(runes[ii])
				found := false
				for _, ac := range activeChars {
					if ch == ac && ch != properties["exponent-separator"] {
						found = true
						break
					}
				}
				if found {
					prefix = string(runes[:ii])
					break
				}
			}

			// Find suffix
			suffix := ""
			for ii := len(runes) - 1; ii >= 0; ii-- {
				ch := string(runes[ii])
				found := false
				for _, ac := range activeChars {
					if ch == ac && ch != properties["exponent-separator"] {
						found = true
						break
					}
				}
				if found {
					suffix = string(runes[ii+1:])
					break
				}
			}

			// Calculate activePart correctly using rune indices
			prefixLen := len([]rune(prefix))
			suffixLen := len([]rune(suffix))
			activePart := string(runes[prefixLen : len(runes)-suffixLen])

			// Find exponent part
			var mantissaPart, exponentPart string
			exponentPosition := strings.Index(activePart, properties["exponent-separator"])
			if exponentPosition == -1 {
				mantissaPart = activePart
				exponentPart = ""
			} else {
				mantissaPart = activePart[:exponentPosition]
				exponentPart = activePart[exponentPosition+len(properties["exponent-separator"]):]
			}

			// Find integer and fractional parts
			var integerPart, fractionalPart string
			decimalPosition := strings.Index(mantissaPart, properties["decimal-separator"])
			if decimalPosition == -1 {
				integerPart = mantissaPart
				fractionalPart = suffix
			} else {
				integerPart = mantissaPart[:decimalPosition]
				fractionalPart = mantissaPart[decimalPosition+1:]
			}

			return &PictureParts{
				prefix:         prefix,
				suffix:         suffix,
				activePart:     activePart,
				mantissaPart:   mantissaPart,
				exponentPart:   exponentPart,
				integerPart:    integerPart,
				fractionalPart: fractionalPart,
				subpicture:     subpicture,
			}
		}

		// validate function (lines 866-938)
		validate := func(parts *PictureParts) error {
			subpicture := parts.subpicture
			decimalPos := strings.Index(subpicture, properties["decimal-separator"])

			// Multiple decimal separators
			if decimalPos != strings.LastIndex(subpicture, properties["decimal-separator"]) {
				return &JSONataError{Code: "D3081"}
			}

			// Multiple percent signs
			if strings.Index(subpicture, properties["percent"]) != strings.LastIndex(subpicture, properties["percent"]) {
				return &JSONataError{Code: "D3082"}
			}

			// Multiple per-mille signs
			if strings.Index(subpicture, properties["per-mille"]) != strings.LastIndex(subpicture, properties["per-mille"]) {
				return &JSONataError{Code: "D3083"}
			}

			// Both percent and per-mille
			if strings.Contains(subpicture, properties["percent"]) && strings.Contains(subpicture, properties["per-mille"]) {
				return &JSONataError{Code: "D3084"}
			}

			// Must contain at least one digit character
			valid := false
			for _, ch := range parts.mantissaPart {
				chStr := string(ch)
				found := false
				for _, digit := range decimalDigitFamily {
					if chStr == digit {
						found = true
						break
					}
				}
				if found || chStr == properties["digit"] {
					valid = true
					break
				}
			}
			if !valid {
				return &JSONataError{Code: "D3085"}
			}

			// Check for invalid characters in active part
			for _, ch := range parts.activePart {
				chStr := string(ch)
				found := false
				for _, ac := range activeChars {
					if chStr == ac {
						found = true
						break
					}
				}
				if !found {
					return &JSONataError{Code: "D3086"}
				}
			}

			// Grouping separator adjacent to decimal separator
			if decimalPos != -1 {
				if (decimalPos > 0 && string(subpicture[decimalPos-1]) == properties["grouping-separator"]) ||
					(decimalPos < len(subpicture)-1 && string(subpicture[decimalPos+1]) == properties["grouping-separator"]) {
					return &JSONataError{Code: "D3087"}
				}
			} else if len(parts.integerPart) > 0 && string(parts.integerPart[len(parts.integerPart)-1]) == properties["grouping-separator"] {
				return &JSONataError{Code: "D3088"}
			}

			// Adjacent grouping separators
			if strings.Contains(subpicture, properties["grouping-separator"]+properties["grouping-separator"]) {
				return &JSONataError{Code: "D3089"}
			}

			// Optional digit validation for integer part
			optionalDigitPos := strings.Index(parts.integerPart, properties["digit"])
			if optionalDigitPos != -1 {
				count := 0
				for _, ch := range parts.integerPart[:optionalDigitPos] {
					chStr := string(ch)
					for _, digit := range decimalDigitFamily {
						if chStr == digit {
							count++
							break
						}
					}
				}
				if count > 0 {
					return &JSONataError{Code: "D3090"}
				}
			}

			// Optional digit validation for fractional part
			optionalDigitPos = strings.LastIndex(parts.fractionalPart, properties["digit"])
			if optionalDigitPos != -1 {
				count := 0
				for _, ch := range parts.fractionalPart[optionalDigitPos:] {
					chStr := string(ch)
					for _, digit := range decimalDigitFamily {
						if chStr == digit {
							count++
							break
						}
					}
				}
				if count > 0 {
					return &JSONataError{Code: "D3091"}
				}
			}

			// Exponent validation
			exponentExists := parts.exponentPart != ""
			if exponentExists && len(parts.exponentPart) > 0 && (strings.Contains(subpicture, properties["percent"]) || strings.Contains(subpicture, properties["per-mille"])) {
				return &JSONataError{Code: "D3092"}
			}

			if exponentExists {
				if len(parts.exponentPart) == 0 {
					return &JSONataError{Code: "D3093"}
				}
				for _, ch := range parts.exponentPart {
					chStr := string(ch)
					found := false
					for _, digit := range decimalDigitFamily {
						if chStr == digit {
							found = true
							break
						}
					}
					if !found {
						return &JSONataError{Code: "D3093"}
					}
				}
			}

			return nil
		}

		// Analysis variables structure
		type AnalysisVars struct {
			integerPartGroupingPositions    []int
			regularGrouping                 int
			minimumIntegerPartSize          int
			scalingFactor                   int
			prefix                          string
			fractionalPartGroupingPositions []int
			minimumFractionalPartSize       int
			maximumFractionalPartSize       int
			minimumExponentSize             int
			suffix                          string
			picture                         string
		}

		// analyse function (lines 940-1024)
		analyse := func(parts *PictureParts) *AnalysisVars {
			// Helper function for grouping positions
			getGroupingPositions := func(part string, toLeft bool) []int {
				var positions []int
				groupingPosition := strings.Index(part, properties["grouping-separator"])
				for groupingPosition != -1 {
					var charsToTheRight int
					var checkPart string
					if toLeft {
						checkPart = part[:groupingPosition]
					} else {
						checkPart = part[groupingPosition:]
					}
					for _, ch := range checkPart {
						chStr := string(ch)
						found := false
						for _, digit := range decimalDigitFamily {
							if chStr == digit {
								found = true
								break
							}
						}
						if found || chStr == properties["digit"] {
							charsToTheRight++
						}
					}
					positions = append(positions, charsToTheRight)
					groupingPosition = strings.Index(parts.integerPart[groupingPosition+1:], properties["grouping-separator"])
					if groupingPosition != -1 {
						groupingPosition += strings.Index(parts.integerPart, properties["grouping-separator"]) + 1
					}
				}
				return positions
			}

			integerPartGroupingPositions := getGroupingPositions(parts.integerPart, false)

			// GCD function for regular grouping
			gcd := func(a, b int) int {
				for b != 0 {
					a, b = b, a%b
				}
				return a
			}

			// Calculate regular grouping
			regular := func(indexes []int) int {
				if len(indexes) == 0 {
					return 0
				}
				factor := indexes[0]
				for _, idx := range indexes[1:] {
					factor = gcd(factor, idx)
				}
				for index := 1; index <= len(indexes); index++ {
					found := false
					for _, idx := range indexes {
						if idx == index*factor {
							found = true
							break
						}
					}
					if !found {
						return 0
					}
				}
				return factor
			}

			regularGrouping := regular(integerPartGroupingPositions)
			fractionalPartGroupingPositions := getGroupingPositions(parts.fractionalPart, true)

			// Count minimum integer part size
			minimumIntegerPartSize := 0
			for _, ch := range parts.integerPart {
				chStr := string(ch)
				for _, digit := range decimalDigitFamily {
					if chStr == digit {
						minimumIntegerPartSize++
						break
					}
				}
			}
			scalingFactor := minimumIntegerPartSize

			// Count fractional part sizes
			minimumFractionalPartSize := 0
			maximumFractionalPartSize := 0
			for _, ch := range parts.fractionalPart {
				chStr := string(ch)
				found := false
				for _, digit := range decimalDigitFamily {
					if chStr == digit {
						minimumFractionalPartSize++
						maximumFractionalPartSize++
						found = true
						break
					}
				}
				if !found && chStr == properties["digit"] {
					maximumFractionalPartSize++
				}
			}

			exponentPresent := parts.exponentPart != ""
			if minimumIntegerPartSize == 0 && maximumFractionalPartSize == 0 {
				if exponentPresent {
					minimumFractionalPartSize = 1
					maximumFractionalPartSize = 1
				} else {
					minimumIntegerPartSize = 1
				}
			}
			if exponentPresent && minimumIntegerPartSize == 0 && strings.Contains(parts.integerPart, properties["digit"]) {
				minimumIntegerPartSize = 1
			}
			if minimumIntegerPartSize == 0 && minimumFractionalPartSize == 0 {
				minimumFractionalPartSize = 1
			}

			minimumExponentSize := 0
			if exponentPresent {
				for _, ch := range parts.exponentPart {
					chStr := string(ch)
					for _, digit := range decimalDigitFamily {
						if chStr == digit {
							minimumExponentSize++
							break
						}
					}
				}
			}

			return &AnalysisVars{
				integerPartGroupingPositions:    integerPartGroupingPositions,
				regularGrouping:                 regularGrouping,
				minimumIntegerPartSize:          minimumIntegerPartSize,
				scalingFactor:                   scalingFactor,
				prefix:                          parts.prefix,
				fractionalPartGroupingPositions: fractionalPartGroupingPositions,
				minimumFractionalPartSize:       minimumFractionalPartSize,
				maximumFractionalPartSize:       maximumFractionalPartSize,
				minimumExponentSize:             minimumExponentSize,
				suffix:                          parts.suffix,
				picture:                         parts.subpicture,
			}
		}

		// Process all sub-pictures
		var parts []*PictureParts
		for _, sp := range subPictures {
			parts = append(parts, splitParts(sp))
		}

		// Validate all parts
		for _, part := range parts {
			if err := validate(part); err != nil {
				return nil, err
			}
		}

		// Analyse all parts
		var variables []*AnalysisVars
		for _, part := range parts {
			variables = append(variables, analyse(part))
		}

		minusSign := properties["minus-sign"]
		zeroDigit := properties["zero-digit"]
		decimalSeparator := properties["decimal-separator"]
		groupingSeparator := properties["grouping-separator"]

		// If only one pattern, create negative pattern
		if len(variables) == 1 {
			negPattern := *variables[0] // Copy
			negPattern.prefix = minusSign + negPattern.prefix
			variables = append(variables, &negPattern)
		}

		// Choose pattern based on value sign
		var pic *AnalysisVars
		if value >= 0 {
			pic = variables[0]
		} else {
			pic = variables[1]
		}

		// Adjust number for percent/per-mille
		adjustedNumber := value
		if strings.Contains(pic.picture, properties["percent"]) {
			adjustedNumber = value * 100
		} else if strings.Contains(pic.picture, properties["per-mille"]) {
			adjustedNumber = value * 1000
		}

		// Handle exponent
		var mantissa, exponent float64
		if pic.minimumExponentSize == 0 {
			mantissa = adjustedNumber
		} else {
			maxMantissa := math.Pow(10, float64(pic.scalingFactor))
			minMantissa := math.Pow(10, float64(pic.scalingFactor-1))
			mantissa = adjustedNumber
			exponent = 0
			for mantissa < minMantissa {
				mantissa *= 10
				exponent--
			}
			for mantissa >= maxMantissa {
				mantissa /= 10
				exponent++
			}
		}

		// Round the mantissa using JavaScript-compatible rounding (lines 1290-1318)
		precision := pic.maximumFractionalPartSize
		rounded := mantissa

		if precision != 0 {
			// Shift decimal place for rounding - use JavaScript's approach to match floating point behavior
			// JavaScript: multiplying introduces floating point precision errors that affect rounding
			multiplier := math.Pow(10, float64(precision))
			rounded = rounded * multiplier

			// Simulate JavaScript floating point precision by converting through string
			// This matches JavaScript's internal representation and rounding behavior
			str := fmt.Sprintf("%.15g", rounded)
			rounded, _ = strconv.ParseFloat(str, 64)
		}

		// Round to nearest integer with banker's rounding
		result := math.Round(rounded)
		diff := result - rounded
		if math.Abs(diff) == 0.5 && int(math.Abs(result))%2 == 1 {
			// Rounded the wrong way - adjust to nearest even number
			if result > rounded {
				result = result - 1
			} else {
				result = result + 1
			}
		}

		if precision != 0 {
			// Shift back
			divisor := math.Pow(10, float64(precision))
			result = result / divisor
		}

		if result == -0 {
			// JSON doesn't do -0
			result = 0
		}
		rounded = result

		// Convert to string
		makeString := func(val float64, dp int) string {
			str := fmt.Sprintf("%."+strconv.Itoa(dp)+"f", math.Abs(val))
			if zeroDigit != "0" {
				var result strings.Builder
				for _, ch := range str {
					if ch >= '0' && ch <= '9' {
						digitIndex := int(ch - '0')
						result.WriteString(decimalDigitFamily[digitIndex])
					} else {
						result.WriteRune(ch)
					}
				}
				str = result.String()
			}
			return str
		}

		stringValue := makeString(rounded, pic.maximumFractionalPartSize)

		// Replace decimal point
		decimalPos := strings.Index(stringValue, ".")
		if decimalPos == -1 {
			stringValue = stringValue + decimalSeparator
		} else {
			stringValue = strings.Replace(stringValue, ".", decimalSeparator, 1)
		}

		// Trim leading zeros (JavaScript lines 1105-1106)
		for len(stringValue) > 0 && string(stringValue[0]) == zeroDigit {
			stringValue = stringValue[1:]
		}
		// Trim trailing zeros (JavaScript lines 1108-1110)
		for len(stringValue) > 0 && string(stringValue[len(stringValue)-1]) == zeroDigit {
			stringValue = stringValue[:len(stringValue)-1]
		}

		// Pad as needed
		decimalPos = strings.Index(stringValue, decimalSeparator)
		padLeft := pic.minimumIntegerPartSize - decimalPos
		padRight := pic.minimumFractionalPartSize - (len(stringValue) - decimalPos - 1)

		if padLeft > 0 {
			stringValue = strings.Repeat(zeroDigit, padLeft) + stringValue
		}
		if padRight > 0 {
			stringValue = stringValue + strings.Repeat(zeroDigit, padRight)
		}

		decimalPos = strings.Index(stringValue, decimalSeparator)

		// Add grouping separators for integer part
		if pic.regularGrouping > 0 {
			groupCount := (decimalPos - 1) / pic.regularGrouping
			for group := 1; group <= groupCount; group++ {
				pos := decimalPos - group*pic.regularGrouping
				stringValue = stringValue[:pos] + groupingSeparator + stringValue[pos:]
			}
		} else {
			for _, pos := range pic.integerPartGroupingPositions {
				insertPos := decimalPos - pos
				if insertPos >= 0 && insertPos < decimalPos {
					stringValue = stringValue[:insertPos] + groupingSeparator + stringValue[insertPos:]
					decimalPos++
				}
			}
		}

		// Add grouping separators for fractional part
		decimalPos = strings.Index(stringValue, decimalSeparator)
		for _, pos := range pic.fractionalPartGroupingPositions {
			insertPos := pos + decimalPos + 1
			if insertPos < len(stringValue) {
				stringValue = stringValue[:insertPos] + groupingSeparator + stringValue[insertPos:]
			}
		}

		// Remove decimal separator if not needed
		decimalPos = strings.Index(stringValue, decimalSeparator)
		if !strings.Contains(pic.picture, decimalSeparator) || decimalPos == len(stringValue)-1 {
			if decimalPos != -1 {
				stringValue = stringValue[:decimalPos] + stringValue[decimalPos+1:]
			}
		}

		// Add exponent if needed
		if pic.minimumExponentSize > 0 {
			stringExponent := makeString(exponent, 0)
			// Remove decimal part from exponent
			if dotPos := strings.Index(stringExponent, "."); dotPos != -1 {
				stringExponent = stringExponent[:dotPos]
			}
			padLeft := pic.minimumExponentSize - len(stringExponent)
			if padLeft > 0 {
				stringExponent = strings.Repeat(zeroDigit, padLeft) + stringExponent
			}
			sign := ""
			if exponent < 0 {
				sign = minusSign
			}
			stringValue = stringValue + properties["exponent-separator"] + sign + stringExponent
		}

		// Add prefix and suffix
		stringValue = pic.prefix + stringValue + pic.suffix

		return stringValue, nil
	}

	/**
	 * Converts a number to a string using a specified number base
	 * JavaScript equivalent: functions.js lines 1160-1186
	 * @param {float64} value - the number to convert
	 * @param {int} radix - the number base; must be between 2 and 36. Defaults to 10
	 * @returns {string} - the converted string
	 */
	formatBaseFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: value, optional radix
		if len(args) == 0 {
			return nil, nil
		}

		value := 0.0
		if v, ok := args[0].(float64); ok {
			value = v
		} else if v, ok := args[0].(int); ok {
			value = float64(v)
		} else {
			// Non-numeric input returns undefined
			return nil, nil
		}

		var radix *int
		if len(args) > 1 {
			if r, ok := args[1].(float64); ok {
				val := int(r)
				radix = &val
			} else if r, ok := args[1].(int); ok {
				radix = &r
			}
		}

		// undefined inputs always return undefined
		if math.IsNaN(value) {
			return nil, nil
		}

		value = math.Round(value)

		base := 10
		if radix != nil {
			base = int(math.Round(float64(*radix)))
		}

		if base < 2 || base > 36 {
			return nil, &JSONataError{
				Code:  "D3100",
				Value: base,
			}
		}

		return strconv.FormatInt(int64(value), base), nil
	}

	/**
	 * Cast argument to number
	 * JavaScript equivalent: functions.js lines 1193-1223
	 * @param {interface{}} arg - Argument
	 * @returns {float64} numeric value of argument
	 */
	numberFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the value to convert
		if len(args) == 0 {
			return nil, nil
		}

		arg := args[0]

		// undefined inputs always return undefined
		if arg == nil {
			return nil, nil
		}

		switch v := arg.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case string:
			// Try to parse as float
			if num, err := strconv.ParseFloat(v, 64); err == nil {
				return num, nil
			}
			// Try to parse as int (hex, octal, binary)
			if strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "0X") {
				if num, err := strconv.ParseInt(v[2:], 16, 64); err == nil {
					return float64(num), nil
				}
			} else if strings.HasPrefix(v, "0o") || strings.HasPrefix(v, "0O") {
				if num, err := strconv.ParseInt(v[2:], 8, 64); err == nil {
					return float64(num), nil
				}
			} else if strings.HasPrefix(v, "0b") || strings.HasPrefix(v, "0B") {
				if num, err := strconv.ParseInt(v[2:], 2, 64); err == nil {
					return float64(num), nil
				}
			}
		case bool:
			if v {
				return 1.0, nil
			}
			return 0.0, nil
		case *JSONataNull:
			// JavaScript number(null) throws an error
			return nil, &JSONataError{
				Code:  "D3030",
				Value: arg,
				Index: 1,
			}
		}

		return nil, &JSONataError{
			Code:  "D3030",
			Value: arg,
			Index: 1,
		}
	}

	/**
	 * Absolute value of a number
	 * JavaScript equivalent: functions.js lines 1230-1240
	 * @param {float64} arg - Argument
	 * @returns {float64} absolute value of argument
	 */
	absFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the number
		if len(args) == 0 {
			return nil, nil
		}

		arg := 0.0
		if a, ok := args[0].(float64); ok {
			arg = a
		} else if a, ok := args[0].(int); ok {
			arg = float64(a)
		} else {
			// Non-numeric input returns undefined
			return nil, nil
		}

		// undefined inputs always return undefined
		if math.IsNaN(arg) {
			return nil, nil
		}

		return math.Abs(arg), nil
	}

	/**
	 * Rounds a number down to integer
	 * JavaScript equivalent: functions.js lines 1247-1257
	 * @param {float64} arg - Argument
	 * @returns {float64} rounded integer
	 */
	floorFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the number
		if len(args) == 0 {
			return nil, nil
		}

		arg := 0.0
		if a, ok := args[0].(float64); ok {
			arg = a
		} else if a, ok := args[0].(int); ok {
			arg = float64(a)
		} else {
			// Non-numeric input returns undefined
			return nil, nil
		}

		// undefined inputs always return undefined
		if math.IsNaN(arg) {
			return nil, nil
		}

		return math.Floor(arg), nil
	}

	/**
	 * Rounds a number up to integer
	 * JavaScript equivalent: functions.js lines 1264-1274
	 * @param {float64} arg - Argument
	 * @returns {float64} rounded integer
	 */
	ceilFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the number
		if len(args) == 0 {
			return nil, nil
		}

		arg := 0.0
		if a, ok := args[0].(float64); ok {
			arg = a
		} else if a, ok := args[0].(int); ok {
			arg = float64(a)
		} else {
			// Non-numeric input returns undefined
			return nil, nil
		}

		// undefined inputs always return undefined
		if math.IsNaN(arg) {
			return nil, nil
		}

		return math.Ceil(arg), nil
	}

	/**
	 * Round to half even
	 * JavaScript equivalent: functions.js lines 1282-1319
	 * @param {float64} arg - Argument
	 * @param {int} precision - number of decimal places
	 * @returns {float64} rounded integer
	 */
	roundFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: number, optional precision
		if len(args) == 0 {
			return nil, nil
		}

		// undefined inputs always return undefined
		if args[0] == nil {
			return nil, nil
		}

		arg := 0.0
		if a, ok := args[0].(float64); ok {
			arg = a
		} else if a, ok := args[0].(int); ok {
			arg = float64(a)
		} else {
			// Non-numeric input returns undefined
			return nil, nil
		}

		var precision *int
		if len(args) > 1 && args[1] != nil {
			if p, ok := args[1].(float64); ok {
				val := int(p)
				precision = &val
			} else if p, ok := args[1].(int); ok {
				precision = &p
			}
		}

		// undefined inputs always return undefined
		if math.IsNaN(arg) {
			return nil, nil
		}

		var result float64

		if precision != nil && *precision != 0 {
			// Use string manipulation to avoid floating point precision issues
			// This mimics the JavaScript implementation
			argStr := fmt.Sprintf("%g", arg)
			parts := strings.Split(argStr, "e")
			exp := 0
			if len(parts) > 1 {
				exp, _ = strconv.Atoi(parts[1])
			}

			// Shift decimal place
			shiftedExp := exp + *precision
			shiftedStr := parts[0]
			if shiftedExp != 0 {
				shiftedStr = fmt.Sprintf("%se%d", parts[0], shiftedExp)
			}
			shiftedNum, _ := strconv.ParseFloat(shiftedStr, 64)

			// Round to nearest int
			result = math.Round(shiftedNum)
			diff := result - shiftedNum
			if math.Abs(diff) == 0.5 && math.Abs(math.Mod(result, 2)) == 1 {
				// rounded the wrong way - adjust to nearest even number
				if result > 0 {
					result = result - 1
				} else {
					result = result + 1
				}
			}

			// Shift back
			resultStr := fmt.Sprintf("%g", result)
			parts = strings.Split(resultStr, "e")
			exp = 0
			if len(parts) > 1 {
				exp, _ = strconv.Atoi(parts[1])
			}
			finalExp := exp - *precision
			finalStr := parts[0]
			if finalExp != 0 {
				finalStr = fmt.Sprintf("%se%d", parts[0], finalExp)
			}
			result, _ = strconv.ParseFloat(finalStr, 64)
		} else {
			// No precision specified - simple rounding
			result = math.Round(arg)
			diff := result - arg
			if math.Abs(diff) == 0.5 && math.Abs(math.Mod(result, 2)) == 1 {
				// rounded the wrong way - adjust to nearest even number
				if result > 0 {
					result = result - 1
				} else {
					result = result + 1
				}
			}
		}

		if result == -0 {
			// JSON doesn't do -0
			result = 0
		}
		return result, nil
	}

	/**
	 * Square root of number
	 * JavaScript equivalent: functions.js lines 1326-1346
	 * @param {float64} arg - Argument
	 * @returns {float64} square root
	 */
	/*
		$sqrt - Mathematical Square Root Function
		========================================

		Calculates the square root of a numeric value with proper error handling
		for mathematical domain constraints.

		Behavior:
		- Returns the square root for non-negative numbers
		- Undefined/NaN input returns undefined (graceful degradation)
		- Negative input throws domain error (mathematical constraint)
		- Follows IEEE 754 floating-point semantics

		Example usage:
		$sqrt(4)        → 2
		$sqrt(2)        → 1.4142135623730951
		$sqrt(0)        → 0
		$sqrt(-1)       → Error D3060 (domain error)
		$sqrt(undefined) → undefined

		Error codes:
		- D3060: Square root of negative number (domain error)

		JavaScript equivalent: functions.js lines 1328-1353
	*/
	sqrtFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the number
		if len(args) == 0 {
			return nil, nil
		}

		arg := 0.0
		if a, ok := args[0].(float64); ok {
			arg = a
		} else if a, ok := args[0].(int); ok {
			arg = float64(a)
		} else {
			// Non-numeric input returns undefined
			return nil, nil
		}

		// undefined inputs always return undefined
		// NaN represents undefined numeric values in JSONata
		if math.IsNaN(arg) {
			return nil, nil
		}

		// Domain validation: square root undefined for negative numbers
		if arg < 0 {
			return nil, &JSONataError{
				Code:  "D3060", // Domain error code for negative sqrt
				Index: 1,       // First parameter caused the error
				Value: arg,     // The problematic value
			}
		}

		// Calculate square root using Go's math library
		return math.Sqrt(arg), nil
	}

	/**
	 * Raises number to the power of the second number
	 * JavaScript equivalent: functions.js lines 1354-1375
	 * @param {float64} arg - the base
	 * @param {float64} exp - the exponent
	 * @returns {float64} result
	 */
	powerFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: base, exponent
		if len(args) < 2 {
			return nil, nil
		}

		arg := 0.0
		if a, ok := args[0].(float64); ok {
			arg = a
		} else if a, ok := args[0].(int); ok {
			arg = float64(a)
		} else {
			// Non-numeric input returns undefined
			return nil, nil
		}

		exp := 0.0
		if e, ok := args[1].(float64); ok {
			exp = e
		} else if e, ok := args[1].(int); ok {
			exp = float64(e)
		} else {
			// Non-numeric input returns undefined
			return nil, nil
		}

		// undefined inputs always return undefined
		if math.IsNaN(arg) {
			return nil, nil
		}

		result := math.Pow(arg, exp)

		if math.IsInf(result, 0) || math.IsNaN(result) {
			return nil, &JSONataError{
				Code:  "D3061",
				Index: 1,
				Value: arg,
			}
		}

		return result, nil
	}

	/**
	 * Returns a random number 0 <= n < 1
	 * JavaScript equivalent: functions.js lines 1381-1383
	 * @returns {float64} random number
	 */
	randomFunc := func(args []interface{}) (interface{}, error) {
		// random() takes no arguments
		return rand.Float64(), nil
	}

	// Forward declaration for recursive function
	/**
	 * Evaluate an input and return a boolean
	 * JavaScript equivalent: functions.js lines 1390-1431
	 * @param {[]interface{}} args - Arguments
	 * @returns {bool} Boolean
	 */
	var booleanFunc func([]interface{}) (interface{}, error)
	booleanFunc = func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the value
		if len(args) == 0 {
			return nil, nil
		}

		arg := args[0]

		// In JSON context, nil represents null, not undefined
		// JavaScript boolean(null) returns false
		// JavaScript boolean(undefined) returns undefined
		// Since JSON doesn't have undefined, nil maps to null behavior

		result := false
		switch v := arg.(type) {
		case []interface{}:
			if len(v) == 1 {
				b, err := booleanFunc([]interface{}{v[0]})
				return b, err
			} else if len(v) > 1 {
				for _, val := range v {
					if b, err := booleanFunc([]interface{}{val}); err != nil {
						return nil, err
					} else if b != nil && b.(bool) {
						result = true
						break
					}
				}
			}
		case string:
			if len(v) > 0 {
				result = true
			}
		case float64:
			if v != 0 {
				result = true
			}
		case int:
			if v != 0 {
				result = true
			}
		case bool:
			result = v
		case map[string]interface{}:
			if len(v) > 0 {
				result = true
			}
		case *JSONataNull:
			// JavaScript boolean(null) returns false
			result = false
		case nil:
			// nil represents undefined, should return undefined
			return nil, nil
		default:
			if !isFunction(arg) && arg != nil {
				result = true
			}
		}
		return result, nil
	}

	/**
	 * returns the Boolean NOT of the arg
	 * JavaScript equivalent: functions.js lines 1438-1445
	 * @param {interface{}} arg - argument
	 * @returns {bool} - NOT arg
	 */
	notFunc := func(args []interface{}) (interface{}, error) {
		// Handle arguments: first argument should be the value
		if len(args) == 0 {
			return nil, nil
		}

		arg := args[0]

		// undefined inputs always return undefined
		if arg == nil {
			return nil, nil
		}

		b, err := booleanFunc([]interface{}{arg})
		if err != nil {
			return nil, err
		}
		if b == nil {
			return nil, nil
		}
		return !b.(bool), nil
	}

	/**
	 * Helper function to build the arguments to be supplied to the function arg of the
	 * HOFs map, filter, each, sift and single
	 * JavaScript equivalent: functions.js lines 1456-1467
	 * @param {interface{}} func - the function to be invoked
	 * @param {interface{}} arg1 - the first (required) arg - the value
	 * @param {interface{}} arg2 - the second (optional) arg - the position (index or key)
	 * @param {interface{}} arg3 - the third (optional) arg - the whole structure (array or object)
	 * @returns {[]interface{}} the argument list
	 */
	hofFuncArgs := func(fn interface{}, arg1, arg2, arg3 interface{}) []interface{} {
		funcArgs := []interface{}{arg1} // the first arg (the value) is required
		// the other two are optional - only supply it if the function can take it
		length := getFunctionArity(fn)
		if length >= 2 {
			funcArgs = append(funcArgs, arg2)
		}
		if length >= 3 {
			funcArgs = append(funcArgs, arg3)
		}
		return funcArgs
	}

	/**
	 * Create a map from an array of arguments
	 * JavaScript equivalent: functions.js lines 1475-1493
	 * @param {[]interface{}} arr - array to map over
	 * @param {interface{}} func - function to apply
	 * @param {*Environment} env - environment
	 * @returns {[]interface{}} Map array
	 */
	mapFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("map function requires at least 2 arguments")
		}

		// Extract arguments based on signature "<af>"
		// args[0] = array (input)
		// args[1] = function (mapper)
		arr, ok1 := args[0].([]interface{})
		if !ok1 {
			return nil, nil
		}

		fn := args[1]

		// undefined inputs always return undefined
		if arr == nil {
			return nil, nil
		}

		result := createSequence()
		// do the map - iterate over the arrays, and invoke func - matches JavaScript
		for i := 0; i < len(arr); i++ {
			funcArgs := hofFuncArgs(fn, arr[i], float64(i), arr)
			// invoke func
			res, err := invokeFunction(fn, funcArgs)
			if err != nil {
				return nil, err
			}
			if res != nil {
				result.Data = append(result.Data, res)
			}
		}

		return result, nil
	}

	/**
	 * Create a map from an array of arguments
	 * JavaScript equivalent: functions.js lines 1501-1520
	 * @param {[]interface{}} arr - array to filter
	 * @param {interface{}} func - predicate function
	 * @param {*Environment} env - environment
	 * @returns {[]interface{}} Filtered array
	 */
	filterFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("filter function requires at least 2 arguments")
		}

		// Extract arguments
		input := args[0]
		fn := args[1]

		// undefined inputs always return undefined
		if input == nil {
			return nil, nil
		}

		// Special handling: In JSONata, if the input is a singleton array [x]
		// and the result would be [x], we should return just x
		// This matches JavaScript behavior
		wasSingletonArray := false
		singletonValue := interface{}(nil)

		// Check if input is an array
		arr, isArray := input.([]interface{})
		if isArray && len(arr) == 1 {
			// Remember this was a singleton array
			wasSingletonArray = true
			singletonValue = arr[0]
		}

		if !isArray {
			// For non-array inputs, treat as a singleton
			// Apply the predicate to the single value
			funcArgs := hofFuncArgs(fn, input, float64(0), []interface{}{input})
			res, err := invokeFunction(fn, funcArgs)
			if err != nil {
				return nil, err
			}
			// convert result to boolean
			boolRes, err := booleanFunc([]interface{}{res})
			if err != nil {
				return nil, err
			}
			if boolRes != nil && boolRes.(bool) {
				return input, nil
			}
			// If predicate is false, return empty array
			return []interface{}{}, nil
		}

		// For array inputs, proceed as before
		result := createSequence()

		for i := 0; i < len(arr); i++ {
			entry := arr[i]
			funcArgs := hofFuncArgs(fn, entry, float64(i), arr)
			// invoke func
			res, err := invokeFunction(fn, funcArgs)
			if err != nil {
				return nil, err
			}
			// convert result to boolean and check if it's truthy
			boolRes, err := booleanFunc([]interface{}{res})
			if err != nil {
				return nil, err
			}
			if boolRes != nil && boolRes.(bool) {
				result.Data = append(result.Data, entry)
			}
		}

		// Special case: if input was a singleton array [x] and result is [x], return x
		if wasSingletonArray && len(result.Data) == 1 && deepEquals(result.Data[0], singletonValue) {
			return singletonValue, nil
		}

		return result.Data, nil
	}

	/**
	 * Given an array, find the single element matching a specified condition
	 * JavaScript equivalent: functions.js lines 1529-1569
	 * @param {[]interface{}} arr - array to filter
	 * @param {interface{}} func - predicate function
	 * @param {*Environment} env - environment
	 * @returns {interface{}} Matching element
	 */
	singleFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("single function requires at least 2 arguments")
		}

		// Extract arguments
		arr, ok1 := args[0].([]interface{})
		if !ok1 {
			return nil, nil
		}

		fn := args[1]

		// undefined inputs always return undefined
		if arr == nil {
			return nil, nil
		}

		hasFoundMatch := false
		var result interface{}

		for i := 0; i < len(arr); i++ {
			entry := arr[i]
			positiveResult := true
			if fn != nil {
				funcArgs := hofFuncArgs(fn, entry, float64(i), arr)
				// invoke func
				res, err := invokeFunction(fn, funcArgs)
				if err != nil {
					return nil, err
				}
				boolRes, err := booleanFunc([]interface{}{res})
				if err != nil {
					return nil, err
				}
				positiveResult = boolRes != nil && boolRes.(bool)
			}
			if positiveResult {
				if !hasFoundMatch {
					result = entry
					hasFoundMatch = true
				} else {
					return nil, &JSONataError{
						Code:  "D3138",
						Index: i,
					}
				}
			}
		}

		if !hasFoundMatch {
			return nil, &JSONataError{
				Code: "D3139",
			}
		}

		return result, nil
	}

	/**
	 * Convolves (zips) each value from a set of arrays
	 * JavaScript equivalent: functions.js lines 1576-1594
	 * @param {...[]interface{}} args - arrays to zip
	 * @returns {[]interface{}} Zipped array
	 */
	zipFunc := func(args []interface{}) (interface{}, error) {
		// Convert variadic args to arrays
		arrays := args
		result := []interface{}{}
		// length of the shortest array
		minLength := math.MaxInt32
		for _, array := range arrays {
			if arr, ok := array.([]interface{}); ok {
				if len(arr) < minLength {
					minLength = len(arr)
				}
			} else {
				// If any argument is not an array (including nil/undefined),
				// treat its length as 0, which will make the result empty
				minLength = 0
			}
		}
		// If we still have MaxInt32, no arrays were found
		if minLength == math.MaxInt32 {
			minLength = 0
		}
		for i := 0; i < minLength; i++ {
			tuple := []interface{}{}
			for _, array := range arrays {
				if arr, ok := array.([]interface{}); ok {
					tuple = append(tuple, arr[i])
				}
			}
			result = append(result, tuple)
		}
		return result, nil
	}

	/**
	 * Fold left function
	 * JavaScript equivalent: functions.js lines 1603-1642
	 * @param {[]interface{}} sequence - Sequence
	 * @param {interface{}} func - Function
	 * @param {interface{}} init - Initial value
	 * @param {*Environment} env - environment
	 * @returns {interface{}} Result
	 */
	foldLeftFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("foldLeft function requires at least 2 arguments")
		}

		// Extract arguments
		sequence, ok1 := args[0].([]interface{})
		if !ok1 {
			return nil, nil
		}

		fn := args[1]
		var init interface{}
		if len(args) >= 3 {
			init = args[2]
		}

		// undefined inputs always return undefined
		if sequence == nil {
			return nil, nil
		}

		arity := getFunctionArity(fn)
		if arity < 2 {
			return nil, &JSONataError{
				Code:  "D3050",
				Index: 1,
			}
		}

		var result interface{}
		index := 0
		if init == nil && len(sequence) > 0 {
			result = sequence[0]
			index = 1
		} else {
			result = init
		}

		for index < len(sequence) {
			args := []interface{}{result, sequence[index]}
			if arity >= 3 {
				args = append(args, float64(index))
			}
			if arity >= 4 {
				args = append(args, sequence)
			}
			// invoke function
			res, err := invokeFunction(fn, args)
			if err != nil {
				return nil, err
			}
			result = res
			index++
		}

		return result, nil
	}

	// Forward declaration for recursive function
	var keysFunc JSONataFunc

	/**
	 * Return keys for an object
	 * JavaScript equivalent: functions.js lines 1649-1666
	 * @param {interface{}} arg - Object
	 * @returns {[]string} Array of keys
	 */
	keysFunc = func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return createSequence(), nil
		}
		arg := args[0]
		result := createSequence()

		switch v := arg.(type) {
		case []interface{}:
			// merge the keys of all of the items in the array
			merge := make(map[string]bool)
			for _, item := range v {
				if allkeys, err := keysFunc([]interface{}{item}); err != nil {
					return nil, err
				} else if allkeys != nil {
					if keySeq, ok := allkeys.(*Sequence); ok {
						for _, key := range keySeq.Data {
							if keyStr, ok := key.(string); ok {
								merge[keyStr] = true
							}
						}
					}
				}
			}
			for key := range merge {
				result.Data = append(result.Data, key)
			}
		case map[string]interface{}:
			for key := range v {
				result.Data = append(result.Data, key)
			}
		}
		if len(result.Data) == 0 {
			return nil, nil
		}
		return result, nil
	}

	// Forward declaration for recursive function
	var lookupFunc JSONataFunc
	var spreadFunc JSONataFunc

	/**
	 * Return value from an object for a given key
	 * JavaScript equivalent: functions.js lines 1674-1693
	 * @param {interface{}} input - Object/Array
	 * @param {string} key - Key in object
	 * @returns {interface{}} Value of key in object
	 */
	lookupFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, nil
		}
		input := args[0]
		key, ok := args[1].(string)
		if !ok {
			return nil, nil
		}
		// lookup the 'name' item in the input
		var result interface{}
		switch v := input.(type) {
		case []interface{}:
			seq := createSequence()
			for _, item := range v {
				res, err := lookupFunc([]interface{}{item, key})
				if err != nil {
					return nil, err
				}
				if res != nil {
					if arr, ok := res.([]interface{}); ok {
						seq.Data = append(seq.Data, arr...)
					} else {
						seq.Data = append(seq.Data, res)
					}
				}
			}
			if len(seq.Data) > 0 {
				result = seq.Data
			}
		case map[string]interface{}:
			result = v[key]
		}
		return result, nil
	}

	/**
	 * Append second argument to first
	 * JavaScript equivalent: functions.js lines 1701-1717
	 * @param {interface{}} arg1 - First argument
	 * @param {interface{}} arg2 - Second argument
	 * @returns {[]interface{}} Appended arguments
	 */
	appendFunc := func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			if len(args) == 0 {
				return nil, nil
			}
			return args[0], nil
		}

		arg1, arg2 := args[0], args[1]

		// disregard undefined args
		if arg1 == nil {
			return arg2, nil
		}
		if arg2 == nil {
			return arg1, nil
		}
		// if either argument is not an array, make it so
		arr1, ok1 := arg1.([]interface{})
		if !ok1 {
			arr1 = []interface{}{arg1}
		}
		arr2, ok2 := arg2.([]interface{})
		if !ok2 {
			arr2 = []interface{}{arg2}
		}
		result := make([]interface{}, len(arr1)+len(arr2))
		copy(result, arr1)
		copy(result[len(arr1):], arr2)
		return result, nil
	}

	/**
	 * Determines if the argument is undefined
	 * JavaScript equivalent: functions.js lines 1724-1730
	 * @param {interface{}} arg - argument
	 * @returns {bool} False if argument undefined, otherwise true
	 */
	existsFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return false, nil
		}
		return args[0] != nil, nil
	}

	// Forward declaration for recursive function
	var spreadFuncRecursive func(interface{}) (interface{}, error)

	/**
	 * Splits an object into an array of object with one property each
	 * JavaScript equivalent: functions.js lines 1737-1755
	 * @param {[]interface{}} args - arguments with the object to split
	 * @returns {[]interface{}} - the array
	 */
	spreadFunc = func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}
		return spreadFuncRecursive(args[0])
	}

	spreadFuncRecursive = func(arg interface{}) (interface{}, error) {
		result := createSequence()

		switch v := arg.(type) {
		case []interface{}:
			// spread all of the items in the array
			for _, item := range v {
				spreadItem, err := spreadFuncRecursive(item)
				if err != nil {
					return nil, err
				}
				if arr, ok := spreadItem.([]interface{}); ok {
					result.Data = append(result.Data, arr...)
				} else if spreadItem != nil {
					result.Data = append(result.Data, spreadItem)
				}
			}
		case map[string]interface{}:
			for key, val := range v {
				obj := map[string]interface{}{key: val}
				result.Data = append(result.Data, obj)
			}
		default:
			// For non-array, non-object values (including lambdas), return as-is
			return arg, nil
		}
		if len(result.Data) == 0 {
			return nil, nil
		}
		return result.Data, nil
	}

	/**
	 * Merges an array of objects into a single object
	 * JavaScript equivalent: functions.js lines 1763-1777
	 * @param {[]interface{}} arg - the objects to merge
	 * @returns {map[string]interface{}} - the object
	 */
	mergeFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}

		arg, ok := args[0].([]interface{})
		if !ok {
			if args[0] == nil {
				return nil, nil
			}
			// If not an array, wrap it
			arg = []interface{}{args[0]}
		}
		// undefined inputs always return undefined
		if arg == nil {
			return nil, nil
		}

		result := make(map[string]interface{})

		for _, item := range arg {
			if obj, ok := item.(map[string]interface{}); ok {
				for prop, val := range obj {
					result[prop] = val
				}
			}
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil
	}

	/**
	 * Reverses the order of items in an array
	 * JavaScript equivalent: functions.js lines 1784-1801
	 * @param {[]interface{}} arr - the array to reverse
	 * @returns {[]interface{}} - the reversed array
	 */
	reverseFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}

		arr, ok := args[0].([]interface{})
		if !ok {
			if args[0] == nil {
				return nil, nil
			}
			// If not an array, return as-is
			return args[0], nil
		}

		if len(arr) <= 1 {
			return arr, nil
		}

		length := len(arr)
		result := make([]interface{}, length)
		for i := 0; i < length; i++ {
			result[length-i-1] = arr[i]
		}

		return result, nil
	}

	/**
	 *
	 * JavaScript equivalent: functions.js lines 1809-1822
	 * @param {map[string]interface{}} obj - the input object to iterate over
	 * @param {interface{}} func - the function to apply to each key/value pair
	 * @param {*Environment} env - environment
	 * @returns {[]interface{}} - the resultant array
	 */
	eachFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("each function requires at least 2 arguments")
		}

		// Extract arguments
		obj, ok1 := args[0].(map[string]interface{})
		if !ok1 {
			return nil, nil
		}

		fn := args[1]
		result := createSequence()

		for key, value := range obj {
			funcArgs := hofFuncArgs(fn, value, key, obj)
			// invoke func
			res, err := invokeFunction(fn, funcArgs)
			if err != nil {
				return nil, err
			}
			if res != nil {
				result.Data = append(result.Data, res)
			}
		}

		if len(result.Data) == 0 {
			return nil, nil
		}
		return result.Data, nil
	}

	/**
	 *
	 * JavaScript equivalent: functions.js lines 1829-1835
	 * @param {string} message - the message to attach to the error
	 * @throws custom error with code 'D3137'
	 */
	errorFunc := func(args []interface{}) (interface{}, error) {
		message := "$error() function evaluated"
		if len(args) > 0 && args[0] != nil {
			message = fmt.Sprintf("%v", args[0])
		}
		return nil, &JSONataError{
			Code:    "D3137",
			Message: message,
		}
	}

	/**
	 *
	 * JavaScript equivalent: functions.js lines 1844-1854
	 * @param {bool} condition - the condition to evaluate
	 * @param {string} message - the message to attach to the error
	 * @returns {interface{}}
	 */
	assertFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, &JSONataError{
				Code:    "D3141",
				Message: "$assert() statement failed",
			}
		}

		// Extract condition
		condition := false
		if args[0] != nil {
			if b, ok := args[0].(bool); ok {
				condition = b
			}
		}

		// Extract message
		message := "$assert() statement failed"
		if len(args) > 1 && args[1] != nil {
			message = fmt.Sprintf("%v", args[1])
		}

		if !condition {
			return nil, &JSONataError{
				Code:    "D3141",
				Message: message,
			}
		}

		return nil, nil
	}

	/**
	 * JavaScript equivalent: functions.js lines 1861-1891
	 *
	 * @param {interface{}} value - the input to which the type will be checked
	 * @returns {string} - the type of the input
	 */
	typeFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}

		value := args[0]
		if value == nil {
			return nil, nil
		}

		// Check for JSONataNull first (explicit null)
		if _, isNull := value.(*JSONataNull); isNull {
			return "null", nil
		}

		switch v := value.(type) {
		case float64, int, int32, int64, uint, uint32, uint64:
			return "number", nil
		case string:
			return "string", nil
		case bool:
			return "boolean", nil
		case []interface{}:
			return "array", nil
		default:
			if isFunction(v) {
				return "function", nil
			}
			// Check for null (which in Go is represented differently)
			rv := reflect.ValueOf(v)
			if !rv.IsValid() {
				return "null", nil
			}
			return "object", nil
		}
	}

	/**
	 * Implements the merge sort (stable) with optional comparator function
	 * JavaScript equivalent: functions.js lines 1900-1966
	 *
	 * @param {[]interface{}} arr - the array to sort
	 * @param {interface{}} comparator - comparator function
	 * @param {*Environment} env - environment
	 * @returns {[]interface{}} - sorted array
	 */
	sortFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}

		arr, ok := args[0].([]interface{})
		if !ok {
			return nil, nil
		}

		var comparator interface{}
		if len(args) > 1 {
			comparator = args[1]
		}
		// undefined inputs always return undefined
		if arr == nil {
			return nil, nil
		}

		if len(arr) <= 1 {
			return arr, nil
		}

		// Default comparator
		if comparator == nil {
			// Check if array is all numbers or all strings
			allNumbers := true
			allStrings := true
			for _, item := range arr {
				switch item.(type) {
				case float64, int:
					allStrings = false
				case string:
					allNumbers = false
				default:
					allNumbers = false
					allStrings = false
				}
			}
			if !allNumbers && !allStrings {
				return nil, &JSONataError{
					Code:  "D3070",
					Index: 1,
				}
			}
		}

		// Implement stable merge sort to match JavaScript behavior
		var mergeSort func([]interface{}) ([]interface{}, error)
		mergeSort = func(arr []interface{}) ([]interface{}, error) {
			if len(arr) <= 1 {
				return arr, nil
			}

			middle := len(arr) / 2
			left, err := mergeSort(arr[:middle])
			if err != nil {
				return nil, err
			}
			right, err := mergeSort(arr[middle:])
			if err != nil {
				return nil, err
			}

			// Merge the sorted halves
			result := make([]interface{}, 0, len(arr))
			i, j := 0, 0

			for i < len(left) && j < len(right) {
				shouldSwap := false

				if comparator == nil {
					// Default comparison
					switch v1 := left[i].(type) {
					case float64:
						if v2, ok := right[j].(float64); ok {
							shouldSwap = v1 > v2
						}
					case int:
						if v2, ok := right[j].(int); ok {
							shouldSwap = v1 > v2
						}
					case string:
						if v2, ok := right[j].(string); ok {
							shouldSwap = v1 > v2
						}
					}
				} else {
					// Call custom comparator function
					if cmpFunc, ok := comparator.(func(interface{}, interface{}) (bool, error)); ok {
						shouldSwap, err = cmpFunc(left[i], right[j])
						if err != nil {
							return nil, err
						}
					} else if lambda, ok := comparator.(*Lambda); ok {
						// Handle Lambda comparator
						compResult, err := lambda.apply(nil, []interface{}{left[i], right[j]})
						if err != nil {
							return nil, err
						}
						// Convert result to boolean using JavaScript truthiness rules
						if compResult != nil {
							switch v := compResult.(type) {
							case bool:
								shouldSwap = v
							case float64:
								shouldSwap = v != 0
							case int:
								shouldSwap = v != 0
							case string:
								shouldSwap = v != ""
							default:
								shouldSwap = true // non-nil values are truthy
							}
						}
					}
				}

				if shouldSwap {
					// right[j] should come before left[i]
					result = append(result, right[j])
					j++
				} else {
					// left[i] should come before right[j] (or they're equal, preserve order)
					result = append(result, left[i])
					i++
				}
			}

			// Append remaining elements
			result = append(result, left[i:]...)
			result = append(result, right[j:]...)

			return result, nil
		}

		return mergeSort(arr)
	}

	/**
	 * Randomly shuffles the contents of an array
	 * JavaScript equivalent: functions.js lines 1973-1994
	 * @param {[]interface{}} arr - the input array
	 * @returns {[]interface{}} the shuffled array
	 */
	shuffleFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}

		arr, ok := args[0].([]interface{})
		if !ok {
			if args[0] == nil {
				return nil, nil
			}
			// If not an array, return as-is
			return args[0], nil
		}
		// undefined inputs always return undefined
		if arr == nil {
			return nil, nil
		}

		if len(arr) <= 1 {
			return arr, nil
		}

		// shuffle using the 'inside-out' variant of the Fisher-Yates algorithm
		result := make([]interface{}, len(arr))
		for i := 0; i < len(arr); i++ {
			j := rand.Intn(i + 1) // random integer such that 0 ≤ j ≤ i
			if i != j {
				result[i] = result[j]
			}
			result[j] = arr[i]
		}

		return result, nil
	}

	/**
	 * Returns the values that appear in a sequence, with duplicates eliminated.
	 * JavaScript equivalent: functions.js lines 2001-2028
	 * @param {[]interface{}} arr - An array or sequence of values
	 * @returns {[]interface{}} - sequence of distinct values
	 */
	distinctFunc := func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}

		arr, ok := args[0].([]interface{})
		if !ok {
			if args[0] == nil {
				return nil, nil
			}
			// If not an array, return as-is
			return args[0], nil
		}
		// undefined inputs always return undefined
		if arr == nil {
			return nil, nil
		}

		if len(arr) <= 1 {
			return arr, nil
		}

		var results []interface{}
		if isSequence(arr) {
			seq := createSequence()
			results = seq.Data
		} else {
			results = []interface{}{}
		}

		for _, value := range arr {
			// is this value already in the result sequence?
			includes := false
			for _, existing := range results {
				if deepEquals(value, existing) {
					includes = true
					break
				}
			}
			if !includes {
				results = append(results, value)
			}
		}
		return results, nil
	}

	/**
	 * Applies a predicate function to each key/value pair in an object
	 * JavaScript equivalent: functions.js lines 2038-2057
	 *
	 * @param {map[string]interface{}} arg - the object to be sifted
	 * @param {interface{}} func - the predicate function (lambda or native)
	 * @param {*Environment} env - environment
	 * @returns {map[string]interface{}} - sifted object
	 */
	siftFunc = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, errors.New("sift function requires at least 2 arguments")
		}

		// Extract arguments
		arg, ok1 := args[0].(map[string]interface{})
		if !ok1 {
			return nil, nil
		}

		fn := args[1]
		result := make(map[string]interface{})

		for item, entry := range arg {
			funcArgs := hofFuncArgs(fn, entry, item, arg)
			// invoke func
			res, err := invokeFunction(fn, funcArgs)
			if err != nil {
				return nil, err
			}
			// convert result to boolean and check if it's truthy
			boolRes, err := booleanFunc([]interface{}{res})
			if err != nil {
				return nil, err
			}
			if boolRes != nil && boolRes.(bool) {
				result[item] = entry
			}
		}

		// empty objects should be changed to undefined
		if len(result) == 0 {
			return nil, nil
		}

		return result, nil
	}

	// Random seed is automatically initialized in Go 1.20+
	// rand.Seed is deprecated and no longer needed

	return &FunctionsModule{
		sumFunc:                sumFunc,
		countFunc:              countFunc,
		maxFunc:                maxFunc,
		minFunc:                minFunc,
		averageFunc:            averageFunc,
		stringFunc:             stringFunc,
		substringFunc:          substringFunc,
		substringBeforeFunc:    substringBeforeFunc,
		substringAfterFunc:     substringAfterFunc,
		lowercaseFunc:          lowercaseFunc,
		uppercaseFunc:          uppercaseFunc,
		lengthFunc:             lengthFunc,
		trimFunc:               trimFunc,
		padFunc:                padFunc,
		matchFunc:              matchFunc,
		containsFunc:           containsFunc,
		replaceFunc:            replaceFunc,
		splitFunc:              splitFunc,
		joinFunc:               joinFunc,
		formatNumberFunc:       formatNumberFunc,
		formatBaseFunc:         formatBaseFunc,
		numberFunc:             numberFunc,
		floorFunc:              floorFunc,
		ceilFunc:               ceilFunc,
		roundFunc:              roundFunc,
		absFunc:                absFunc,
		sqrtFunc:               sqrtFunc,
		powerFunc:              powerFunc,
		randomFunc:             randomFunc,
		booleanFunc:            booleanFunc,
		notFunc:                notFunc,
		mapFunc:                mapFunc,
		zipFunc:                zipFunc,
		filterFunc:             filterFunc,
		singleFunc:             singleFunc,
		foldLeftFunc:           foldLeftFunc,
		siftFunc:               siftFunc,
		keysFunc:               keysFunc,
		lookupFunc:             lookupFunc,
		appendFunc:             appendFunc,
		existsFunc:             existsFunc,
		spreadFunc:             spreadFunc,
		mergeFunc:              mergeFunc,
		reverseFunc:            reverseFunc,
		eachFunc:               eachFunc,
		errorFunc:              errorFunc,
		assertFunc:             assertFunc,
		typeFunc:               typeFunc,
		sortFunc:               sortFunc,
		shuffleFunc:            shuffleFunc,
		distinctFunc:           distinctFunc,
		base64encodeFunc:       base64encodeFunc,
		base64decodeFunc:       base64decodeFunc,
		encodeUrlComponentFunc: encodeUrlComponentFunc,
		encodeUrlFunc:          encodeUrlFunc,
		decodeUrlComponentFunc: decodeUrlComponentFunc,
		decodeUrlFunc:          decodeUrlFunc,
	}
}

// FunctionsModule contains all the built-in functions
type FunctionsModule struct {
	sumFunc                JSONataFunc
	countFunc              JSONataFunc
	maxFunc                JSONataFunc
	minFunc                JSONataFunc
	averageFunc            JSONataFunc
	stringFunc             JSONataFunc
	substringFunc          JSONataFunc
	substringBeforeFunc    JSONataFunc
	substringAfterFunc     JSONataFunc
	lowercaseFunc          JSONataFunc
	uppercaseFunc          JSONataFunc
	lengthFunc             JSONataFunc
	trimFunc               JSONataFunc
	padFunc                JSONataFunc
	matchFunc              JSONataFunc
	containsFunc           JSONataFunc
	replaceFunc            JSONataFunc
	splitFunc              JSONataFunc
	joinFunc               JSONataFunc
	formatNumberFunc       JSONataFunc
	formatBaseFunc         JSONataFunc
	numberFunc             JSONataFunc
	floorFunc              JSONataFunc
	ceilFunc               JSONataFunc
	roundFunc              JSONataFunc
	absFunc                JSONataFunc
	sqrtFunc               JSONataFunc
	powerFunc              JSONataFunc
	randomFunc             JSONataFunc
	booleanFunc            JSONataFunc
	notFunc                JSONataFunc
	mapFunc                JSONataFunc
	zipFunc                JSONataFunc
	filterFunc             JSONataFunc
	singleFunc             JSONataFunc
	foldLeftFunc           JSONataFunc
	siftFunc               JSONataFunc
	keysFunc               JSONataFunc
	lookupFunc             JSONataFunc
	appendFunc             JSONataFunc
	existsFunc             JSONataFunc
	spreadFunc             JSONataFunc
	mergeFunc              JSONataFunc
	reverseFunc            JSONataFunc
	eachFunc               JSONataFunc
	errorFunc              JSONataFunc
	assertFunc             JSONataFunc
	typeFunc               JSONataFunc
	sortFunc               JSONataFunc
	shuffleFunc            JSONataFunc
	distinctFunc           JSONataFunc
	base64encodeFunc       JSONataFunc
	base64decodeFunc       JSONataFunc
	encodeUrlComponentFunc JSONataFunc
	encodeUrlFunc          JSONataFunc
	decodeUrlComponentFunc JSONataFunc
	decodeUrlFunc          JSONataFunc
}

// MatchResult represents the result of a regex match
type MatchResult struct {
	Match  string
	Start  int
	End    int
	Groups []string
	Next   func() *MatchResult
}
