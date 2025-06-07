// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

/*
JSONata Parser - Recursive Descent Expression Parser
===================================================

Portions Copyright IBM Corp. 2016, 2018 All Rights Reserved
Project name: JSONata
This project is licensed under the MIT License, see LICENSE

Overview:
--------
This module implements a recursive descent parser for the JSONata expression language.
It transforms textual JSONata expressions into Abstract Syntax Trees (ASTs) that can
be efficiently evaluated against JSON data. The parser handles JSONata's rich syntax
including path expressions, function calls, operators, and complex constructs.

Parser Architecture:
------------------

1. **Tokenizer**: Breaks input text into meaningful tokens (identifiers, operators, literals)
2. **Recursive Descent**: Uses a hierarchy of parsing functions corresponding to grammar rules
3. **Operator Precedence**: Implements precedence climbing for correct operator evaluation order
4. **Error Recovery**: Optional recovery mode for parsing malformed expressions
5. **AST Construction**: Builds structured tree representation of parsed expressions

Key Grammar Elements:
-------------------

- **Path Expressions**: Field navigation, array indexing, filtering (e.g., "person.name[0]")
- **Function Calls**: Built-in and user functions with arguments (e.g., "$sum(values)")
- **Operators**: Arithmetic, comparison, logical, and JSONata-specific operators
- **Literals**: Strings, numbers, booleans, null values
- **Lambda Functions**: Anonymous function creation with closures
- **Conditionals**: Ternary operator and if-then-else constructs
- **Array/Object Construction**: Explicit structure creation
- **Variable Binding**: Local variable assignment and scoping

Operator Precedence (highest to lowest):
--------------------------------------
- 80: Field access (.), array access ([), parentheses ((), context (@), index (#)
- 75: Object construction ({)
- 70: Range (..)
- 60: Multiplication (*), division (/), modulo (%), descendant (**)
- 50: Addition (+), subtraction (-)
- 40: Comparison (=, !=, <, >, <=, >=), sort (^)
- 20: Logical OR (|), ternary (?), range (..)
- 10: Assignment (:=)

This precedence ensures expressions like "a + b * c" parse as "a + (b * c)" correctly.

Error Handling:
--------------
The parser supports two modes:
- **Normal Mode**: Fails fast on syntax errors with detailed error messages
- **Recovery Mode**: Attempts to continue parsing past errors for IDE/tooling support

AST Node Structure:
-----------------
Each AST node contains:
- Type: The kind of expression node (e.g., "path", "function", "binary")
- Value: Node-specific data (e.g., operator symbol, literal value)
- Children: Sub-expressions that make up this expression
- Position: Source location for error reporting and debugging
- Attributes: Additional metadata for evaluation (e.g., array preservation flags)

The resulting AST preserves all semantic information needed for evaluation while
abstracting away syntactic details like whitespace and redundant parentheses.
*/

package jsonata

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Global parser instance providing JSONata expression parsing capabilities
// This singleton is initialized once and shared across all parsing operations
var parser = initParser()

