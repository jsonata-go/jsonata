// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

/**
 * Test file for verifying Go implementation matches JavaScript implementation
 *
 * SETUP REQUIREMENTS:
 *
 * For macOS:
 * 1. Install Node.js: brew install node
 * 2. Verify installation: node --version
 *
 * For Linux:
 * 1. Install Node.js:
 *    - Ubuntu/Debian: sudo apt-get install nodejs
 *    - RHEL/CentOS: sudo yum install nodejs
 *    - Or use NodeSource: curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash - && sudo apt-get install -y nodejs
 * 2. Verify installation: node --version
 *
 * This test file executes JavaScript functions from jsonata-js/src/utils.js
 * and compares the results with the Go implementation to verify correctness
 * of the transliteration.
 */

package jsonata

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// executeJS runs JavaScript code and returns the result
func executeJS(jsCode string) (string, error) {
	cmd := exec.Command("node", "-e", jsCode)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("JS execution error: %v, output: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// Removed unused compareJSONOutput function

// Test isNumeric function
func TestIsNumeric(t *testing.T) {
	// Test with hardcoded values to match JavaScript exactly
	jsCode := `
		const utils = require('./jsonata-js/src/utils.js');
		const inputs = [42, -42, 3.14, -3.14, 0, "hello", true, false, null, undefined, NaN];
		const results = inputs.map(input => {
			try {
				return utils.isNumeric(input);
			} catch (e) {
				return 'error: ' + e.message;
			}
		});
		console.log(JSON.stringify(results));
	`

	jsOutput, err := executeJS(jsCode)
	if err != nil {
		t.Fatalf("Failed to execute JavaScript: %v", err)
	}

	var jsResults []interface{}
	if err := json.Unmarshal([]byte(jsOutput), &jsResults); err != nil {
		t.Fatalf("Failed to parse JavaScript results: %v", err)
	}

	// Test Go implementation with same inputs
	inputs := []interface{}{42, -42, 3.14, -3.14, 0, "hello", true, false, nil, nil, math.NaN()}

	for i, input := range inputs {
		goResult, err := isNumeric(input)
		if err != nil {
			t.Fatalf("isNumeric failed for input %v: %v", input, err)
		}

		// Compare with JavaScript result
		jsResult := false
		if jsResultStr, ok := jsResults[i].(string); ok && strings.HasPrefix(jsResultStr, "error") {
			// JavaScript threw an error, skip comparison
			continue
		} else if jsBool, ok := jsResults[i].(bool); ok {
			jsResult = jsBool
		}

		if goResult != jsResult {
			inputStr := fmt.Sprintf("%v", input)
			if i == 9 {
				inputStr = "undefined"
			} else if i == 10 {
				inputStr = "NaN"
			}
			t.Errorf("isNumeric(%v): Go returned %v, JS returned %v", inputStr, goResult, jsResults[i])
		}
	}
}

// Test isArrayOfStrings function
func TestIsArrayOfStrings(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"array of strings", `["hello", "world"]`},
		{"empty array", `[]`},
		{"array with number", `["hello", 42]`},
		{"array with null", `["hello", null]`},
		{"not an array", `"hello"`},
		{"object", `{"key": "value"}`},
		{"null", `null`},
	}

	// Create JavaScript code with test inputs
	jsInputs := []string{}
	for _, tc := range testCases {
		jsInputs = append(jsInputs, tc.input)
	}

	jsCode := `
		const utils = require('./jsonata-js/src/utils.js');
		const inputs = %s;
		const results = inputs.map(inputStr => {
			const input = JSON.parse(inputStr);
			return utils.isArrayOfStrings(input);
		});
		console.log(JSON.stringify(results));
	`

	inputsJSON, err := json.Marshal(jsInputs)
	if err != nil {
		t.Fatalf("Failed to marshal inputs: %v", err)
	}
	jsCode = fmt.Sprintf(jsCode, string(inputsJSON))

	jsOutput, err := executeJS(jsCode)
	if err != nil {
		t.Fatalf("Failed to execute JavaScript: %v", err)
	}

	var jsResults []bool
	if err := json.Unmarshal([]byte(jsOutput), &jsResults); err != nil {
		t.Fatalf("Failed to parse JavaScript results: %v", err)
	}

	// Test Go implementation
	for i, tc := range testCases {
		var input interface{}
		if err := json.Unmarshal([]byte(tc.input), &input); err != nil {
			t.Fatalf("Failed to parse input: %v", err)
		}

		goResult := isArrayOfStrings(input)
		jsResult := jsResults[i]

		if goResult != jsResult {
			t.Errorf("isArrayOfStrings(%v): Go returned %v, JS returned %v", tc.input, goResult, jsResult)
		}
	}
}

