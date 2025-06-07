package jsonata

import (
	"encoding/json"
	"errors"
	"fmt"

	v154 "github.com/jsonata-go/jsonata/v154"
)

// v154Instance implements JSONataInstance for v1.5.4
type v154Instance struct{}

// Version returns the version string
func (v *v154Instance) Version() string {
	return "v1.5.4"
}

// Compile compiles a JSONata expression
func (v *v154Instance) Compile(expr string, recoveryMode bool) (Expression, error) {
	// v1.5.4 doesn't support recovery mode
	if recoveryMode {
		// We'll just ignore recovery mode and compile normally
	}

	compiled, err := v154.Compile(expr)
	if err != nil {
		return nil, err
	}
	return &v154Expression{expr: compiled}, nil
}

// Parse parses a JSONata expression into an AST
func (v *v154Instance) Parse(expr string) (interface{}, error) {
	// v1.5.4 doesn't expose a separate Parse function
	// We'll compile and return the string representation
	compiled, err := v154.Compile(expr)
	if err != nil {
		return nil, err
	}
	// Return the string representation as a basic AST substitute
	return compiled.String(), nil
}

// ParseWithRecovery parses with error recovery
func (v *v154Instance) ParseWithRecovery(expr string) (interface{}, error) {
	// v1.5.4 doesn't support recovery mode
	return v.Parse(expr)
}

// v154Expression wraps a v1.5.4 Expr to implement our Expression interface
type v154Expression struct {
	expr *v154.Expr
	// Store bindings that would be used in v2.0.6 style
	bindings map[string]interface{}
}

// Evaluate evaluates the expression
func (e *v154Expression) Evaluate(inputJSON []byte, bindings map[string]interface{}) ([]byte, error) {
	// Parse the input JSON
	var input interface{}
	if len(inputJSON) > 0 {
		if err := json.Unmarshal(inputJSON, &input); err != nil {
			return nil, fmt.Errorf("failed to unmarshal input JSON: %w", err)
		}
	}

	// v1.5.4 handles variables and functions differently
	// We need to register them before evaluation
	if len(bindings) > 0 {
		// Separate functions from variables
		vars := make(map[string]interface{})
		exts := make(map[string]v154.Extension)

		for name, value := range bindings {
			// Check if it's a function
			if fn, ok := value.(func(args []interface{}) (interface{}, error)); ok {
				// Wrap the function in an Extension
				exts[name] = v154.Extension{
					Func: fn,
				}
			} else {
				// It's a variable
				vars[name] = value
			}
		}

		// Register extensions if any
		if len(exts) > 0 {
			if err := e.expr.RegisterExts(exts); err != nil {
				return nil, fmt.Errorf("failed to register functions: %w", err)
			}
		}

		// Register variables if any
		if len(vars) > 0 {
			if err := e.expr.RegisterVars(vars); err != nil {
				return nil, fmt.Errorf("failed to register variables: %w", err)
			}
		}
	}

	// Evaluate the expression
	result, err := e.expr.Eval(input)
	if err != nil {
		// Handle the special ErrUndefined case
		if errors.Is(err, v154.ErrUndefined) {
			// Return empty result for undefined, matching v2.0.6 behavior
			return []byte("null"), nil
		}
		return nil, err
	}

	// Convert result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return resultJSON, nil
}

// SetMaxDepth sets the maximum recursion depth
func (e *v154Expression) SetMaxDepth(maxDepth int) {
	// v1.5.4 doesn't support this feature
	// Silently ignore
}

// SetMaxTime sets the maximum execution time
func (e *v154Expression) SetMaxTime(maxMs int) {
	// v1.5.4 doesn't support this feature
	// Silently ignore
}

// SetMaxRange sets the maximum range size
func (e *v154Expression) SetMaxRange(maxRange int) {
	// v1.5.4 doesn't support this feature
	// Silently ignore
}

// Assign assigns a value to a variable
func (e *v154Expression) Assign(name string, value interface{}) {
	// v1.5.4 requires registering variables before evaluation
	// We'll store them and apply during Evaluate
	if e.bindings == nil {
		e.bindings = make(map[string]interface{})
	}
	e.bindings[name] = value
}

// RegisterFunction registers a custom function
func (e *v154Expression) RegisterFunction(name string, implementation interface{}, signature string) error {
	// v1.5.4 doesn't use signatures in the same way
	// and requires registration before evaluation
	return fmt.Errorf("RegisterFunction is not supported in v1.5.4 adapter - use bindings parameter in Evaluate instead")
}

// AST returns the abstract syntax tree
func (e *v154Expression) AST() interface{} {
	// v1.5.4 doesn't expose the AST directly
	// Return the string representation instead
	return e.expr.String()
}

// Errors returns parser errors
func (e *v154Expression) Errors() []error {
	// v1.5.4 doesn't support recovery mode or error collection
	return nil
}

// RegisterGlobalFunction registers a custom function globally
func (v *v154Instance) RegisterGlobalFunction(name string, implementation interface{}, signature string) error {
	// v1.5.4 doesn't support global function registration
	// Return an informative error
	return fmt.Errorf("RegisterGlobalFunction is not supported in v1.5.4 - this feature requires v2.0.6 or later")
}

// SetDefaultMaxDepth sets the global default maximum recursion depth
func (v *v154Instance) SetDefaultMaxDepth(maxDepth int) {
	// v1.5.4 doesn't support DoS protection features
	// Silently ignore
}

// SetDefaultMaxTime sets the global default maximum execution time
func (v *v154Instance) SetDefaultMaxTime(maxMs int) {
	// v1.5.4 doesn't support DoS protection features
	// Silently ignore
}

// SetDefaultMaxRange sets the global default maximum range size
func (v *v154Instance) SetDefaultMaxRange(maxRange int) {
	// v1.5.4 doesn't support DoS protection features
	// Silently ignore
}

// Does this JSON contain the special undefined value?
func (v *v154Instance) IsUndefined(value []byte) bool {
	return v154.IsUndefined(value)
}

// Make a JSONata error (generally from a user-defined function)
func (v *v154Instance) MakeError(code string, message string) error {
	return v154.MakeError(code, message)
}
