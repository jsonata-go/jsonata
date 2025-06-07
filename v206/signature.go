// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

/*
JSONata Function Signature System
=================================

Portions Copyright IBM Corp. 2016, 2018 All Rights Reserved
Project name: JSONata
This project is licensed under the MIT License, see LICENSE

Overview:
--------
This module implements JSONata's function signature system, providing runtime type
checking and validation for function calls. The signature system ensures that
functions receive arguments of the correct types and cardinality, providing clear
error messages when type mismatches occur.

Signature Syntax:
---------------
Function signatures use a compact notation to describe parameter and return types:

Type Symbols:
- s: string
- n: number
- b: boolean
- l: null
- a: array
- o: object
- f: function
- j: any JSON value
- m: missing/undefined
- x: any value (wildcard)

Modifiers:
- ? : optional parameter
- + : one or more
- * : zero or more
- - : context parameter (doesn't count toward arity)

Examples:
- "<s:s>": Function takes one string, returns string
- "<n-n:n>": Function takes two numbers, returns number
- "<a<n>:n>": Function takes array of numbers, returns number
- "<s?n?:s>": Function takes optional string and optional number, returns string
- "<f<j>:j>": Higher-order function taking function that takes JSON and returns JSON

Complex Signatures:
- "<a+:a>": One or more arrays, returns array
- "<x-:b>": Takes any value, returns boolean (- means value doesn't affect arity)
- "<f<jj:j>a<j>j?:j>": Function taking (func(j,j)->j, array<j>, optional j) -> j

Validation Process:
-----------------
1. Parse signature string into structured representation
2. Check argument count against parameter requirements
3. Validate each argument type against parameter type specification
4. Handle optional parameters and variadic arguments
5. Provide detailed error messages for type mismatches

The signature system enables:
- Compile-time-like type checking for dynamic language
- Self-documenting function interfaces
- Consistent error reporting across all built-in functions
- Support for higher-order functions with complex type requirements
*/

package jsonata

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

/*
getSymbol - Value to Type Symbol Conversion
==========================================

Converts a Go value to its corresponding JSONata type symbol for signature validation.
This function bridges Go's type system with JSONata's signature type notation,
enabling runtime type checking of function arguments.

Type Mapping (Go -> JSONata symbol):
- string -> "s" (string)
- All numeric types -> "n" (number)
- bool -> "b" (boolean)
- nil -> "l" (null/literal)
- Functions -> "f" (function)
- Slices/Arrays -> "a" (array)
- Maps/Structs -> "o" (object)
- Unknown/undefined -> "m" (missing)

This mapping ensures that JSONata's type system can accurately represent and
validate Go values while maintaining compatibility with the JavaScript reference
implementation's type semantics.

JavaScript equivalent: signature.js line 156
*/
func getSymbol(value interface{}) string {
	var symbol string

	// Functions are first-class values in JSONata and need special handling
	if isFunction(value) {
		symbol = "f"
	} else {
		// Map Go types to JSONata type symbols
		switch v := value.(type) {
		case string:
			symbol = "s" // String type
		case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			symbol = "n" // All numeric types map to JSONata number
		case bool:
			symbol = "b" // Boolean type
		case nil:
			// Return a character that doesn't match any specific type but will be handled specially
			symbol = "?" // Special marker for nil/undefined
		case *JSONataNull:
			symbol = "l" // Null/literal (JSONataNull represents JavaScript null)
		case *JSONataArray:
			symbol = "a" // JSONataArray should be treated as an array type
		default:
			// Use reflection for complex types
			rv := reflect.ValueOf(v)
			switch rv.Kind() {
			case reflect.Slice, reflect.Array:
				symbol = "a" // Array type (includes Go slices)
			case reflect.Map, reflect.Struct:
				symbol = "o" // Object type (maps and structs)
			default:
				// Unknown or undefined value types
				symbol = "m" // m for missing/undefined
			}
		}
	}
	return symbol
}

