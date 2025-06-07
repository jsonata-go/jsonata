// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package jsonata

import (
	"testing"
	"time"
)

func TestEvaluationTimeout(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		data        string
		maxTimeMs   int
		shouldError bool
		errorCode   string
	}{
		{
			name:        "Simple expression completes within timeout",
			expression:  `$.name`,
			data:        `{"name": "John"}`,
			maxTimeMs:   100,
			shouldError: false,
		},
		{
			name:        "Large array processing times out",
			expression:  `[1..100000].$string($)`,
			data:        `{}`,
			maxTimeMs:   5,
			shouldError: true,
			errorCode:   "U1002",
		},
		{
			name:        "Nested loops time out",
			expression:  `[1..100].([1..100].([1..100].$))`,
			data:        `{}`,
			maxTimeMs:   10,
			shouldError: true,
			errorCode:   "U1002",
		},
		{
			name:        "No timeout when maxTime is 0",
			expression:  `[1..1000].$string($)`,
			data:        `{}`,
			maxTimeMs:   0,
			shouldError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expr, err := Compile(test.expression, false)
			if err != nil {
				t.Fatalf("Failed to compile expression: %v", err)
			}

			// Set the timeout
			expr.SetMaxTime(test.maxTimeMs)

			// Record start time
			start := time.Now()

			// Evaluate the expression
			_, err = expr.Evaluate([]byte(test.data), nil)

			// Record elapsed time
			elapsed := time.Since(start)

			if test.shouldError {
				if err == nil {
					t.Errorf("Expected timeout error but got none")
				} else if jsonErr, ok := err.(*JSONataError); ok {
					if jsonErr.Code != test.errorCode {
						t.Errorf("Expected error code %s, got %s", test.errorCode, jsonErr.Code)
					}
					if jsonErr.Message != "Evaluation timeout exceeded" {
						t.Errorf("Expected error message 'Evaluation timeout exceeded', got '%s'", jsonErr.Message)
					}
				} else {
					t.Errorf("Expected JSONataError, got %T: %v", err, err)
				}

				// Verify that the evaluation was stopped within reasonable time
				maxAllowedTime := time.Duration(test.maxTimeMs+50) * time.Millisecond // Allow 50ms grace
				if elapsed > maxAllowedTime {
					t.Errorf("Evaluation took too long to timeout: %v (max allowed: %v)", elapsed, maxAllowedTime)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTimeoutInDifferentScenarios(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		data       string
		maxTimeMs  int
	}{
		{
			name:       "Timeout in path evaluation",
			expression: `$.[$ != null].([1..10000].$)`,
			data:       `[1,2,3,4,5]`,
			maxTimeMs:  5,
		},
		{
			name:       "Timeout in function evaluation",
			expression: `$map([1..10000], function($v){[1..1000].$string($)})`,
			data:       `{}`,
			maxTimeMs:  10,
		},
		{
			name:       "Timeout in filter evaluation",
			expression: `[1..100000][$string($) = "50000"]`,
			data:       `{}`,
			maxTimeMs:  5,
		},
		{
			name:       "Timeout in group expression",
			expression: `[1..10000]{$string($): $}`,
			data:       `{}`,
			maxTimeMs:  5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expr, err := Compile(test.expression, false)
			if err != nil {
				t.Fatalf("Failed to compile expression: %v", err)
			}

			expr.SetMaxTime(test.maxTimeMs)

			_, err = expr.Evaluate([]byte(test.data), nil)
			if err == nil {
				t.Errorf("Expected timeout error but got none")
			} else if jsonErr, ok := err.(*JSONataError); ok {
				if jsonErr.Code != "U1002" {
					t.Errorf("Expected error code U1002, got %s", jsonErr.Code)
				}
			}
		})
	}
}