/*
initParser - Initialize JSONata Expression Parser
===============================================

Creates and configures the complete parsing infrastructure for JSONata expressions.
This includes operator precedence tables, tokenization rules, and parsing functions
that implement JSONata's grammar through recursive descent.

The parser handles the full JSONata language including:
- Path expressions with complex navigation
- Function calls with argument validation
- Operator expressions with proper precedence
- Lambda functions and closures
- Conditional expressions and control flow
- Array and object construction
- Variable binding and scoping

Returns a parsing function that can transform expression strings into ASTs.
*/
func initParser() func(string, bool) (*ASTNode, error) {
	// Pre-compile regex for hex validation
	hexPattern := regexp.MustCompile("^[0-9a-fA-F]+$")

	/*
		Operator Precedence Table
		========================

		Defines the binding strength of all JSONata operators. Higher numbers
		bind more tightly, ensuring correct parsing of complex expressions.

		Special cases:
		- Closing brackets (], }, )) have precedence 0 to terminate their constructs
		- Delimiters (,, ;, :) have precedence 0 as they separate elements
		- Assignment (:=) has lowest precedence (10) to be right-associative
		- Function application operators (@, #) have high precedence (80)
	*/
	operators := map[string]int{
		".":   75, // Field access: person.name
		"[":   80, // Array access/filter: array[0], items[price > 10]
		"]":   0,  // Array close (terminator)
		"{":   70, // Object constructor: {name: value}
		"}":   0,  // Object close (terminator)
		"(":   80, // Parentheses/grouping: (expression)
		")":   0,  // Parentheses close (terminator)
		",":   0,  // Argument separator (delimiter)
		"@":   80, // Context variable binding: @var
		"#":   80, // Index variable binding: #index
		";":   80, // Statement separator in blocks
		":":   80, // Key-value separator in objects
		"?":   20, // Ternary conditional: condition ? then : else
		"+":   50, // Addition: a + b
		"-":   50, // Subtraction: a - b
		"*":   60, // Multiplication: a * b
		"/":   60, // Division: a / b
		"%":   60, // Modulo: a % b
		"|":   20, // Logical OR: a | b
		"=":   40, // Equality: a = b
		"<":   40, // Less than: a < b
		">":   40, // Greater than: a > b
		"^":   40, // Sort/order-by: array^(expression)
		"**":  60, // Descendant operator: a.**b
		"..":  20, // Range operator: 1..5
		":=":  10, // Variable assignment: $var := value
		"!=":  40, // Inequality: a != b
		"<=":  40, // Less than or equal: a <= b
		">=":  40, // Greater than or equal: a >= b,
		"~>":  40,
		"and": 30,
		"or":  25,
		"in":  40,
		"&":   50,
		"!":   0, // not an operator, but needed as a stop character for name tokens
		"~":   0, // not an operator, but needed as a stop character for name tokens
	}

	escapes := map[string]string{ // JSON string escape sequences - see json.org
		"\"": "\"",
		"\\": "\\",
		"/":  "/",
		"b":  "\b",
		"f":  "\f",
		"n":  "\n",
		"r":  "\r",
		"t":  "\t",
	}

	// Tokenizer (lexer) - invoked by the parser to return one token at a time
	// JavaScript: parser.js line 63
	tokenizer := func(path string) func(bool) (*Token, error) {
		position := 0
		length := len(path)

		create := func(type_ string, value interface{}) *Token {
			return &Token{Type: type_, Value: value, Position: position}
		}

		scanRegex := func() (*regexp.Regexp, error) {
			// the prefix '/' will have been previously scanned. Find the end of the regex.
			// search for closing '/' ignoring any that are escaped, or within brackets
			start := position
			depth := 0
			var pattern string
			var flags string

			isClosingSlash := func(position int) bool {
				if position < len(path) && path[position] == '/' && depth == 0 {
					backslashCount := 0
					for position-(backslashCount+1) >= 0 && path[position-(backslashCount+1)] == '\\' {
						backslashCount++
					}
					if backslashCount%2 == 0 {
						return true
					}
				}
				return false
			}

			for position < length {
				currentChar := path[position]
				if isClosingSlash(position) {
					// end of regex found
					pattern = path[start:position]
					if pattern == "" {
						return nil, &JSONataError{
							Code:     "S0301",
							Position: position,
						}
					}
					position++
					// flags
					start = position
					for position < length && (path[position] == 'i' || path[position] == 'm') {
						position++
					}
					flags = path[start:position] + "g"
					// In Go, we use regexp package which has different flag syntax
					// Convert JS flags to Go syntax
					if strings.Contains(flags, "i") {
						pattern = "(?i)" + pattern
					}
					if strings.Contains(flags, "m") {
						pattern = "(?m)" + pattern
					}
					return regexp.Compile(pattern)
				}
				if position > 0 && ((currentChar == '(' || currentChar == '[' || currentChar == '{') && path[position-1] != '\\') {
					depth++
				}
				if position > 0 && ((currentChar == ')' || currentChar == ']' || currentChar == '}') && path[position-1] != '\\') {
					depth--
				}

				position++
			}
			return nil, &JSONataError{
				Code:     "S0302",
				Position: position,
			}
		}

		var next func(prefix bool) (*Token, error)
		next = func(prefix bool) (*Token, error) {
			if position >= length {
				return nil, nil
			}
			currentChar := path[position]
			// skip whitespace
			for position < length && strings.ContainsRune(" \t\n\r\v", rune(currentChar)) {
				position++
				if position < length {
					currentChar = path[position]
				}
			}
			// skip comments
			if position < length-1 && currentChar == '/' && path[position+1] == '*' {
				commentStart := position
				position += 2
				if position < length {
					currentChar = path[position]
				}
				for !(position < length-1 && currentChar == '*' && path[position+1] == '/') {
					position++
					if position >= length {
						// no closing tag
						return nil, &JSONataError{
							Code:     "S0106",
							Position: commentStart,
						}
					}
					currentChar = path[position]
				}
				position += 2
				return next(prefix) // need this to swallow any following whitespace
			}
			// test for regex
			if !prefix && currentChar == '/' {
				position++
				regex, err := scanRegex()
				if err != nil {
					return nil, err
				}
				return create("regex", regex), nil
			}
			// handle double-char operators
			if position < length-1 {
				if currentChar == '.' && path[position+1] == '.' {
					// double-dot .. range operator
					position += 2
					return create("operator", ".."), nil
				}
				if currentChar == ':' && path[position+1] == '=' {
					// := assignment
					position += 2
					return create("operator", ":="), nil
				}
				if currentChar == '!' && path[position+1] == '=' {
					// !=
					position += 2
					return create("operator", "!="), nil
				}
				if currentChar == '>' && path[position+1] == '=' {
					// >=
					position += 2
					return create("operator", ">="), nil
				}
				if currentChar == '<' && path[position+1] == '=' {
					// <=
					position += 2
					return create("operator", "<="), nil
				}
				if currentChar == '*' && path[position+1] == '*' {
					// **  descendant wildcard
					position += 2
					return create("operator", "**"), nil
				}
				if currentChar == '~' && path[position+1] == '>' {
					// ~>  chain function
					position += 2
					return create("operator", "~>"), nil
				}
			}
			// test for single char operators
			if _, ok := operators[string(currentChar)]; ok {
				position++
				return create("operator", string(currentChar)), nil
			}
			// test for string literals
			if currentChar == '"' || currentChar == '\'' {
				quoteType := currentChar
				// double quoted string literal - find end of string
				position++
				qstr := ""
				for position < length {
					currentChar = path[position]
					if currentChar == '\\' { // escape sequence
						position++
						if position >= length {
							break
						}
						currentChar = path[position]
						if esc, ok := escapes[string(currentChar)]; ok {
							qstr += esc
						} else if currentChar == 'u' {
							// \u should be followed by 4 hex digits
							if position+4 < length {
								octets := path[position+1 : position+5]
								if hexPattern.MatchString(octets) {
									codepoint, err := strconv.ParseInt(octets, 16, 32)
									if err != nil {
										return nil, &JSONataError{
											Code:     "S0104",
											Position: position,
										}
									}
									qstr += string(rune(codepoint))
									position += 4
								} else {
									return nil, &JSONataError{
										Code:     "S0104",
										Position: position,
									}
								}
							} else {
								return nil, &JSONataError{
									Code:     "S0104",
									Position: position,
								}
							}
						} else {
							// illegal escape sequence
							return nil, &JSONataError{
								Code:     "S0103",
								Position: position,
								Token:    string(currentChar),
							}
						}
					} else if currentChar == quoteType {
						position++
						return create("string", qstr), nil
					} else {
						// CRITICAL FIX: Decode UTF-8 rune instead of using single byte
						r, size := utf8.DecodeRuneInString(path[position:])
						if r == utf8.RuneError && size == 1 {
							// Invalid UTF-8 sequence
							return nil, &JSONataError{
								Code:     "S0104",
								Position: position,
							}
						}
						qstr += string(r)
						position += size - 1 // -1 because we'll increment position at the end of the loop
					}
					position++
				}
				return nil, &JSONataError{
					Code:     "S0101",
					Position: position,
				}
			}
			// test for numbers
			numregex := regexp.MustCompile(`^-?(0|([1-9][0-9]*))(\.[0-9]+)?([Ee][-+]?[0-9]+)?`)
			if position < length {
				match := numregex.FindString(path[position:])
				if match != "" {
					num, err := strconv.ParseFloat(match, 64)
					if err == nil && !math.IsNaN(num) && !math.IsInf(num, 0) {
						position += len(match)
						return create("number", num), nil
					} else {
						return nil, &JSONataError{
							Code:     "S0102",
							Position: position,
							Token:    match,
						}
					}
				}
			}
			// test for quoted names (backticks)
			var name string
			if currentChar == '`' {
				// scan for closing quote
				position++
				end := strings.IndexByte(path[position:], '`')
				if end != -1 {
					name = path[position : position+end]
					position = position + end + 1
					return create("name", name), nil
				}
				position = length
				return nil, &JSONataError{
					Code:     "S0105",
					Position: position,
				}
			}
			// test for names
			i := position
			var ch byte
			for {
				if i < length {
					ch = path[i]
				}
				_, isOperator := operators[string(ch)]
				if i == length || strings.ContainsRune(" \t\n\r\v", rune(ch)) || isOperator {
					if position < length && path[position] == '$' {
						// variable reference
						name = path[position+1 : i]
						position = i
						return create("variable", name), nil
					} else {
						name = path[position:i]
						position = i
						switch name {
						case "or", "in", "and":
							return create("operator", name), nil
						case "true":
							return create("value", true), nil
						case "false":
							return create("value", false), nil
						case "null":
							return create("value", JSONNull), nil
						default:
							if position == length && name == "" {
								// whitespace at end of input
								return nil, nil
							}
							return create("name", name), nil
						}
					}
				} else {
					i++
				}
			}
		}

		return next
	}

	// This parser implements the 'Top down operator precedence' algorithm developed by Vaughan R Pratt; http://dl.acm.org/citation.cfm?id=512931.
	// and builds on the Javascript framework described by Douglas Crockford at http://javascript.crockford.com/tdop/tdop.html
	// and in 'Beautiful Code', edited by Andy Oram and Greg Wilson, Copyright 2007 O'Reilly Media, Inc. 798-0-596-51004-6

	// JavaScript: parser.js line 338
	parser := func(source string, recovery bool) (ast *ASTNode, err error) {
		var node *Symbol
		var lexer func(bool) (*Token, error)

		symbolTable := make(map[string]*Symbol)
		errors := []error{}

		remainingTokens := func() ([]*Token, error) {
			remaining := []*Token{}
			if node.id != "(end)" {
				remaining = append(remaining, &Token{Type: node.type_, Value: node.value, Position: node.position})
			}
			nxt, err := lexer(false)
			for nxt != nil && err == nil {
				remaining = append(remaining, nxt)
				nxt, err = lexer(false)
			}
			if err != nil {
				return remaining, err
			}
			return remaining, nil
		}

		baseSymbol := &Symbol{
			nud: func(s *Symbol) (*ASTNode, error) {
				// error - symbol has been invoked as a unary operator
				err := &JSONataError{
					Code:     "S0211",
					Token:    fmt.Sprintf("%v", s.value),
					Position: s.position,
				}

				if recovery {
					remaining, remainingErr := remainingTokens()
					if remainingErr != nil {
						err.Remaining = []*Token{}
					} else {
						err.Remaining = remaining
					}
					err.Type = "error"
					errors = append(errors, err)
					return &ASTNode{Type: "error", Error: err}, nil
				} else {
					// Return nil AST and the error to stop parsing
					return nil, err
				}
			},
		}

		// JavaScript: parser.js line 379
		symbol := func(id string, bp int) *Symbol {
			s, ok := symbolTable[id]
			if ok {
				if bp >= s.lbp {
					s.lbp = bp
				}
			} else {
				s = &Symbol{
					id:    id,
					value: id,
					lbp:   bp,
					nud:   baseSymbol.nud,
					led:   baseSymbol.led,
				}
				symbolTable[id] = s
			}
			return s
		}

		// JavaScript: parser.js line 395
		handleError := func(parseErr *JSONataError) (*ASTNode, error) {
			if recovery {
				// tokenize the rest of the buffer and add it to an error token
				remaining, remainingErr := remainingTokens()
				if remainingErr != nil {
					// If we can't get remaining tokens, just use what we have
					parseErr.Remaining = []*Token{}
				} else {
					parseErr.Remaining = remaining
				}
				errors = append(errors, parseErr)
				sym := symbolTable["(error)"]
				node = &Symbol{
					id:       sym.id,
					value:    sym.value,
					lbp:      sym.lbp,
					nud:      sym.nud,
					led:      sym.led,
					type_:    "(error)",
					position: parseErr.Position,
				}
				return &ASTNode{Type: "(error)", Error: parseErr}, nil
			} else {
				// Return nil AST and the error to stop parsing
				return nil, parseErr
			}
		}

		// JavaScript: parser.js line 411
		advance := func(id string, infix bool) (*ASTNode, error) {
			if id != "" && node.id != id {
				var code string
				if node.id == "(end)" {
					// unexpected end of buffer
					code = "S0203"
				} else {
					code = "S0202"
				}
				err := &JSONataError{
					Code:     code,
					Position: node.position,
					Token:    fmt.Sprintf("%v", node.value),
					Value:    id,
				}
				return handleError(err)
			}
			nextToken, lexErr := lexer(infix)
			if lexErr != nil {
				if jsonataErr, ok := lexErr.(*JSONataError); ok {
					return handleError(jsonataErr)
				} else {
					return handleError(&JSONataError{
						Code:     "S0104", // Generic lexer error
						Position: 0,
						Message:  lexErr.Error(),
					})
				}
			}
			if nextToken == nil {
				node = symbolTable["(end)"]
				if node == nil {
					node = &Symbol{id: "(end)", value: "(end)"}
				}
				node.position = len(source)
				return nil, nil
			}
			value := nextToken.Value
			type_ := nextToken.Type
			var sym *Symbol
			switch type_ {
			case "name", "variable":
				sym = symbolTable["(name)"]
			case "operator":
				var ok bool
				sym, ok = symbolTable[value.(string)]
				if !ok {
					return handleError(&JSONataError{
						Code:     "S0204",
						Position: nextToken.Position,
						Token:    fmt.Sprintf("%v", value),
					})
				}
			case "string", "number", "value":
				sym = symbolTable["(literal)"]
			case "regex":
				type_ = "regex"
				sym = symbolTable["(regex)"]
			default:
				/* istanbul ignore next */
				return handleError(&JSONataError{
					Code:     "S0205",
					Position: nextToken.Position,
					Token:    fmt.Sprintf("%v", value),
				})
			}

			node = &Symbol{
				id:       sym.id,
				value:    value,
				lbp:      sym.lbp,
				nud:      sym.nud,
				led:      sym.led,
				type_:    type_,
				position: nextToken.Position,
			}
			return nil, nil
		}

		// Pratt's algorithm
		// JavaScript: parser.js line 480
		expression := func(rbp int) (*ASTNode, error) {
			var left *ASTNode
			var err error
			t := node
			_, err = advance("", true)
			if err != nil {
				return nil, err
			}
			left, err = t.nud(t)
			if err != nil {
				return nil, err
			}
			for rbp < node.lbp {
				t = node
				_, err = advance("", false)
				if err != nil {
					return nil, err
				}
				left, err = t.led(t, left)
				if err != nil {
					return nil, err
				}
			}
			return left, nil
		}

		terminal := func(id string) {
			s := symbol(id, 0)
			s.nud = func(self *Symbol) (*ASTNode, error) {
				return self.toASTNode(), nil
			}
		}

		// match infix operators
		// <expression> <operator> <expression>
		// left associative
		infix := func(id string, bp int, led func(*Symbol, *ASTNode) (*ASTNode, error)) *Symbol {
			bindingPower := bp
			if bindingPower == 0 {
				bindingPower = operators[id]
			}
			s := symbol(id, bindingPower)
			if led != nil {
				s.led = led
			} else {
				s.led = func(self *Symbol, left *ASTNode) (*ASTNode, error) {
					rhs, err := expression(bindingPower)
					if err != nil {
						return nil, err
					}
					node := self.toASTNode()
					node.Type = "binary"
					node.LHS = left
					node.RHS = rhs
					return node, nil
				}
			}
			return s
		}

		// match infix operators
		// <expression> <operator> <expression>
		// right associative
		infixr := func(id string, bp int, led func(*Symbol, *ASTNode) (*ASTNode, error)) *Symbol {
			s := symbol(id, bp)
			s.led = led
			return s
		}

		// match prefix operators
		// <operator> <expression>
		prefix := func(id string, nud func(*Symbol) (*ASTNode, error)) *Symbol {
			s := symbol(id, 0)
			if nud != nil {
				s.nud = nud
			} else {
				s.nud = func(self *Symbol) (*ASTNode, error) {
					expr, err := expression(70)
					if err != nil {
						return nil, err
					}
					node := self.toASTNode()
					node.Type = "unary"
					node.Expression = expr
					return node, nil
				}
			}
			return s
		}

		terminal("(end)")
		terminal("(name)")
		terminal("(literal)")
		terminal("(regex)")
		symbol(":", 0)
		symbol(";", 0)
		symbol(",", 0)
		symbol(")", 0)
		symbol("]", 0)
		symbol("}", 0)
		symbol("..", 0)      // range operator
		infix(".", 0, nil)   // map operator
		infix("+", 0, nil)   // numeric addition
		infix("-", 0, nil)   // numeric subtraction
		infix("*", 0, nil)   // numeric multiplication
		infix("/", 0, nil)   // numeric division
		infix("%", 0, nil)   // numeric modulus
		infix("=", 0, nil)   // equality
		infix("<", 0, nil)   // less than
		infix(">", 0, nil)   // greater than
		infix("!=", 0, nil)  // not equal to
		infix("<=", 0, nil)  // less than or equal
		infix(">=", 0, nil)  // greater than or equal
		infix("&", 0, nil)   // string concatenation
		infix("and", 0, nil) // Boolean AND
		infix("or", 0, nil)  // Boolean OR
		infix("in", 0, nil)  // is member of array
		terminal("and")      // the 'keywords' can also be used as terminals (field names)
		terminal("or")       //
		terminal("in")       //
		prefix("-", nil)     // unary numeric negation
		infix("~>", 0, nil)  // function application

		infixr("(error)", 10, func(self *Symbol, left *ASTNode) (*ASTNode, error) {
			remaining, remainingErr := remainingTokens()
			if remainingErr != nil {
				remaining = []*Token{}
			}
			return &ASTNode{
				Type:      "error",
				LHS:       left,
				Error:     node.error,
				Remaining: remaining,
			}, nil
		})

		// field wildcard (single level)
		prefix("*", func(self *Symbol) (*ASTNode, error) {
			node := self.toASTNode()
			node.Type = "wildcard"
			return node, nil
		})

		// descendant wildcard (multi-level)
		prefix("**", func(self *Symbol) (*ASTNode, error) {
			node := self.toASTNode()
			node.Type = "descendant"
			return node, nil
		})

		// parent operator
		prefix("%", func(self *Symbol) (*ASTNode, error) {
			node := self.toASTNode()
			node.Type = "parent"
			return node, nil
		})

		// function invocation
		infix("(", operators["("], func(self *Symbol, left *ASTNode) (*ASTNode, error) {
			// left is is what we are trying to invoke
			result := self.toASTNode()
			result.Type = "function"
			result.Procedure = left
			result.Arguments = []*ASTNode{}
			if node.id != ")" {
				for {
					if node.type_ == "operator" && node.id == "?" {
						// partial function application
						result.Type = "partial"
						result.Arguments = append(result.Arguments, &ASTNode{
							Type:  "operator",
							Value: "?",
						})
						_, err := advance("?", false)
						if err != nil {
							return nil, err
						}
					} else {
						arg, err := expression(0)
						if err != nil {
							return nil, err
						}
						result.Arguments = append(result.Arguments, arg)
					}
					if node.id != "," {
						break
					}
					_, err := advance(",", false)
					if err != nil {
						return nil, err
					}
				}
			}
			_, err := advance(")", true)
			if err != nil {
				return nil, err
			}
			// if the name of the function is 'function' or λ, then this is function definition (lambda function)
			if left.Type == "name" && (left.Value == "function" || left.Value == "λ") {
				// all of the args must be VARIABLE tokens
				for index, arg := range result.Arguments {
					if arg.Type != "variable" {
						return handleError(&JSONataError{
							Code:     "S0208",
							Position: arg.Position,
							Token:    fmt.Sprintf("%v", arg.Value),
							Value:    index + 1,
						})
					}
				}
				result.Type = "lambda"
				// is the next token a '<' - if so, parse the function signature
				if node.id == "<" {
					sigPos := node.position
					_ = sigPos // currently unused
					depth := 1
					sig := "<"
					for depth > 0 && node.id != "{" && node.id != "(end)" {
						tok, err := advance("", false)
						if err != nil {
							return nil, err
						}
						if tok != nil && node.id == ">" {
							depth--
						} else if tok != nil && node.id == "<" {
							depth++
						}
						sig += fmt.Sprintf("%v", node.value)
					}
					validator, err := parseSignature(sig)
					if err != nil {
						// Preserve specific error codes from parseSignature
						if jsonataErr, ok := err.(*JSONataError); ok {
							jsonataErr.Position = sigPos
							return handleError(jsonataErr)
						}
						// Generic signature error fallback
						return handleError(&JSONataError{
							Code:     "S0104",
							Position: sigPos,
							Token:    sig,
						})
					}
					if validator != nil {
						result.Signature = validator
					}
				}
				// parse the function body
				_, err = advance("{", false)
				if err != nil {
					return nil, err
				}
				result.Body, err = expression(0)
				if err != nil {
					return nil, err
				}
				_, err = advance("}", false)
				if err != nil {
					return nil, err
				}
			}
			return result, nil
		})

		// parenthesis - block expression
		prefix("(", func(self *Symbol) (*ASTNode, error) {
			expressions := []*ASTNode{}
			for node.id != ")" {
				expr, err := expression(0)
				if err != nil {
					return nil, err
				}
				expressions = append(expressions, expr)
				if node.id != ";" {
					break
				}
				_, err = advance(";", false)
				if err != nil {
					return nil, err
				}
			}
			_, err := advance(")", true)
			if err != nil {
				return nil, err
			}
			node := self.toASTNode()
			node.Type = "block"
			node.Expressions = expressions
			return node, nil
		})

		// array constructor
		prefix("[", func(self *Symbol) (*ASTNode, error) {
			a := []*ASTNode{}
			if node.id != "]" {
				for {
					item, err := expression(0)
					if err != nil {
						return nil, err
					}
					if node.id == ".." {
						// range operator
						rangeNode := &ASTNode{
							Type:     "binary",
							Value:    "..",
							Position: node.position,
							LHS:      item,
						}
						_, err = advance("..", false)
						if err != nil {
							return nil, err
						}
						rangeNode.RHS, err = expression(0)
						if err != nil {
							return nil, err
						}
						item = rangeNode
					}
					a = append(a, item)
					if node.id != "," {
						break
					}
					_, err = advance(",", false)
					if err != nil {
						return nil, err
					}
				}
			}
			_, err = advance("]", true)
			if err != nil {
				return nil, err
			}
			node := self.toASTNode()
			node.Type = "unary"
			node.Value = "["
			node.Expressions = a
			return node, nil
		})

		// filter - predicate or array index
		infix("[", operators["["], func(self *Symbol, left *ASTNode) (*ASTNode, error) {
			if node.id == "]" {
				// empty predicate means maintain singleton arrays in the output
				step := left
				for step != nil && step.Type == "binary" && step.Value == "[" {
					step = step.LHS
				}
				step.KeepArray = true
				_, err := advance("]", false)
				if err != nil {
					return nil, err
				}
				return left, nil
			} else {
				rhs, err := expression(operators["]"])
				if err != nil {
					return nil, err
				}
				result := self.toASTNode()
				result.Type = "binary"
				result.Value = "["
				result.LHS = left
				result.RHS = rhs
				_, err = advance("]", true)
				if err != nil {
					return nil, err
				}
				return result, nil
			}
		})

		// order-by
		infix("^", operators["^"], func(self *Symbol, left *ASTNode) (*ASTNode, error) {
			_, err := advance("(", false)
			if err != nil {
				return nil, err
			}
			terms := []*SortTerm{}
			for {
				term := &SortTerm{
					Descending: false,
				}
				if node.id == "<" {
					// ascending sort
					_, err = advance("<", false)
					if err != nil {
						return nil, err
					}
				} else if node.id == ">" {
					// descending sort
					term.Descending = true
					_, err = advance(">", false)
					if err != nil {
						return nil, err
					}
				} else {
					//unspecified - default to ascending
				}
				term.Expression, err = expression(0)
				if err != nil {
					return nil, err
				}
				terms = append(terms, term)
				if node.id != "," {
					break
				}
				_, err := advance(",", false)
				if err != nil {
					return nil, err
				}
			}
			_, err = advance(")", false)
			if err != nil {
				return nil, err
			}
			result := self.toASTNode()
			result.Type = "binary"
			result.Value = "^"
			result.LHS = left
			result.RHSTerms = terms
			return result, nil
		})

		objectParser := func(self *Symbol, left *ASTNode) (*ASTNode, error) {
			a := [][2]*ASTNode{}
			if node.id != "}" {
				for {
					n, err := expression(0)
					if err != nil {
						return nil, err
					}
					_, err = advance(":", false)
					if err != nil {
						return nil, err
					}
					v, err := expression(0)
					if err != nil {
						return nil, err
					}
					a = append(a, [2]*ASTNode{n, v}) // holds an array of name/value expression pairs
					if node.id != "," {
						break
					}
					_, err = advance(",", false)
					if err != nil {
						return nil, err
					}
				}
			}
			_, err := advance("}", true)
			if err != nil {
				return nil, err
			}
			if left == nil {
				// NUD - unary prefix form
				node := self.toASTNode()
				node.Type = "unary"
				node.Value = "{"
				node.LHSPairs = a
				return node, nil
			} else {
				// LED - binary infix form
				node := self.toASTNode()
				node.Type = "binary"
				node.Value = "{"
				node.LHS = left
				node.RHSPairs = a
				return node, nil
			}
		}

		// object constructor
		prefix("{", func(self *Symbol) (*ASTNode, error) {
			return objectParser(self, nil)
		})

		// object grouping
		infix("{", operators["{"], objectParser)

		// bind variable
		infixr(":=", operators[":="], func(self *Symbol, left *ASTNode) (*ASTNode, error) {
			if left.Type != "variable" {
				return handleError(&JSONataError{
					Code:     "S0212",
					Position: left.Position,
					Token:    fmt.Sprintf("%v", left.Value),
				})
			}
			rhs, err := expression(operators[":="] - 1) // subtract 1 from bindingPower for right associative operators
			if err != nil {
				return nil, err
			}
			node := self.toASTNode()
			node.Type = "binary"
			node.Value = ":="
			node.LHS = left
			node.RHS = rhs
			return node, nil
		})

		// focus variable bind
		infix("@", operators["@"], func(self *Symbol, left *ASTNode) (*ASTNode, error) {
			rhs, err := expression(operators["@"])
			if err != nil {
				return nil, err
			}
			result := self.toASTNode()
			result.Type = "binary"
			result.Value = "@"
			result.LHS = left
			result.RHS = rhs
			if result.RHS.Type != "variable" {
				return handleError(&JSONataError{
					Code:     "S0214",
					Position: result.RHS.Position,
					Token:    "@",
				})
			}
			return result, nil
		})

		// index (position) variable bind
		infix("#", operators["#"], func(self *Symbol, left *ASTNode) (*ASTNode, error) {
			rhs, err := expression(operators["#"])
			if err != nil {
				return nil, err
			}
			result := self.toASTNode()
			result.Type = "binary"
			result.Value = "#"
			result.LHS = left
			result.RHS = rhs
			if result.RHS.Type != "variable" {
				return handleError(&JSONataError{
					Code:     "S0214",
					Position: result.RHS.Position,
					Token:    "#",
				})
			}
			return result, nil
		})

		// if/then/else ternary operator ?:
		infix("?", operators["?"], func(self *Symbol, left *ASTNode) (*ASTNode, error) {
			thenExpr, err := expression(0)
			if err != nil {
				return nil, err
			}
			result := self.toASTNode()
			result.Type = "condition"
			result.Condition = left
			result.Then = thenExpr
			if node.id == ":" {
				// else condition
				_, err = advance(":", false)
				if err != nil {
					return nil, err
				}
				result.Else, err = expression(0)
				if err != nil {
					return nil, err
				}
			}
			return result, nil
		})

		// object transformer
		prefix("|", func(self *Symbol) (*ASTNode, error) {
			pattern, err := expression(0)
			if err != nil {
				return nil, err
			}
			result := self.toASTNode()
			result.Type = "transform"
			result.Pattern = pattern
			_, err = advance("|", false)
			if err != nil {
				return nil, err
			}
			result.Update, err = expression(0)
			if err != nil {
				return nil, err
			}
			if node.id == "," {
				_, err = advance(",", false)
				if err != nil {
					return nil, err
				}
				result.Delete, err = expression(0)
				if err != nil {
					return nil, err
				}
			}
			_, err = advance("|", false)
			if err != nil {
				return nil, err
			}
			return result, nil
		})

		// tail call optimization
		// this is invoked by the post parser to analyse lambda functions to see
		// if they make a tail call.  If so, it is replaced by a thunk which will
		// be invoked by the trampoline loop during function application.
		// This enables tail-recursive functions to be written without growing the stack
		var tailCallOptimize func(*ASTNode) *ASTNode
		tailCallOptimize = func(expr *ASTNode) *ASTNode {
			var result *ASTNode
			if expr.Type == "function" && expr.Predicate == nil {
				thunk := &ASTNode{
					Type:      "lambda",
					Thunk:     true,
					Arguments: []*ASTNode{},
					Position:  expr.Position,
				}
				thunk.Body = expr
				result = thunk
			} else if expr.Type == "condition" {
				// analyse both branches
				expr.Then = tailCallOptimize(expr.Then)
				if expr.Else != nil {
					expr.Else = tailCallOptimize(expr.Else)
				}
				result = expr
			} else if expr.Type == "block" {
				// only the last expression in the block
				length := len(expr.Expressions)
				if length > 0 {
					expr.Expressions[length-1] = tailCallOptimize(expr.Expressions[length-1])
				}
				result = expr
			} else {
				result = expr
			}
			return result
		}

		ancestorLabel := 0
		ancestorIndex := 0
		ancestry := []*ASTNode{}

		var seekParent func(*ASTNode, *AncestorSlot) (*AncestorSlot, error)
		seekParent = func(node *ASTNode, slot *AncestorSlot) (*AncestorSlot, error) {
			switch node.Type {
			case "name", "wildcard":
				slot.Level--
				if slot.Level == 0 {
					if node.Ancestor == nil {
						node.Ancestor = slot
					} else {
						// reuse the existing label
						ancestry[slot.Index].Slot.Label = node.Ancestor.Label
						node.Ancestor = slot
					}
					node.Tuple = true
				}
			case "parent":
				slot.Level++
			case "block":
				// look in last expression in the block
				if len(node.Expressions) > 0 {
					node.Tuple = true
					var err error
					slot, err = seekParent(node.Expressions[len(node.Expressions)-1], slot)
					if err != nil {
						return nil, err
					}
				}
			case "path":
				// last step in path
				node.Tuple = true
				index := len(node.Steps) - 1
				var err error
				slot, err = seekParent(node.Steps[index], slot)
				if err != nil {
					return nil, err
				}
				index--
				for slot.Level > 0 && index >= 0 {
					// check previous steps
					var err error
					slot, err = seekParent(node.Steps[index], slot)
					if err != nil {
						return nil, err
					}
					index--
				}
			case "unary":
				// unary expressions like object/array constructors can't provide parent context
				return nil, &JSONataError{
					Code:     "S0217",
					Token:    node.Type,
					Position: node.Position,
				}
			default:
				// error - can't derive ancestor
				return nil, &JSONataError{
					Code:     "S0217",
					Token:    node.Type,
					Position: node.Position,
				}
			}
			return slot, nil
		}

		pushAncestry := func(result, value *ASTNode) {
			if value.SeekingParent != nil || value.Type == "parent" {
				slots := []*AncestorSlot{}
				if value.SeekingParent != nil {
					slots = value.SeekingParent
				}
				if value.Type == "parent" {
					slots = append(slots, value.Slot)
				}
				if result.SeekingParent == nil {
					result.SeekingParent = slots
				} else {
					result.SeekingParent = append(result.SeekingParent, slots...)
				}
			}
		}

		resolveAncestry := func(path *ASTNode) error {
			index := len(path.Steps) - 1
			laststep := path.Steps[index]
			slots := []*AncestorSlot{}
			if laststep.SeekingParent != nil {
				slots = laststep.SeekingParent
			}
			if laststep.Type == "parent" {
				slots = append(slots, laststep.Slot)
			}
			for is := 0; is < len(slots); is++ {
				slot := slots[is]
				index = len(path.Steps) - 2
				for slot.Level > 0 {
					if index < 0 {
						if path.SeekingParent == nil {
							path.SeekingParent = []*AncestorSlot{slot}
						} else {
							path.SeekingParent = append(path.SeekingParent, slot)
						}
						break
					}
					// try previous step
					step := path.Steps[index]
					index--
					// multiple contiguous steps that bind the focus should be skipped
					for index >= 0 && step.Focus != "" && path.Steps[index].Focus != "" {
						step = path.Steps[index]
						index--
					}
					var err error
					slot, err = seekParent(step, slot)
					if err != nil {
						return err
					}
				}
			}
			return nil
		}

		// post-parse stage
		// the purpose of this is to add as much semantic value to the parse tree as possible
		// in order to simplify the work of the evaluator.
		// This includes flattening the parts of the AST representing location paths,
		// converting them to arrays of steps which in turn may contain arrays of predicates.
		// following this, nodes containing '.' and '[' should be eliminated from the AST.
		// JavaScript: parser.js line 998
		var processAST func(*ASTNode) (*ASTNode, error)
		processAST = func(expr *ASTNode) (*ASTNode, error) {
			var result *ASTNode
			switch expr.Type {
			case "binary":
				switch expr.Value {
				case ".":
					lstep, err := processAST(expr.LHS)
					if err != nil {
						return nil, err
					}

					if lstep.Type == "path" {
						result = lstep
					} else {
						result = &ASTNode{Type: "path", Steps: []*ASTNode{lstep}}
					}
					if lstep.Type == "parent" {
						result.SeekingParent = []*AncestorSlot{lstep.Slot}
					}
					rest, err := processAST(expr.RHS)
					if err != nil {
						return nil, err
					}
					if rest.Type == "function" &&
						rest.Procedure.Type == "path" &&
						len(rest.Procedure.Steps) == 1 &&
						rest.Procedure.Steps[0].Type == "name" &&
						len(result.Steps) > 0 &&
						result.Steps[len(result.Steps)-1].Type == "function" {
						// next function in chain of functions - will override a thenable
						result.Steps[len(result.Steps)-1].NextFunction = rest.Procedure.Steps[0].Value.(string)
					}
					if rest.Type == "path" {
						result.Steps = append(result.Steps, rest.Steps...)
					} else {
						if rest.Predicate != nil {
							rest.Stages = rest.Predicate
							rest.Predicate = nil
						}
						result.Steps = append(result.Steps, rest)
					}
					// any steps within a path that are string literals, should be changed to 'name'
					for _, step := range result.Steps {
						if step.Type == "number" || step.Type == "value" {
							// don't allow steps to be numbers or the values true/false/null
							return handleError(&JSONataError{
								Code:     "S0213",
								Position: step.Position,
								Value:    step.Value,
							})
						}
						if step.Type == "string" {
							step.Type = "name"
						}
					}
					// any step that signals keeping a singleton array, should be flagged on the path
					for _, step := range result.Steps {
						if step.KeepArray {
							result.KeepSingletonArray = true
							break
						}
					}
					// if first step is a path constructor, flag it for special handling
					if len(result.Steps) > 0 {
						firststep := result.Steps[0]
						if firststep.Type == "unary" && firststep.Value == "[" {
							firststep.ConsArray = true
						}
					}
					// if the last step is an array constructor, flag it so it doesn't flatten
					if len(result.Steps) > 0 {
						laststep := result.Steps[len(result.Steps)-1]
						if laststep.Type == "unary" && laststep.Value == "[" {
							laststep.ConsArray = true
						}
					}
					if err := resolveAncestry(result); err != nil {
						return nil, err
					}
				case "[":
					// predicated step
					// LHS is a step or a predicated step
					// RHS is the predicate expr
					result, err = processAST(expr.LHS)
					if err != nil {
						return nil, err
					}
					step := result
					type_ := "predicate"
					if result.Type == "path" {
						step = result.Steps[len(result.Steps)-1]
						type_ = "stages"
					}
					if step.Group != nil {
						return handleError(&JSONataError{
							Code:     "S0209",
							Position: expr.Position,
						})
					}
					predicate, err := processAST(expr.RHS)
					if err != nil {
						return nil, err
					}
					if predicate.SeekingParent != nil {
						for _, slot := range predicate.SeekingParent {
							if slot.Level == 1 {
								_, err := seekParent(step, slot)
								if err != nil {
									return nil, err
								}
							} else {
								slot.Level--
							}
						}
						pushAncestry(step, predicate)
					}
					if type_ == "predicate" {
						if step.Predicate == nil {
							step.Predicate = []*Stage{}
						}
						step.Predicate = append(step.Predicate, &Stage{Type: "filter", Expr: predicate, Position: expr.Position})
					} else {
						if step.Stages == nil {
							step.Stages = []*Stage{}
						}
						step.Stages = append(step.Stages, &Stage{Type: "filter", Expr: predicate, Position: expr.Position})
					}
				case "{":
					// group-by
					// LHS is a step or a predicated step
					// RHS is the object constructor expr
					result, err = processAST(expr.LHS)
					if err != nil {
						return nil, err
					}
					if result.Group != nil {
						return handleError(&JSONataError{
							Code:     "S0210",
							Position: expr.Position,
						})
					}
					// object constructor - process each pair
					lhsPairs := [][2]*ASTNode{}
					for _, pair := range expr.RHSPairs {
						key, err := processAST(pair[0])
						if err != nil {
							return nil, err
						}
						value, err := processAST(pair[1])
						if err != nil {
							return nil, err
						}
						lhsPairs = append(lhsPairs, [2]*ASTNode{key, value})
					}
					result.Group = &Group{
						LHS:      lhsPairs,
						Position: expr.Position,
					}
				case "^":
					// order-by
					// LHS is the array to be ordered
					// RHS defines the terms
					result, err = processAST(expr.LHS)
					if err != nil {
						return nil, err
					}
					if result.Type != "path" {
						result = &ASTNode{Type: "path", Steps: []*ASTNode{result}}
					}
					sortStep := &ASTNode{Type: "sort", Position: expr.Position}
					sortStep.Terms = []*SortTerm{}
					for _, term := range expr.RHSTerms {
						expression, err := processAST(term.Expression)
						if err != nil {
							return nil, err
						}
						pushAncestry(sortStep, expression)
						sortStep.Terms = append(sortStep.Terms, &SortTerm{
							Descending: term.Descending,
							Expression: expression,
						})
					}
					result.Steps = append(result.Steps, sortStep)
					if err := resolveAncestry(result); err != nil {
						return nil, err
					}
				case ":=":
					result = &ASTNode{Type: "bind", Value: expr.Value, Position: expr.Position}
					result.LHS, err = processAST(expr.LHS)
					if err != nil {
						return nil, err
					}
					result.RHS, err = processAST(expr.RHS)
					if err != nil {
						return nil, err
					}
					pushAncestry(result, result.RHS)
				case "@":
					result, err = processAST(expr.LHS)
					if err != nil {
						return nil, err
					}
					step := result
					if result.Type == "path" {
						step = result.Steps[len(result.Steps)-1]
					}
					// throw error if there are any predicates defined at this point
					// at this point the only type of stages can be predicates
					if step.Stages != nil || step.Predicate != nil {
						return handleError(&JSONataError{
							Code:     "S0215",
							Position: expr.Position,
						})
					}
					// also throw if this is applied after an 'order-by' clause
					if step.Type == "sort" {
						return handleError(&JSONataError{
							Code:     "S0216",
							Position: expr.Position,
						})
					}
					if expr.KeepArray {
						step.KeepArray = true
					}
					step.Focus = expr.RHS.Value.(string)
					step.Tuple = true
				case "#":
					result, err = processAST(expr.LHS)
					if err != nil {
						return nil, err
					}
					step := result
					if result.Type == "path" {
						if len(result.Steps) > 0 {
							step = result.Steps[len(result.Steps)-1]
						}
					} else {
						result = &ASTNode{Type: "path", Steps: []*ASTNode{result}}
						if step.Predicate != nil {
							step.Stages = step.Predicate
							step.Predicate = nil
						}
					}
					if step.Stages == nil {
						step.Index = expr.RHS.Value.(string)
					} else {
						step.Stages = append(step.Stages, &Stage{Type: "index", Value: expr.RHS.Value.(string), Position: expr.Position})
					}
					step.Tuple = true
				case "~>":
					result = &ASTNode{Type: "apply", Value: expr.Value, Position: expr.Position}
					result.LHS, err = processAST(expr.LHS)
					if err != nil {
						return nil, err
					}
					result.RHS, err = processAST(expr.RHS)
					if err != nil {
						return nil, err
					}
					result.KeepArray = result.LHS.KeepArray || result.RHS.KeepArray
				default:
					result = &ASTNode{Type: expr.Type, Value: expr.Value, Position: expr.Position}
					result.LHS, err = processAST(expr.LHS)
					if err != nil {
						return nil, err
					}
					result.RHS, err = processAST(expr.RHS)
					if err != nil {
						return nil, err
					}
					pushAncestry(result, result.LHS)
					pushAncestry(result, result.RHS)
				}
			case "unary":
				result = &ASTNode{Type: expr.Type, Value: expr.Value, Position: expr.Position}
				if expr.Value == "[" {
					// array constructor - process each item
					result.Expressions = []*ASTNode{}
					for _, item := range expr.Expressions {
						value, err := processAST(item)
						if err != nil {
							return nil, err
						}
						pushAncestry(result, value)
						result.Expressions = append(result.Expressions, value)
					}
				} else if expr.Value == "{" {
					// object constructor - process each pair
					result.LHSPairs = [][2]*ASTNode{}
					for _, pair := range expr.LHSPairs {
						key, err := processAST(pair[0])
						if err != nil {
							return nil, err
						}
						pushAncestry(result, key)
						value, err := processAST(pair[1])
						if err != nil {
							return nil, err
						}
						pushAncestry(result, value)
						result.LHSPairs = append(result.LHSPairs, [2]*ASTNode{key, value})
					}
				} else {
					// all other unary expressions - just process the expression
					result.Expression, err = processAST(expr.Expression)
					if err != nil {
						return nil, err
					}
					// if unary minus on a number, then pre-process
					if expr.Value == "-" && result.Expression.Type == "number" {
						result = result.Expression
						if num, ok := result.Value.(float64); ok {
							result.Value = -num
						}
					} else {
						pushAncestry(result, result.Expression)
					}
				}
			case "function", "partial":
				result = &ASTNode{Type: expr.Type, Name: expr.Name, Value: expr.Value, Position: expr.Position}
				result.Arguments = []*ASTNode{}
				for _, arg := range expr.Arguments {
					argAST, err := processAST(arg)
					if err != nil {
						return nil, err
					}
					pushAncestry(result, argAST)
					result.Arguments = append(result.Arguments, argAST)
				}
				result.Procedure, err = processAST(expr.Procedure)
				if err != nil {
					return nil, err
				}
			case "lambda":
				result = &ASTNode{
					Type:      expr.Type,
					Arguments: expr.Arguments,
					Signature: expr.Signature,
					Position:  expr.Position,
				}
				body, err := processAST(expr.Body)
				if err != nil {
					return nil, err
				}
				result.Body = tailCallOptimize(body)
			case "condition":
				result = &ASTNode{Type: expr.Type, Position: expr.Position}
				result.Condition, err = processAST(expr.Condition)
				if err != nil {
					return nil, err
				}
				pushAncestry(result, result.Condition)
				result.Then, err = processAST(expr.Then)
				if err != nil {
					return nil, err
				}
				pushAncestry(result, result.Then)
				if expr.Else != nil {
					result.Else, err = processAST(expr.Else)
					if err != nil {
						return nil, err
					}
					pushAncestry(result, result.Else)
				}
			case "transform":
				result = &ASTNode{Type: expr.Type, Position: expr.Position}
				result.Pattern, err = processAST(expr.Pattern)
				if err != nil {
					return nil, err
				}
				result.Update, err = processAST(expr.Update)
				if err != nil {
					return nil, err
				}
				if expr.Delete != nil {
					result.Delete, err = processAST(expr.Delete)
					if err != nil {
						return nil, err
					}
				}
			case "block":
				result = &ASTNode{Type: expr.Type, Position: expr.Position}
				// array of expressions - process each one
				result.Expressions = []*ASTNode{}
				for _, item := range expr.Expressions {
					part, err := processAST(item)
					if err != nil {
						return nil, err
					}
					pushAncestry(result, part)
					if part.ConsArray || (part.Type == "path" && len(part.Steps) > 0 && part.Steps[0].ConsArray) {
						result.ConsArray = true
					}
					result.Expressions = append(result.Expressions, part)
				}
				// TODO scan the array of expressions to see if any of them assign variables (JavaScript: parser.js line 1287)
				// if so, need to mark the block as one that needs to create a new frame
			case "name":
				result = &ASTNode{Type: "path", Steps: []*ASTNode{expr}}
				if expr.KeepArray {
					result.KeepSingletonArray = true
				}
			case "parent":
				result = &ASTNode{
					Type: "parent",
					Slot: &AncestorSlot{
						Label: fmt.Sprintf("!%d", ancestorLabel),
						Level: 1,
						Index: ancestorIndex,
					},
				}
				ancestorLabel++
				ancestorIndex++
				ancestry = append(ancestry, result)
			case "string", "number", "value", "wildcard", "descendant", "variable", "regex":
				result = expr
			case "operator":
				// the tokens 'and' and 'or' might have been used as a name rather than an operator
				if expr.Value == "and" || expr.Value == "or" || expr.Value == "in" {
					expr.Type = "name"
					result, err = processAST(expr)
					if err != nil {
						return nil, err
					}
				} else /* istanbul ignore else */ if expr.Value == "?" {
					// partial application
					result = expr
				} else {
					return handleError(&JSONataError{
						Code:     "S0201",
						Position: expr.Position,
						Token:    fmt.Sprintf("%v", expr.Value),
					})
				}
			case "error":
				result = expr
				if expr.LHS != nil {
					result, err = processAST(expr.LHS)
					if err != nil {
						return nil, err
					}
				}
			default:
				code := "S0206"
				/* istanbul ignore else */
				if expr.ID == "(end)" {
					code = "S0207"
				}
				err := &JSONataError{
					Code:     code,
					Position: expr.Position,
					Token:    fmt.Sprintf("%v", expr.Value),
				}
				if recovery {
					errors = append(errors, err)
					return &ASTNode{Type: "error", Error: err}, nil
				} else {
					return handleError(err)
				}
			}
			if expr.KeepArray {
				result.KeepArray = true
			}
			return result, nil
		}

		// now invoke the tokenizer and the parser and return the syntax tree

		lexer = tokenizer(source)
		_, err = advance("", false)
		if err != nil {
			return nil, err
		}
		// parse the tokens
		expr, err := expression(0)
		if err != nil {
			return nil, err
		}
		if node.id != "(end)" {
			err := &JSONataError{
				Code:     "S0201",
				Position: node.position,
				Token:    fmt.Sprintf("%v", node.value),
			}
			_, handleErr := handleError(err)
			if handleErr != nil {
				return nil, handleErr
			}
		}
		expr, err = processAST(expr)
		if err != nil {
			return nil, err
		}

		if expr.Type == "parent" || expr.SeekingParent != nil {
			// error - trying to derive ancestor at top level
			err = &JSONataError{
				Code:     "S0217",
				Token:    expr.Type,
				Position: expr.Position,
			}
			return nil, err
		}

		if len(errors) > 0 {
			expr.Errors = errors
		}

		if err != nil {
			return nil, err
		}
		ast = expr
		return ast, err
	}

	return parser
}

