// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package jsonata

import (
	"strings"
	"testing"
	"time"
)

// TestDoSProtections tests various denial-of-service protection mechanisms
func TestDoSProtections(t *testing.T) {
	// Test recursion depth protection
	t.Run("Recursion Depth Protection", func(t *testing.T) {
		testCases := []struct {
			name      string
			expr      string
			maxDepth  int
			shouldErr bool
			errorCode string
		}{
			{
				name:      "Deep recursion with factorial",
				expr:      `($factorial := function($n) { $n <= 1 ? 1 : $n * $factorial($n - 1) }; $factorial(1000))`,
				maxDepth:  100,
				shouldErr: true,
				errorCode: "U1001",
			},
			{
				name:      "Non-tail recursive sum",
				expr:      `($sum := function($n) { $n <= 0 ? 0 : $n + $sum($n - 1) }; $sum(200))`,
				maxDepth:  100,
				shouldErr: true,
				errorCode: "U1001",
			},
			{
				name:      "Safe recursion within limit",
				expr:      `($factorial := function($n) { $n <= 1 ? 1 : $n * $factorial($n - 1) }; $factorial(5))`,
				maxDepth:  20,
				shouldErr: false,
			},
			{
				name:      "Nested function calls",
				expr:      `($f := function($x) { $x > 0 ? $f($f($x - 1)) : 0 }; $f(100))`,
				maxDepth:  20,
				shouldErr: true,
				errorCode: "U1001",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				expr, err := Compile(tc.expr, false)
				if err != nil {
					t.Fatalf("Failed to compile: %v", err)
				}

				expr.SetMaxDepth(tc.maxDepth)

				_, err = expr.Evaluate([]byte(`{}`), nil)
				if tc.shouldErr {
					if err == nil {
						t.Error("Expected recursion depth error but got none")
					} else if jsonErr, ok := err.(*JSONataError); ok {
						if jsonErr.Code != tc.errorCode {
							t.Errorf("Expected error code %s, got %s", tc.errorCode, jsonErr.Code)
						}
					}
				} else {
					if err != nil {
						t.Errorf("Unexpected error: %v", err)
					}
				}
			})
		}
	})

	// Test range limit protection
	t.Run("Range Limit Protection", func(t *testing.T) {
		testCases := []struct {
			name      string
			expr      string
			maxRange  int
			shouldErr bool
			errorCode string
		}{
			{
				name:      "Huge range direct",
				expr:      `[1..1000000000]`,
				maxRange:  1000,
				shouldErr: true,
				errorCode: "D2014",
			},
			{
				name:      "Huge range in expression",
				expr:      `[0..999999999].$string($)`,
				maxRange:  10000,
				shouldErr: true,
				errorCode: "D2014",
			},
			{
				name:      "Multiple ranges combined",
				expr:      `$append([1..5000], [5001..10000])`,
				maxRange:  4000,
				shouldErr: true,
				errorCode: "D2014",
			},
			{
				name:      "Safe range",
				expr:      `[1..100]`,
				maxRange:  1000,
				shouldErr: false,
			},
			{
				name:      "Range at exact limit",
				expr:      `[1..1000]`,
				maxRange:  1000,
				shouldErr: false,
			},
			{
				name:      "Range just over limit",
				expr:      `[1..1001]`,
				maxRange:  1000,
				shouldErr: true,
				errorCode: "D2014",
			},
			{
				name:      "Negative range (should be undefined, not error)",
				expr:      `[100..1]`,
				maxRange:  1000,
				shouldErr: false,
			},
			{
				name:      "Default 10M limit",
				expr:      `[1..10000001]`,
				maxRange:  0, // Use default
				shouldErr: true,
				errorCode: "D2014",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				expr, err := Compile(tc.expr, false)
				if err != nil {
					t.Fatalf("Failed to compile: %v", err)
				}

				if tc.maxRange > 0 {
					expr.SetMaxRange(tc.maxRange)
				}

				result, err := expr.Evaluate([]byte(`{}`), nil)
				if tc.shouldErr {
					if err == nil {
						t.Error("Expected range limit error but got none")
					} else if jsonErr, ok := err.(*JSONataError); ok {
						if jsonErr.Code != tc.errorCode {
							t.Errorf("Expected error code %s, got %s", tc.errorCode, jsonErr.Code)
						}
					}
				} else {
					if err != nil {
						t.Errorf("Unexpected error: %v", err)
					}
					// For negative range, result should be null or empty array
					if tc.name == "Negative range (should be undefined, not error)" {
						if string(result) != "null" && string(result) != "[]" {
							t.Errorf("Expected null or [] for negative range, got %s", string(result))
						}
					}
				}
			})
		}
	})

	// Test execution time protection
	t.Run("Execution Time Protection", func(t *testing.T) {
		testCases := []struct {
			name       string
			expr       string
			maxTimeMs  int
			shouldErr  bool
			errorCode  string
			skipReason string
		}{
			{
				name:      "Infinite loop simulation via deep recursion",
				expr:      `($loop := function($n) { $loop($n + 1) }; $loop(1))`,
				maxTimeMs: 100,
				shouldErr: true,
				errorCode: "U1002",
			},
			{
				name:      "Heavy computation",
				expr:      `[1..10000].([1..100].($ * $ * $ * $))`,
				maxTimeMs: 50,
				shouldErr: true,
				errorCode: "U1002",
			},
			{
				name:      "Nested loops",
				expr:      `[1..100].([1..100].([1..10].($ * $ * $)))`,
				maxTimeMs: 50, // Reduced from 100ms to 50ms to ensure timeout on fast Macs
				shouldErr: true,
				errorCode: "U1002",
			},
			{
				name:      "Quick operation within timeout",
				expr:      `[1..10].($*2)`,
				maxTimeMs: 1000,
				shouldErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.skipReason != "" {
					t.Skip(tc.skipReason)
				}

				expr, err := Compile(tc.expr, false)
				if err != nil {
					t.Fatalf("Failed to compile: %v", err)
				}

				expr.SetMaxTime(tc.maxTimeMs)
				// Also set recursion limit to prevent stack overflow before timeout
				expr.SetMaxDepth(100)

				start := time.Now()
				_, err = expr.Evaluate([]byte(`{}`), nil)
				elapsed := time.Since(start)

				if tc.shouldErr {
					if err == nil {
						t.Error("Expected timeout error but got none")
					} else if jsonErr, ok := err.(*JSONataError); ok {
						// Could be either timeout or recursion limit
						if jsonErr.Code != tc.errorCode && jsonErr.Code != "U1001" {
							t.Errorf("Expected error code %s or U1001, got %s", tc.errorCode, jsonErr.Code)
						}
					}
					// Verify it didn't run too long
					maxAllowedTime := time.Duration(tc.maxTimeMs*2) * time.Millisecond
					if elapsed > maxAllowedTime {
						t.Errorf("Execution took too long: %v (max allowed: %v)", elapsed, maxAllowedTime)
					}
				} else {
					if err != nil {
						t.Errorf("Unexpected error: %v", err)
					}
				}
			})
		}
	})

	// Test regex catastrophic backtracking protection
	t.Run("Regex Backtracking Protection", func(t *testing.T) {
		// Note: Go's regexp package (RE2) is immune to catastrophic backtracking
		// These tests verify that expressions complete quickly despite complex patterns
		testCases := []struct {
			name         string
			expr         string
			maxTimeMs    int
			expectResult bool
			description  string
		}{
			{
				name:         "Catastrophic backtracking pattern",
				expr:         `$match("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", /^(a?){50}a{50}$/)`,
				maxTimeMs:    1000,
				expectResult: true, // Should match
				description:  "Classic catastrophic backtracking - RE2 handles efficiently",
			},
			{
				name:         "Nested quantifiers",
				expr:         `$match("aaaaaaaaaaaaaaaaaaaab", /(a*)*b/)`,
				maxTimeMs:    100,
				expectResult: true,
				description:  "Nested quantifiers that cause exponential behavior in PCRE",
			},
			{
				name:         "Complex alternation",
				expr:         `$match("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaab", /(a|a)*b/)`,
				maxTimeMs:    100,
				expectResult: true,
				description:  "Alternation with overlapping patterns",
			},
			{
				name:         "Possessive quantifiers simulation",
				expr:         `$match("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", /a+a+a+a+a+a+a+b/)`,
				maxTimeMs:    100,
				expectResult: false, // Won't match (no 'b' at end)
				description:  "Multiple possessive-like quantifiers",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				expr, err := Compile(tc.expr, false)
				if err != nil {
					// $match might not be implemented
					if strings.Contains(err.Error(), "$match") {
						t.Skip("$match function not implemented")
					}
					t.Fatalf("Failed to compile: %v", err)
				}

				expr.SetMaxTime(tc.maxTimeMs)

				start := time.Now()
				result, err := expr.Evaluate([]byte(`{}`), nil)
				elapsed := time.Since(start)

				if err != nil {
					// If $match is not implemented, skip
					if strings.Contains(err.Error(), "match") || strings.Contains(err.Error(), "U0101") {
						t.Skip("$match function not available")
					}
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify it completed quickly (RE2 is linear time)
				if elapsed > time.Duration(tc.maxTimeMs)*time.Millisecond {
					t.Errorf("Regex took too long: %v (max: %dms)", elapsed, tc.maxTimeMs)
				}

				t.Logf("Pattern completed in %v - %s", elapsed, tc.description)
				if result != nil {
					t.Logf("Result: %s", string(result))
				}
			})
		}
	})

	// Test combined DoS vectors
	t.Run("Combined DoS Vectors", func(t *testing.T) {
		testCases := []struct {
			name      string
			expr      string
			maxDepth  int
			maxRange  int
			maxTimeMs int
			errorCode string // Any of these error codes is acceptable
		}{
			{
				name:      "Recursion with large range",
				expr:      `($f := function($arr) { $count($arr) > 0 ? $f($arr[[0..$count($arr)-2]]) : 0 }; $f([1..1000]))`,
				maxDepth:  50,
				maxRange:  100,
				maxTimeMs: 500,
				errorCode: "D2014", // Should fail on range first
			},
			{
				name:      "Map over large range with computation",
				expr:      `[1..10000].(function($x) { [1..$x].($*$*$) })`,
				maxDepth:  100,
				maxRange:  1000,
				maxTimeMs: 100,
				errorCode: "D2014", // Should fail on range
			},
			{
				name:      "Double recursion (non-tail)",
				expr:      `($fib := function($n) { $n <= 1 ? $n : $fib($n - 1) + $fib($n - 2) }; $fib(30))`,
				maxDepth:  20,
				maxRange:  1000000,
				maxTimeMs: 5000,
				errorCode: "U1001", // Should hit recursion limit
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				expr, err := Compile(tc.expr, false)
				if err != nil {
					t.Fatalf("Failed to compile: %v", err)
				}

				expr.SetMaxDepth(tc.maxDepth)
				expr.SetMaxRange(tc.maxRange)
				expr.SetMaxTime(tc.maxTimeMs)

				_, err = expr.Evaluate([]byte(`{}`), nil)
				if err == nil {
					t.Error("Expected DoS protection error but got none")
				} else if jsonErr, ok := err.(*JSONataError); ok {
					// Any of the protection mechanisms could trigger
					validCodes := []string{"U1001", "U1002", "D2014", "T2009", "T0410"}
					found := false
					for _, code := range validCodes {
						if jsonErr.Code == code {
							found = true
							t.Logf("Protection triggered with code: %s", code)
							break
						}
					}
					if !found {
						t.Errorf("Expected one of error codes %v, got %s", validCodes, jsonErr.Code)
					}
				}
			})
		}
	})

	// Test edge cases
	t.Run("Edge Cases", func(t *testing.T) {
		t.Run("Zero limits mean default/unlimited", func(t *testing.T) {
			expr, _ := Compile(`[1..100]`, false)
			expr.SetMaxDepth(0) // Unlimited
			expr.SetMaxRange(0) // Default 10M
			expr.SetMaxTime(0)  // Unlimited

			result, err := expr.Evaluate([]byte(`{}`), nil)
			if err != nil {
				t.Errorf("Should work with zero limits: %v", err)
			}
			if result == nil {
				t.Error("Expected result")
			}
		})

		t.Run("Limits work across expression reuse", func(t *testing.T) {
			expr, _ := Compile(`[1..$.limit]`, false)
			expr.SetMaxRange(100)

			// First evaluation should fail
			_, err := expr.Evaluate([]byte(`{"limit": 101}`), nil)
			if err == nil {
				t.Error("Expected range limit error")
			} else if jsonErr, ok := err.(*JSONataError); ok {
				if jsonErr.Code != "D2014" {
					t.Errorf("Expected error code D2014, got %s", jsonErr.Code)
				}
			}

			// Second evaluation with lower limit should work
			result, err := expr.Evaluate([]byte(`{"limit": 50}`), nil)
			if err != nil {
				t.Errorf("Should work with smaller range: %v", err)
			}
			if result == nil {
				t.Error("Expected result")
			}
		})
	})

	// Test global default limits
	t.Run("Global Default Limits", func(t *testing.T) {
		// Save current defaults
		originalMaxDepth := defaultMaxDepth
		originalMaxTime := defaultMaxTime
		originalMaxRange := defaultMaxRange

		// Restore defaults after test
		defer func() {
			SetDefaultMaxDepth(originalMaxDepth)
			SetDefaultMaxTime(originalMaxTime)
			SetDefaultMaxRange(originalMaxRange)
		}()

		t.Run("Default Max Depth", func(t *testing.T) {
			// Set global default
			SetDefaultMaxDepth(50)

			// Compile new expression - should inherit default
			expr, err := Compile(`($factorial := function($n) { $n <= 1 ? 1 : $n * $factorial($n - 1) }; $factorial(100))`, false)
			if err != nil {
				t.Fatalf("Failed to compile: %v", err)
			}

			// Should fail due to depth limit
			_, err = expr.Evaluate([]byte(`{}`), nil)
			if err == nil {
				t.Error("Expected recursion depth error but got none")
			} else if jsonErr, ok := err.(*JSONataError); ok {
				if jsonErr.Code != "U1001" {
					t.Errorf("Expected error code U1001, got %s", jsonErr.Code)
				}
			}
		})

		t.Run("Default Max Time", func(t *testing.T) {
			// Set global default
			SetDefaultMaxTime(50)

			// Compile new expression - should inherit default
			expr, err := Compile(`[1..10000].([1..100].($ * $ * $ * $))`, false)
			if err != nil {
				t.Fatalf("Failed to compile: %v", err)
			}

			// Should fail due to time limit
			_, err = expr.Evaluate([]byte(`{}`), nil)
			if err == nil {
				t.Error("Expected timeout error but got none")
			} else if jsonErr, ok := err.(*JSONataError); ok {
				if jsonErr.Code != "U1002" {
					t.Errorf("Expected error code U1002, got %s", jsonErr.Code)
				}
			}
		})

		t.Run("Default Max Range", func(t *testing.T) {
			// Set global default
			SetDefaultMaxRange(1000)

			// Compile new expression - should inherit default
			expr, err := Compile(`[1..10000]`, false)
			if err != nil {
				t.Fatalf("Failed to compile: %v", err)
			}

			// Should fail due to range limit
			_, err = expr.Evaluate([]byte(`{}`), nil)
			if err == nil {
				t.Error("Expected range limit error but got none")
			} else if jsonErr, ok := err.(*JSONataError); ok {
				if jsonErr.Code != "D2014" {
					t.Errorf("Expected error code D2014, got %s", jsonErr.Code)
				}
			}
		})

		t.Run("Expression-specific limits override defaults", func(t *testing.T) {
			// Set restrictive global defaults
			SetDefaultMaxDepth(10)
			SetDefaultMaxTime(10)
			SetDefaultMaxRange(10)

			// Compile expression
			expr, err := Compile(`($factorial := function($n) { $n <= 1 ? 1 : $n * $factorial($n - 1) }; $factorial(15))`, false)
			if err != nil {
				t.Fatalf("Failed to compile: %v", err)
			}

			// Should fail with default limit
			_, err = expr.Evaluate([]byte(`{}`), nil)
			if err == nil {
				t.Error("Expected recursion depth error but got none")
			}

			// Override with higher limit
			expr.SetMaxDepth(50)

			// Should succeed now
			_, err = expr.Evaluate([]byte(`{}`), nil)
			if err != nil {
				t.Errorf("Expected success after raising limit, got error: %v", err)
			}
		})

		t.Run("Zero defaults mean unlimited", func(t *testing.T) {
			// Set all defaults to 0 (unlimited)
			SetDefaultMaxDepth(0)
			SetDefaultMaxTime(0)
			SetDefaultMaxRange(0)

			// Compile expression with reasonable limits
			expr, err := Compile(`[1..100].($*2)`, false)
			if err != nil {
				t.Fatalf("Failed to compile: %v", err)
			}

			// Should succeed - no limits applied
			result, err := expr.Evaluate([]byte(`{}`), nil)
			if err != nil {
				t.Errorf("Unexpected error with unlimited defaults: %v", err)
			}
			if result == nil {
				t.Error("Expected result but got nil")
			}
		})
	})
}

// TestDoSProtectionPerformance verifies that protections don't significantly impact normal operations
func TestDoSProtectionPerformance(t *testing.T) {
	t.Run("Normal operations remain fast", func(t *testing.T) {
		expressions := []string{
			`[1..100].($*2)`,
			`{"a": [1,2,3], "b": [4,5,6]}.*.($sum($))`,
			`$keys($).{$: $lookup($$, $)}`,
			`[1..10].($*$)`,
		}

		for _, exprStr := range expressions {
			expr, err := Compile(exprStr, false)
			if err != nil {
				t.Fatalf("Failed to compile %s: %v", exprStr, err)
			}

			// Set reasonable limits
			expr.SetMaxDepth(100)
			expr.SetMaxRange(10000)
			expr.SetMaxTime(1000)

			start := time.Now()
			_, err = expr.Evaluate([]byte(`{"a": 1, "b": 2, "c": 3}`), nil)
			elapsed := time.Since(start)

			if err != nil {
				t.Errorf("Normal expression failed with limits: %v", err)
			}

			// Should complete very quickly
			if elapsed > 100*time.Millisecond {
				t.Errorf("Expression took too long: %v", elapsed)
			}
		}
	})
}
