// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

/**
 * Test file for verifying Go parser implementation matches JavaScript implementation
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
 * This test file executes JavaScript parser from jsonata-js/src/parser.js
 * and compares the results with the Go implementation to verify correctness
 * of the transliteration.
 */

package jsonata

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// executeParserJS runs JavaScript code and returns the result
func executeParserJS(jsCode string) (string, error) {
	cmd := exec.Command("node", "-e", jsCode)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("JS execution error: %v, output: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// Test parser with various expressions
func TestParser(t *testing.T) {
	testCases := []struct {
		name       string
		expression string
		shouldErr  bool
	}{
		// Basic literals
		{"number literal", "42", false},
		{"float literal", "3.14", false},
		{"string literal", `"hello"`, false},
		{"boolean true", "true", false},
		{"boolean false", "false", false},
		{"null literal", "null", false},

		// Basic operators
		{"addition", "1 + 2", false},
		{"subtraction", "5 - 3", false},
		{"multiplication", "4 * 3", false},
		{"division", "10 / 2", false},
		{"modulo", "10 % 3", false},
		{"concatenation", `"hello" & " world"`, false},

		// Comparison operators
		{"equals", "1 = 1", false},
		{"not equals", "1 != 2", false},
		{"less than", "1 < 2", false},
		{"greater than", "2 > 1", false},
		{"less than or equal", "1 <= 2", false},
		{"greater than or equal", "2 >= 1", false},

		// Logical operators
		{"and", "true and false", false},
		{"or", "true or false", false},
		{"not", "not true", true},

		// Path expressions
		{"simple path", "$.name", false},
		{"nested path", "$.person.name", false},
		{"array index", "$.items[0]", false},
		{"wildcard", "$.items[*]", false},
		{"descendant", "**", false},
		{"filter expression", "$.items[price < 10]", false},

		// Array constructor
		{"empty array", "[]", false},
		{"array with elements", "[1, 2, 3]", false},
		{"nested array", "[[1, 2], [3, 4]]", false},

		// Object constructor
		{"empty object", "{}", false},
		{"object with properties", `{"name": "John", "age": 30}`, false},
		{"computed property", `{("na" & "me"): "John"}`, false},

		// Function calls
		{"function no args", "$count()", false},
		{"function with args", "$sum(1, 2, 3)", false},
		{"nested function", "$sum($count(), 5)", false},

		// Lambda expressions
		{"simple lambda", "function($x) { $x * 2 }", false},
		{"lambda with multiple params", "function($x, $y) { $x + $y }", false},

		// Conditional expression
		{"ternary", "true ? 1 : 2", false},
		{"nested ternary", "true ? (false ? 1 : 2) : 3", false},

		// Range operator
		{"range", "[1..10]", false},

		// Partial function application
		{"partial application", "$sum(?, 2)", false},

		// Sorting and grouping
		{"sort expression", "$ ^(name)", false},
		{"group expression", "$ {name: age}", false},

		// Transform operator
		{"transform", "$ ~> |account.order|{'product': product.name}|", false},

		// Error cases
		{"unclosed string", `"hello`, true},
		{"unclosed array", "[1, 2", true},
		{"unclosed object", "{name:", true},
		{"invalid operator", "1 ++ 2", true},
		{"invalid path", "$.", true},
		{"missing closing paren", "sum(1, 2", true},
	}

	for _, tc := range testCases {
		// Properly escape the expression as a JSON string for JavaScript
		escapedExpr, _ := json.Marshal(tc.expression)

		// Test JavaScript parser
		jsCode := fmt.Sprintf(`
			const parser = require('./jsonata-js/src/parser.js');
			try {
				const ast = parser(%s);
				// Convert AST to a simplified structure for comparison
				const simplifyAST = (node) => {
					if (!node) return null;
					const simplified = {
						type: node.type,
						value: node.value
					};
					// Include some key properties based on type
					if (node.type === 'literal' || node.type === 'number' || node.type === 'string') {
						simplified.value = node.value;
					} else if (node.type === 'unary' || node.type === 'binary') {
						simplified.value = node.value;
						simplified.lhs = simplifyAST(node.lhs);
						simplified.rhs = simplifyAST(node.rhs);
						simplified.expression = simplifyAST(node.expression);
					} else if (node.type === 'path') {
						simplified.steps = node.steps ? node.steps.map(simplifyAST) : undefined;
					} else if (node.type === 'name') {
						simplified.value = node.value;
					} else if (node.type === 'function') {
						simplified.name = node.name;
						simplified.arguments = node.arguments ? node.arguments.map(simplifyAST) : undefined;
					} else if (node.type === 'lambda') {
						simplified.arguments = node.arguments;
						simplified.body = simplifyAST(node.body);
					} else if (node.type === 'condition') {
						simplified.condition = simplifyAST(node.condition);
						simplified.then = simplifyAST(node.then);
						simplified.else = simplifyAST(node.else);
					} else if (node.type === 'block') {
						simplified.expressions = node.expressions ? node.expressions.map(simplifyAST) : undefined;
					} else if (node.type === 'array') {
						simplified.expressions = node.expressions ? node.expressions.map(simplifyAST) : undefined;
					}
					return simplified;
				};
				console.log(JSON.stringify({success: true, type: ast.type}));
			} catch (e) {
				console.log(JSON.stringify({error: e.message || e.code || "parse error"}));
			}
		`, escapedExpr)

		jsOutput, err := executeParserJS(jsCode)
		if err != nil {
			t.Fatalf("Failed to execute JavaScript for %s: %v", tc.expression, err)
		}

		// Parse JS result
		var jsResult struct {
			Success bool   `json:"success"`
			Type    string `json:"type"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal([]byte(jsOutput), &jsResult); err != nil {
			t.Fatalf("Failed to parse JS result for %s: %v (output: %s)", tc.name, err, jsOutput)
		}

		// Test Go parser
		goErr := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					if e, ok := r.(*JSONataError); ok {
						err = e
					} else {
						err = fmt.Errorf("%v", r)
					}
				}
			}()
			ast, err := Parse(tc.expression)
			// If we got here and err is nil, parsing succeeded
			_ = ast
			if err != nil {
				return err
			}
			return nil
		}()

		// Compare results
		jsErrored := jsResult.Error != ""
		goErrored := goErr != nil

		if tc.shouldErr {
			// Both should error
			if !jsErrored && !goErrored {
				t.Errorf("Parser(%q): Expected error but both JS and Go succeeded", tc.expression)
			} else if !jsErrored && goErrored {
				t.Errorf("Parser(%q): Expected error - Go errored (%v) but JS succeeded", tc.expression, goErr)
			} else if jsErrored && !goErrored {
				t.Errorf("Parser(%q): Expected error - JS errored (%s) but Go succeeded", tc.expression, jsResult.Error)
			}
		} else {
			// Both should succeed
			if jsErrored && goErrored {
				t.Errorf("Parser(%q): Both errored - JS: %s, Go: %v", tc.expression, jsResult.Error, goErr)
			} else if jsErrored && !goErrored {
				t.Errorf("Parser(%q): JS errored (%s) but Go succeeded", tc.expression, jsResult.Error)
			} else if !jsErrored && goErrored {
				t.Errorf("Parser(%q): Go errored (%v) but JS succeeded", tc.expression, goErr)
			}
		}
	}
}

// Test parser error codes match between JS and Go
func TestParserErrorCodes(t *testing.T) {
	testCases := []struct {
		name       string
		expression string
		errorCode  string // Expected error code
	}{
		{"unterminated string", `"hello`, "S0101"},
		{"unexpected token", "1 ++ 2", "S0102"},
		{"unexpected end", "1 +", "S0203"},
		{"invalid number", "123abc", "S0102"},
		{"missing property after dot", "$.", "S0204"},
		{"unclosed bracket", "[1, 2", "S0205"},
		{"unclosed brace", "{a: 1", "S0209"},
		{"unclosed parenthesis", "(1 + 2", "S0208"},
	}

	for _, tc := range testCases {
		// Properly escape the expression as a JSON string for JavaScript
		escapedExpr, _ := json.Marshal(tc.expression)

		// Test JavaScript parser
		jsCode := fmt.Sprintf(`
			const parser = require('./jsonata-js/src/parser.js');
			try {
				parser(%s);
				console.log(JSON.stringify({success: true}));
			} catch (e) {
				console.log(JSON.stringify({error: e.code || "UNKNOWN"}));
			}
		`, escapedExpr)

		jsOutput, err := executeParserJS(jsCode)
		if err != nil {
			t.Fatalf("Failed to execute JavaScript for %s: %v", tc.expression, err)
		}

		// Parse JS result
		var jsResult struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal([]byte(jsOutput), &jsResult); err != nil {
			t.Fatalf("Failed to parse JS result for %s: %v", tc.name, err)
		}

		// Test Go parser
		var goErrorCode string
		_, err = Parse(tc.expression)
		if err != nil {
			if e, ok := err.(*JSONataError); ok {
				goErrorCode = e.Code
			} else {
				goErrorCode = "UNKNOWN"
			}
		}

		// Compare error codes
		if jsResult.Error != goErrorCode {
			t.Errorf("Parser error code mismatch for %q: JS returned %q, Go returned %q",
				tc.expression, jsResult.Error, goErrorCode)
		}
	}
}

// Test tokenizer specifically (lexical analysis)
func TestTokenizer(t *testing.T) {
	testCases := []struct {
		name       string
		expression string
	}{
		// Various token types
		{"numbers", "123 456.789 -42 0"},
		{"strings", `"hello" 'world' "with \"quotes\""`},
		{"identifiers", "name $variable $_test $123"},
		{"operators", "+ - * / % = != < > <= >= & and or not in"},
		{"delimiters", "( ) [ ] { } , ; : . .. ? |"},
		{"whitespace handling", "  a  \t  b  \n  c  "},
		{"comments", "/* comment */ 42 /* another */ + /* more */ 3"},
	}

	for _, tc := range testCases {
		// For tokenizer testing, we just verify both parsers can handle the input
		// without errors (or with the same errors)

		// Test JavaScript
		// Properly escape the expression as a JSON string for JavaScript
		escapedExpr, _ := json.Marshal(tc.expression)
		jsCode := fmt.Sprintf(`
			const parser = require('./jsonata-js/src/parser.js');
			try {
				parser(%s);
				console.log("OK");
			} catch (e) {
				console.log("ERROR: " + (e.code || e.message));
			}
		`, escapedExpr)

		jsOutput, err := executeParserJS(jsCode)
		if err != nil {
			t.Fatalf("Failed to execute JavaScript for %s: %v", tc.expression, err)
		}

		// Test Go
		goResult := "OK"
		_, err = Parse(tc.expression)
		if err != nil {
			if e, ok := err.(*JSONataError); ok {
				goResult = "ERROR: " + e.Code
			} else {
				goResult = fmt.Sprintf("ERROR: %v", err)
			}
		}

		// Both should have same result (OK or same error)
		if strings.HasPrefix(jsOutput, "OK") != strings.HasPrefix(goResult, "OK") {
			t.Errorf("Tokenizer mismatch for %q: JS: %s, Go: %s", tc.expression, jsOutput, goResult)
		}
	}
}

// Test operator precedence
func TestOperatorPrecedence(t *testing.T) {
	testCases := []struct {
		expression string
		expected   interface{} // Expected result when evaluated
	}{
		// Arithmetic precedence
		{"2 + 3 * 4", 14.0},    // multiplication before addition
		{"(2 + 3) * 4", 20.0},  // parentheses override
		{"10 - 4 - 2", 4.0},    // left associative
		{"2 * 3 * 4", 24.0},    // left associative
		{"12 / 2 / 3", 2.0},    // left associative
		{"2 + 3 * 4 - 5", 9.0}, // * before + and -

		// Comparison and logical precedence
		{"1 < 2 and 3 < 4", true},
		{"1 < 2 or 3 > 4", true},
		{"not 1 < 2", false}, // not has lower precedence than <
		{"1 = 1 and 2 = 2", true},

		// Mixed operators
		{"2 * 3 > 5", true},          // arithmetic before comparison
		{"true and 1 + 1 = 2", true}, // arithmetic before comparison before logical
	}

	for _, tc := range testCases {
		// We can't easily test the AST structure, but we can test that both
		// parsers produce expressions that evaluate to the same result

		// For now, just verify both can parse without error
		jsCode := fmt.Sprintf(`
			const parser = require('./jsonata-js/src/parser.js');
			try {
				const ast = parser("%s");
				console.log(JSON.stringify({success: true}));
			} catch (e) {
				console.log(JSON.stringify({error: e.code || e.message}));
			}
		`, tc.expression)

		jsOutput, err := executeParserJS(jsCode)
		if err != nil {
			t.Fatalf("Failed to execute JavaScript for %s: %v", tc.expression, err)
		}

		var jsResult struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		json.Unmarshal([]byte(jsOutput), &jsResult)

		// Test Go parser
		_, goErr := Parse(tc.expression)

		if jsResult.Success && goErr != nil {
			t.Errorf("Precedence parse error for %q: Go failed but JS succeeded", tc.expression)
		} else if !jsResult.Success && goErr == nil {
			t.Errorf("Precedence parse error for %q: JS failed but Go succeeded", tc.expression)
		}
	}
}