// signature module - returns parseSignature function
func initSignature() func(string) (*SignatureValidator, error) {
	// A mapping between the function signature symbols and the full plural of the type
	// Expected to be used in error messages
	// Dummy references for unused imports
	_ = errors.New
	_ = fmt.Sprint

	arraySignatureMapping := map[string]string{
		"a": "arrays",
		"b": "booleans",
		"f": "functions",
		"n": "numbers",
		"o": "objects",
		"s": "strings",
	}
	_ = arraySignatureMapping // currently unused

	/**
	 * Parses a function signature definition and returns a validation function
	 * @param {string} signature - the signature between the <angle brackets>
	 * @returns {*SignatureValidator} validation function
	 */
	// JavaScript: signature.js line 28
	parseSignature := func(signature string) (*SignatureValidator, error) {
		// create a Regex that represents this signature and return a function that when invoked,
		// returns the validated (possibly fixed-up) arguments, or throws a validation error
		// step through the signature, one symbol at a time
		position := 1
		params := []SignatureParam{}
		param := SignatureParam{}
		var prevParam *SignatureParam
		if len(params) > 0 {
			prevParam = &params[len(params)-1]
		} else {
			prevParam = &param
		}

		for position < len(signature) {
			symbol := signature[position]
			if symbol == ':' {
				// TODO figure out what to do with the return type (JavaScript: signature.js line 39)
				// ignore it for now
				break
			}

			next := func() {
				params = append(params, param)
				if len(params) > 0 {
					prevParam = &params[len(params)-1]
				}
				param = SignatureParam{}
			}

			// JavaScript: signature.js line 50
			findClosingBracket := func(str string, start int, openSymbol, closeSymbol byte) int {
				// returns the position of the closing symbol (e.g. bracket) in a string
				// that balances the opening symbol at position start
				depth := 1
				position := start
				for position < len(str) {
					position++
					if position < len(str) {
						symbol := str[position]
						if symbol == closeSymbol {
							depth--
							if depth == 0 {
								// we're done
								break // out of while loop
							}
						} else if symbol == openSymbol {
							depth++
						}
					}
				}
				return position
			}

			switch symbol {
			case 's', 'n', 'b', 'l', 'o': // string, number, boolean, null, object
				param.regex = "[" + string(symbol) + "m?]"
				param.paramType = string(symbol)
				next()
			case 'a': // array
				// normally treat any value as singleton array
				param.regex = "[asnblfom?]"
				param.paramType = string(symbol)
				param.array = true
				next()
			case 'f': // function
				param.regex = "[f?]"
				param.paramType = string(symbol)
				next()
			case 'j': // any JSON type
				param.regex = "[asnblom?]"
				param.paramType = string(symbol)
				next()
			case 'x': // any type
				param.regex = "[asnblfom?]"
				param.paramType = string(symbol)
				next()
			case '-': // use context if param not supplied
				prevParam.context = true
				// Remove '?' from context regex since nil/undefined shouldn't match for context
				contextRegexStr := prevParam.regex
				if strings.Contains(contextRegexStr, "?") {
					contextRegexStr = strings.ReplaceAll(contextRegexStr, "?", "")
				}
				prevParam.contextRegex = regexp.MustCompile(contextRegexStr) // pre-compiled to test the context type at runtime
				prevParam.regex += "?"
			case '?', '+': // optional param, one or more
				prevParam.regex += string(symbol)
			case '(': // choice of types
				// search forward for matching ')'
				endParen := findClosingBracket(signature, position, '(', ')')
				choice := signature[position+1 : endParen]
				if !strings.Contains(choice, "<") {
					// no parameterized types, simple regex
					param.regex = "[" + choice + "m?]"
				} else {
					// TODO harder (JavaScript: signature.js line 120)
					return nil, &JSONataError{
						Code:  "S0402",
						Stack: "",
						Value: choice,
					}
				}
				param.paramType = "(" + choice + ")"
				position = endParen
				next()
			case '<': // type parameter - can only be applied to 'a' and 'f'
				if prevParam.paramType == "a" || prevParam.paramType == "f" {
					// search forward for matching '>'
					endPos := findClosingBracket(signature, position, '<', '>')
					prevParam.subtype = signature[position+1 : endPos]
					position = endPos
				} else {
					return nil, &JSONataError{
						Code:  "S0401",
						Stack: "",
						Value: prevParam.paramType,
					}
				}
			}
			position++
		}

		regexStr := "^"
		for _, p := range params {
			regexStr += "(" + p.regex + ")"
		}
		regexStr += "$"
		regex := regexp.MustCompile(regexStr)

		// JavaScript: signature.js line 190
		throwValidationError := func(badArgs []interface{}, badSig string) error {
			// to figure out where this went wrong we need apply each component of the
			// regex to each argument until we get to the one that fails to match
			partialPattern := "^"
			goodTo := 0
			for index := 0; index < len(params); index++ {
				partialPattern += params[index].regex
				match := regexp.MustCompile(partialPattern).FindString(badSig)
				if match == "" {
					// failed here
					return &JSONataError{
						Code:  "T0410",
						Stack: "",
						Value: func() interface{} {
							if goodTo < len(badArgs) {
								return badArgs[goodTo]
							}
							return nil
						}(),
					}
				}
				goodTo = len(match)
			}
			// if it got this far, it's probably because of extraneous arguments (we
			// haven't added the trailing '$' in the regex yet.
			var value interface{}
			if goodTo < len(badArgs) {
				value = badArgs[goodTo]
			}
			return &JSONataError{
				Code:  "T0410",
				Stack: "",
				Value: value,
			}
		}

		return &SignatureValidator{
			Definition: signature,
			// JavaScript: signature.js line 221
			Validate: func(args []interface{}, context interface{}) ([]interface{}, error) {
				suppliedSig := ""
				for _, arg := range args {
					suppliedSig += getSymbol(arg)
				}
				matches := regex.FindStringSubmatch(suppliedSig)
				if matches != nil {
					validatedArgs := []interface{}{}
					argIndex := 0
					for index, param := range params {
						var arg interface{}
						if argIndex < len(args) {
							arg = args[argIndex]
						}
						match := ""
						if index+1 < len(matches) {
							match = matches[index+1]
						}
						if match == "" {
							if param.context && param.contextRegex != nil {
								// substitute context value for missing arg
								// first check that the context value is the right type
								contextType := getSymbol(context)
								// Special case: 'x' type (any) should accept null context
								// This matches JavaScript behavior where $string() with null context returns "null"
								if param.paramType == "x" && context == nil {
									validatedArgs = append(validatedArgs, context)
								} else if param.contextRegex.MatchString(contextType) {
									validatedArgs = append(validatedArgs, context)
								} else {
									// context value not compatible with this argument
									return nil, &JSONataError{
										Code:  "T0411",
										Stack: "",
										Value: context,
									}
								}
							} else {
								validatedArgs = append(validatedArgs, arg)
								argIndex++
							}
						} else {
							// may have matched multiple args (if the regex ends with a '+'
							// split into single tokens
							for _, single := range match {
								if param.paramType == "a" {
									if single == 'm' || single == '?' {
										// missing (undefined) or nil
										arg = nil
									} else {
										if argIndex < len(args) {
											arg = args[argIndex]
											// Unwrap single JSONataArray from []interface{} wrapper and convert to slice
											if arr, ok := arg.([]interface{}); ok && len(arr) == 1 {
												if jsonArr, ok := arr[0].(*JSONataArray); ok {
													// Convert JSONataArray to []interface{} for function consumption
													elements := make([]interface{}, jsonArr.Length())
													for i := 0; i < jsonArr.Length(); i++ {
														elements[i] = jsonArr.Get(i)
													}
													arg = elements
												}
											}
										}
										arrayOK := true
										// Allow nil (undefined) to pass through for any type
										if arg != nil {
											// Only validate if arg is not nil
											// is there type information on the contents of the array?
											if param.subtype != "" {
												if single != 'a' && string(single) != param.subtype {
													arrayOK = false
												} else if single == 'a' {
													// Check if arg is array/slice or JSONataArray
													if jsonArr, ok := arg.(*JSONataArray); ok {
														// Handle JSONataArray
														if jsonArr.Length() > 0 {
															itemType := getSymbol(jsonArr.Get(0))
															if itemType != string(param.subtype[0]) { // TODO recurse further (JavaScript: signature.js line 272)
																arrayOK = false
															} else {
																// make sure every item in the array is this type
																for i := 0; i < jsonArr.Length(); i++ {
																	if getSymbol(jsonArr.Get(i)) != itemType {
																		arrayOK = false
																		break
																	}
																}
															}
														}
													} else {
														// Handle regular Go arrays/slices
														rv := reflect.ValueOf(arg)
														if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
															if rv.Len() > 0 {
																itemType := getSymbol(rv.Index(0).Interface())
																if itemType != string(param.subtype[0]) { // TODO recurse further (JavaScript: signature.js line 272)
																	arrayOK = false
																} else {
																	// make sure every item in the array is this type
																	for i := 0; i < rv.Len(); i++ {
																		if getSymbol(rv.Index(i).Interface()) != itemType {
																			arrayOK = false
																			break
																		}
																	}
																}
															}
														}
													}
												}
											} // end of if arg != nil
										}
										if !arrayOK {
											return nil, &JSONataError{
												Code:  "T0412",
												Stack: "",
												Value: arg,
											}
										}
										// the function expects an array. If it's not one, make it so
										if single != 'a' {
											arg = []interface{}{arg}
										}
									}
									validatedArgs = append(validatedArgs, arg)
									argIndex++
								} else {
									validatedArgs = append(validatedArgs, arg)
									argIndex++
								}
							}
						}
					}
					return validatedArgs, nil
				}
				return nil, throwValidationError(args, suppliedSig)
			},
		}, nil
	}

	return parseSignature
}

// SignatureParam represents a parameter in a function signature
type SignatureParam struct {
	regex        string
	paramType    string
	array        bool
	subtype      string
	context      bool
	contextRegex *regexp.Regexp
}

// SignatureValidator represents a parsed signature with validation
type SignatureValidator struct {
	Definition string
	Validate   func([]interface{}, interface{}) ([]interface{}, error)
}

// Global parseSignature function
var parseSignature = initSignature()

// ParseSignature is the exported version of parseSignature
var ParseSignature = parseSignature