// Parser types

// Token represents a lexical token
type Token struct {
	Type     string
	Value    interface{}
	Position int
}

// Symbol represents a parser symbol
type Symbol struct {
	id       string
	value    interface{}
	lbp      int
	nud      func(*Symbol) (*ASTNode, error)
	led      func(*Symbol, *ASTNode) (*ASTNode, error)
	type_    string
	position int
	error    *JSONataError
}

// toASTNode converts a Symbol to an ASTNode, copying all relevant fields.
// This is used when JavaScript returns 'this' from a symbol method.
func (s *Symbol) toASTNode() *ASTNode {
	return &ASTNode{
		ID:       s.id,
		Type:     s.type_,
		Value:    s.value,
		Position: s.position,
	}
}

// ASTNode represents a node in the abstract syntax tree
type ASTNode struct {
	Type               string
	Value              interface{}
	Position           int
	LHS                *ASTNode
	RHS                *ASTNode
	RHSTerms           []*SortTerm
	RHSPairs           [][2]*ASTNode
	LHSPairs           [][2]*ASTNode
	Expressions        []*ASTNode
	Expression         *ASTNode
	Arguments          []*ASTNode
	Procedure          *ASTNode
	Steps              []*ASTNode
	Focus              string
	Index              string
	Pattern            *ASTNode
	Update             *ASTNode
	Delete             *ASTNode
	Condition          *ASTNode
	Then               *ASTNode
	Else               *ASTNode
	Body               *ASTNode
	Name               string
	ID                 string
	Predicate          []*Stage
	Stages             []*Stage
	Terms              []*SortTerm
	Group              *Group
	KeepArray          bool
	KeepSingletonArray bool
	ConsArray          bool
	Tuple              bool
	NextFunction       string
	Thunk              bool
	Signature          *SignatureValidator
	Ancestor           *AncestorSlot
	SeekingParent      []*AncestorSlot
	Slot               *AncestorSlot
	Error              error
	Errors             []error
	Remaining          []*Token
}

// Stage represents a predicate or index stage
type Stage struct {
	Type     string
	Expr     *ASTNode
	Value    string
	Position int
}

// SortTerm represents a sort term
type SortTerm struct {
	Descending bool
	Expression *ASTNode
}

// Group represents a group-by clause
type Group struct {
	LHS                 [][2]*ASTNode
	Position            int
	IsObjectConstructor bool // True if this group is from an object constructor expression
}

// AncestorSlot represents an ancestor reference
type AncestorSlot struct {
	Label string
	Level int
	Index int
}

// Parse parses a JSONata expression string and returns an AST
// This is the main exported function that takes an expression string and returns an AST
func Parse(expr string) (*ASTNode, error) {
	return parser(expr, false)
}

// ParseWithRecovery parses a JSONata expression string with error recovery enabled
// Returns an AST that may contain error nodes for invalid syntax
func ParseWithRecovery(expr string) (*ASTNode, error) {
	return parser(expr, true)
}
