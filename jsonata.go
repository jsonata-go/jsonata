// Package jsonata provides a version management system for multiple JSONata implementations
package jsonata

import (
	"fmt"
	"sort"
	"sync"
)

// JSONataInstance represents a specific version of JSONata with its exported APIs
type JSONataInstance interface {
	// Version returns the version string of this JSONata instance
	Version() string

	// Compile compiles a JSONata expression string into an executable Expression
	Compile(expr string, recoveryMode bool) (Expression, error)

	// Parse parses a JSONata expression into an AST
	Parse(expr string) (interface{}, error)

	// ParseWithRecovery parses with error recovery enabled
	ParseWithRecovery(expr string) (interface{}, error)

	// RegisterGlobalFunction registers a custom function globally for all expressions
	RegisterGlobalFunction(name string, implementation interface{}, signature string) error

	// SetDefaultMaxDepth sets the global default maximum recursion depth for all new expressions
	SetDefaultMaxDepth(maxDepth int)

	// SetDefaultMaxTime sets the global default maximum execution time for all new expressions (in milliseconds)
	SetDefaultMaxTime(maxMs int)

	// SetDefaultMaxRange sets the global default maximum size for range expressions for all new expressions
	SetDefaultMaxRange(maxRange int)

	// Does this JSON contain the special undefined value?
	IsUndefined(value []byte) bool

	// Make a JSONata error (generally from a user-defined function)
	MakeError(code string, message string) error
}

// Expression represents a compiled JSONata expression that can be evaluated
type Expression interface {
	// Evaluate evaluates the expression against JSON input with optional bindings
	Evaluate(inputJSON []byte, bindings map[string]interface{}) ([]byte, error)

	// SetMaxDepth sets the maximum recursion depth
	SetMaxDepth(maxDepth int)

	// SetMaxTime sets the maximum execution time in milliseconds
	SetMaxTime(maxMs int)

	// SetMaxRange sets the maximum range size
	SetMaxRange(maxRange int)

	// Assign assigns a value to a variable in the expression's environment
	Assign(name string, value interface{})

	// RegisterFunction registers a custom function
	RegisterFunction(name string, implementation interface{}, signature string) error

	// AST returns the abstract syntax tree of the expression
	AST() interface{}

	// Errors returns any parser errors (for recovery mode)
	Errors() []error
}

// versionRegistry holds all registered JSONata versions
var (
	versionRegistry = make(map[string]func() JSONataInstance)
	registryMutex   sync.RWMutex
)

// RegisterVersion registers a JSONata version implementation
func RegisterVersion(version string, factory func() JSONataInstance) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	versionRegistry[version] = factory
}

// AvailableVersions returns a sorted list of available JSONata versions
func AvailableVersions() []string {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	versions := make([]string, 0, len(versionRegistry))
	for version := range versionRegistry {
		versions = append(versions, version)
	}

	// Sort versions in reverse order (newest first)
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))
	return versions
}

// Open returns a JSONata instance for the specified version
func Open(version string) (JSONataInstance, error) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	factory, exists := versionRegistry[version]
	if !exists {
		return nil, fmt.Errorf("JSONata version %s not found", version)
	}

	return factory(), nil
}

// LatestVersion returns the latest available version string
func LatestVersion() string {
	versions := AvailableVersions()
	if len(versions) == 0 {
		return ""
	}
	return versions[0]
}

// OpenLatest returns a JSONata instance for the latest available version
func OpenLatest() (JSONataInstance, error) {
	latest := LatestVersion()
	if latest == "" {
		return nil, fmt.Errorf("no JSONata versions available")
	}
	return Open(latest)
}
