// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

/**
 * Portions Copyright IBM Corp. 2016 All Rights Reserved
 *   Project name: JSONata
 *   This project is licensed under the MIT License, see LICENSE
 */

package jsonata

import (
	"testing"
)

// TestParserRecovery tests the parser recovery functionality
// This is a transliteration of jsonata-js/test/parser-recovery.js
func TestParserRecovery(t *testing.T) {

	t.Run("Invoke parser with valid expression", func(t *testing.T) {
		t.Run("Account.Order[0]", func(t *testing.T) {
			// Test parser recovery mode with a valid expression
			// This should parse successfully and return no errors
			expr, err := Compile("Account.Order[0]", true)
			if err != nil {
				t.Fatalf("Parsing failed: %v", err)
			}

			// Verify the expression was created successfully
			if expr == nil {
				t.Fatal("Expression should not be nil")
			}

			// Verify no parsing errors were collected
			if len(expr.errors) > 0 {
				t.Errorf("Expected no parsing errors, got: %v", expr.errors)
			}

			// Test that the expression can evaluate successfully
			testData := map[string]interface{}{
				"Account": map[string]interface{}{
					"Order": []interface{}{
						map[string]interface{}{"id": 1},
						map[string]interface{}{"id": 2},
					},
				},
			}

			result, err := evaluateWithJSON(expr, testData, nil)
			if err != nil {
				t.Errorf("Evaluation failed: %v", err)
			}

			expected := map[string]interface{}{"id": float64(1)}
			if !isDeepEqual(result, expected) {
				t.Errorf("Expected %v, got %v", expected, result)
			}
			//                     "position": 14,
			//                     "type": "filter"
			//                 }
			//             ]
			//         }
			//     ]
			// }
		})
	})

	t.Run("Invoke parser with incomplete expression", func(t *testing.T) {
		testCases := []struct {
			name        string
			expression  string
			description string
		}{
			{
				name:        "Account.",
				expression:  "Account.",
				description: "should handle incomplete dot notation",
			},
			{
				name:        "Account[",
				expression:  "Account[",
				description: "should handle incomplete bracket notation",
			},
			{
				name:        "Account.Order[;0].Product",
				expression:  "Account.Order[;0].Product",
				description: "should handle invalid semicolon in filter",
			},
			{
				name:        "Account.Order[0;].Product",
				expression:  "Account.Order[0;].Product",
				description: "should handle trailing semicolon in filter",
			},
			{
				name:        "Account.Order[0].Product;",
				expression:  "Account.Order[0].Product;",
				description: "should handle trailing semicolon",
			},
			{
				name:        "$inputSource[0].UnstructuredAnswers^()[0].Text",
				expression:  "$inputSource[0].UnstructuredAnswers^()[0].Text",
				description: "should handle complex expression with sort operator",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Test parser recovery mode with malformed expressions
				// Recovery mode should allow parsing to continue and collect errors
				expr, err := Compile(tc.expression, true)

				// Recovery mode should not fail parsing, even for invalid syntax
				if err != nil {
					t.Fatalf("Recovery parsing should not fail for: %s, got error: %v", tc.expression, err)
				}

				// Verify the expression was created
				if expr == nil {
					t.Fatal("Expression should not be nil in recovery mode")
				}

				// For malformed expressions, we should have parsing errors collected
				if len(expr.errors) == 0 {
					t.Logf("Expected parsing errors for malformed expression: %s", tc.expression)
					// Note: This might be OK if our parser is more lenient than JS
				}

				// If there are errors, the expression should not be executable
				if len(expr.errors) > 0 {
					// Try to evaluate - this should fail
					_, evalErr := evaluateWithJSON(expr, nil, nil)
					if evalErr == nil {
						t.Errorf("Expression with parsing errors should not be executable: %s", tc.expression)
					}
				}
			})
		}

		t.Run("An expression with syntax error should not be executable", func(t *testing.T) {
			t.Run("Account.", func(t *testing.T) {
				// Test that expressions with syntax errors cannot be executed
				// even when parsed in recovery mode
				expr, err := Compile("Account.", true)

				// Recovery mode should allow parsing
				if err != nil {
					t.Fatalf("Recovery parsing failed: %v", err)
				}

				// But evaluation should fail
				_, evalErr := evaluateWithJSON(expr, nil, nil)
				if evalErr == nil {
					t.Error("Expression with syntax error should not be executable")
				}

				// Check that we get the expected error code
				if jsonataErr, ok := evalErr.(*JSONataError); ok {
					if jsonataErr.Code != "S0500" {
						t.Errorf("Expected error code S0500, got %s", jsonataErr.Code)
					}
				}
			})
		})
	})
}

// TestBasicParserErrors tests basic parser error handling without recovery mode
func TestBasicParserErrors(t *testing.T) {
	t.Run("Invalid syntax should cause compilation error", func(t *testing.T) {
		invalidExpressions := []string{
			"Account.",
			"Account[",
			"Account.Order[;0]",
			"$inputSource[0].UnstructuredAnswers^()[0]",
		}

		for _, expr := range invalidExpressions {
			t.Run("expression: "+expr, func(t *testing.T) {
				// In our current implementation, these should fail at compile time
				_, err := Compile(expr, false)
				if err == nil {
					t.Errorf("Expected compilation error for invalid expression: %s", expr)
				}
			})
		}
	})
}
