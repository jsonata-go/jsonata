package jsonata

import (
	"fmt"

	v206 "github.com/jsonata-go/jsonata/v206"
)

// v206Instance implements JSONataInstance for v2.0.6
type v206Instance struct{}

// Version returns the version string
func (v *v206Instance) Version() string {
	return "v2.0.6"
}

// Compile compiles a JSONata expression
func (v *v206Instance) Compile(expr string, recoveryMode bool) (Expression, error) {
	compiled, err := v206.Compile(expr, recoveryMode)
	if err != nil {
		return nil, err
	}
	return &v206Expression{expr: compiled}, nil
}

// Parse parses a JSONata expression into an AST
func (v *v206Instance) Parse(expr string) (interface{}, error) {
	return v206.Parse(expr)
}

// ParseWithRecovery parses with error recovery
func (v *v206Instance) ParseWithRecovery(expr string) (interface{}, error) {
	return v206.ParseWithRecovery(expr)
}

// v206Expression wraps a v2.0.6 Expression to implement our Expression interface
type v206Expression struct {
	expr *v206.Expression
}

// Evaluate evaluates the expression
func (e *v206Expression) Evaluate(inputJSON []byte, bindings map[string]interface{}) ([]byte, error) {
	return e.expr.Evaluate(inputJSON, bindings)
}

// SetMaxDepth sets the maximum recursion depth
func (e *v206Expression) SetMaxDepth(maxDepth int) {
	e.expr.SetMaxDepth(maxDepth)
}

// SetMaxTime sets the maximum execution time
func (e *v206Expression) SetMaxTime(maxMs int) {
	e.expr.SetMaxTime(maxMs)
}

// SetMaxRange sets the maximum range size
func (e *v206Expression) SetMaxRange(maxRange int) {
	e.expr.SetMaxRange(maxRange)
}

// Assign assigns a value to a variable
func (e *v206Expression) Assign(name string, value interface{}) {
	e.expr.Assign(name, value)
}

// RegisterFunction registers a custom function
func (e *v206Expression) RegisterFunction(name string, implementation interface{}, signature string) error {
	// v2.0.6 expects JSONataFunc type
	fn, ok := implementation.(v206.JSONataFunc)
	if !ok {
		// Try to convert if it's a compatible function type
		if fnGeneric, ok := implementation.(func(args []interface{}) (interface{}, error)); ok {
			fn = v206.JSONataFunc(fnGeneric)
		} else {
			return fmt.Errorf("implementation must be a JSONataFunc")
		}
	}
	return e.expr.RegisterFunction(name, fn, signature)
}

// AST returns the abstract syntax tree
func (e *v206Expression) AST() interface{} {
	return e.expr.AST()
}

// Errors returns parser errors
func (e *v206Expression) Errors() []error {
	return e.expr.Errors()
}

// RegisterGlobalFunction registers a custom function globally
func (v *v206Instance) RegisterGlobalFunction(name string, implementation interface{}, signature string) error {
	// v2.0.6 expects JSONataFunc type
	fn, ok := implementation.(v206.JSONataFunc)
	if !ok {
		// Try to convert if it's a compatible function type
		if fnGeneric, ok := implementation.(func(args []interface{}) (interface{}, error)); ok {
			fn = v206.JSONataFunc(fnGeneric)
		} else {
			return fmt.Errorf("implementation must be a JSONataFunc")
		}
	}
	return v206.RegisterGlobalFunction(name, fn, signature)
}

// SetDefaultMaxDepth sets the global default maximum recursion depth
func (v *v206Instance) SetDefaultMaxDepth(maxDepth int) {
	v206.SetDefaultMaxDepth(maxDepth)
}

// SetDefaultMaxTime sets the global default maximum execution time
func (v *v206Instance) SetDefaultMaxTime(maxMs int) {
	v206.SetDefaultMaxTime(maxMs)
}

// SetDefaultMaxRange sets the global default maximum range size
func (v *v206Instance) SetDefaultMaxRange(maxRange int) {
	v206.SetDefaultMaxRange(maxRange)
}

// Does this JSON contain the special undefined value?
func (v *v206Instance) IsUndefined(value []byte) bool {
	return v206.IsUndefined(value)
}

// Make a JSONata error (generally from a user-defined function)
func (v *v206Instance) MakeError(code string, message string) error {
	return v206.MakeError(code, message)
}