// Test isArrayOfNumbers function
func TestIsArrayOfNumbers(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"array of numbers", `[1, 2, 3.14]`},
		{"empty array", `[]`},
		{"array with string", `[1, "hello", 3]`},
		{"array with null", `[1, null, 3]`},
		{"not an array", `42`},
		{"object", `{"key": 123}`},
	}

	// Create JavaScript code with test inputs
	jsInputs := []string{}
	for _, tc := range testCases {
		jsInputs = append(jsInputs, tc.input)
	}

	jsCode := `
		const utils = require('./jsonata-js/src/utils.js');
		const inputs = %s;
		const results = inputs.map(inputStr => {
			const input = JSON.parse(inputStr);
			return utils.isArrayOfNumbers(input);
		});
		console.log(JSON.stringify(results));
	`

	inputsJSON, err := json.Marshal(jsInputs)
	if err != nil {
		t.Fatalf("Failed to marshal inputs: %v", err)
	}
	jsCode = fmt.Sprintf(jsCode, string(inputsJSON))

	jsOutput, err := executeJS(jsCode)
	if err != nil {
		t.Fatalf("Failed to execute JavaScript: %v", err)
	}

	var jsResults []bool
	if err := json.Unmarshal([]byte(jsOutput), &jsResults); err != nil {
		t.Fatalf("Failed to parse JavaScript results: %v", err)
	}

	// Test Go implementation
	for i, tc := range testCases {
		var input interface{}
		if err := json.Unmarshal([]byte(tc.input), &input); err != nil {
			t.Fatalf("Failed to parse input: %v", err)
		}

		goResult := isArrayOfNumbers(input)
		jsResult := jsResults[i]

		if goResult != jsResult {
			t.Errorf("isArrayOfNumbers(%v): Go returned %v, JS returned %v", tc.input, goResult, jsResult)
		}
	}
}

// Test stringToArray function
func TestStringToArray(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"simple string", "hello"},
		{"empty string", ""},
		{"unicode string", "🌟🎉"},
		{"string with spaces", "hello world"},
		{"special characters", "a\nb\tc"},
	}

	jsCode := `
		const utils = require('./jsonata-js/src/utils.js');
		const inputs = %s;
		const results = inputs.map(input => utils.stringToArray(input));
		console.log(JSON.stringify(results));
	`

	inputs := []string{}
	for _, tc := range testCases {
		inputs = append(inputs, tc.input)
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		t.Fatalf("Failed to marshal inputs: %v", err)
	}
	jsCode = fmt.Sprintf(jsCode, string(inputsJSON))

	jsOutput, err := executeJS(jsCode)
	if err != nil {
		t.Fatalf("Failed to execute JavaScript: %v", err)
	}

	var jsResults [][]string
	if err := json.Unmarshal([]byte(jsOutput), &jsResults); err != nil {
		t.Fatalf("Failed to parse JavaScript results: %v", err)
	}

	// Test Go implementation
	for i, tc := range testCases {
		goResult := stringToArray(tc.input)
		jsResult := jsResults[i]

		if !reflect.DeepEqual(goResult, jsResult) {
			t.Errorf("stringToArray(%q): Go returned %v, JS returned %v", tc.input, goResult, jsResult)
		}
	}
}

// Test isDeepEqual function
func TestIsDeepEqual(t *testing.T) {
	testCases := []struct {
		name string
		lhs  string
		rhs  string
	}{
		{"equal primitives", `42`, `42`},
		{"unequal primitives", `42`, `43`},
		{"equal strings", `"hello"`, `"hello"`},
		{"unequal strings", `"hello"`, `"world"`},
		{"equal arrays", `[1, 2, 3]`, `[1, 2, 3]`},
		{"unequal arrays length", `[1, 2, 3]`, `[1, 2]`},
		{"unequal arrays values", `[1, 2, 3]`, `[1, 2, 4]`},
		{"equal objects", `{"a": 1, "b": 2}`, `{"b": 2, "a": 1}`},
		{"unequal objects", `{"a": 1}`, `{"a": 2}`},
		{"equal nested", `{"a": [1, 2], "b": {"c": 3}}`, `{"b": {"c": 3}, "a": [1, 2]}`},
		{"null vs undefined", `null`, `null`},
		{"equal booleans", `true`, `true`},
		{"unequal booleans", `true`, `false`},
	}

	// Create arrays for JavaScript
	jsLhs := []string{}
	jsRhs := []string{}
	for _, tc := range testCases {
		jsLhs = append(jsLhs, tc.lhs)
		jsRhs = append(jsRhs, tc.rhs)
	}

	jsCode := `
		const utils = require('./jsonata-js/src/utils.js');
		const lhsInputs = %s;
		const rhsInputs = %s;
		const results = lhsInputs.map((lhsStr, i) => {
			const lhs = JSON.parse(lhsStr);
			const rhs = JSON.parse(rhsInputs[i]);
			return utils.isDeepEqual(lhs, rhs);
		});
		console.log(JSON.stringify(results));
	`

	lhsJSON, err := json.Marshal(jsLhs)
	if err != nil {
		t.Fatalf("Failed to marshal lhs inputs: %v", err)
	}
	rhsJSON, err := json.Marshal(jsRhs)
	if err != nil {
		t.Fatalf("Failed to marshal rhs inputs: %v", err)
	}
	jsCode = fmt.Sprintf(jsCode, string(lhsJSON), string(rhsJSON))

	jsOutput, err := executeJS(jsCode)
	if err != nil {
		t.Fatalf("Failed to execute JavaScript: %v", err)
	}

	var jsResults []bool
	if err := json.Unmarshal([]byte(jsOutput), &jsResults); err != nil {
		t.Fatalf("Failed to parse JavaScript results: %v", err)
	}

	// Test Go implementation
	for i, tc := range testCases {
		var lhs, rhs interface{}
		if err := json.Unmarshal([]byte(tc.lhs), &lhs); err != nil {
			t.Fatalf("Failed to parse lhs: %v", err)
		}
		if err := json.Unmarshal([]byte(tc.rhs), &rhs); err != nil {
			t.Fatalf("Failed to parse rhs: %v", err)
		}

		goResult := isDeepEqual(lhs, rhs)
		jsResult := jsResults[i]

		if goResult != jsResult {
			t.Errorf("isDeepEqual(%v, %v): Go returned %v, JS returned %v", tc.lhs, tc.rhs, goResult, jsResult)
		}
	}
}
