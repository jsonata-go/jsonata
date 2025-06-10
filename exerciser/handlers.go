package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/jsonata-go/jsonata"
)

// EvaluateRequest represents the request body for evaluation
type EvaluateRequest struct {
	Expression string          `json:"expression"`
	Input      json.RawMessage `json:"input"`
	Bindings   string          `json:"bindings"`
	Version    string          `json:"version"`
}

// EvaluateResponse represents the response for evaluation
type EvaluateResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// handleVersions returns available JSONata versions
func handleVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	versions := jsonata.AvailableVersions()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"versions": versions,
	})
}

// handleEvaluate evaluates a JSONata expression
func handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Default to latest version if not specified
	version := req.Version
	if version == "" {
		version = jsonata.LatestVersion()
	}

	// Open the requested version
	instance, err := jsonata.Open(version)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(EvaluateResponse{
			Error: fmt.Sprintf("failed to open JSONata version %s: %v", version, err),
		})
		return
	}

	// Compile the expression
	expr, err := instance.Compile(req.Expression, false)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(EvaluateResponse{
			Error: fmt.Sprintf("compilation error: %v", err),
		})
		return
	}

	// Initialize some test expressions
	expr.RegisterFunction("doNotRoute", func(args []any) (any, error) { return "", instance.MakeError("", "{do-not-route}") }, "<:x>")
	expr.RegisterFunction("addToFleet", func(args []any) (any, error) { return "", instance.MakeError("", "{add-to-fleet}") }, "<:x>")
	expr.RegisterFunction("removeFromFleet", func(args []any) (any, error) { return "", instance.MakeError("", "{remove-from-fleet}") }, "<:x>")
	expr.RegisterFunction("leaveFleetAlone", func(args []any) (any, error) { return "", instance.MakeError("", "{leave-fleet-alone}") }, "<:x>")

	// ($loop := function($n) { $loop($n + 1) }; $loop(1))
	expr.SetMaxTime(1000) // Maximum execution time in milliseconds
	// ($factorial := function($n) { $n <= 1 ? 1 : $n * $factorial($n - 1) }; $factorial(100000))
	expr.SetMaxDepth(100) // Maximum recursion depth
	// [0..999999999].$string($)
	expr.SetMaxRange(100) // Maximum entries for range expressions

	// Parse bindings if provided
	bindings := make(map[string]interface{})
	if req.Bindings != "" && req.Bindings != defaultBindingsText {
		// Try to evaluate the bindings as a JavaScript-like object
		// For now, we'll support a simple subset
		if err := parseBindings(req.Bindings, bindings); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(EvaluateResponse{
				Error: fmt.Sprintf("bindings error: %v", err),
			})
			return
		}
	}

	// Evaluate the expression
	result, err := expr.Evaluate(req.Input, bindings)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(EvaluateResponse{
			Error: fmt.Sprintf("evaluation error: %v", err),
		})
		return
	}

	// Return the result
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EvaluateResponse{
		Result: json.RawMessage(result),
	})
}

// parseBindings parses a JavaScript-like bindings object
// This is a simplified parser that handles basic cases
func parseBindings(bindingsStr string, bindings map[string]interface{}) error {
	// Remove comments
	lines := strings.Split(bindingsStr, "\n")
	var cleanedLines []string
	for _, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		cleanedLines = append(cleanedLines, line)
	}
	cleaned := strings.Join(cleanedLines, "\n")

	// Check if it looks like a simple object
	cleaned = strings.TrimSpace(cleaned)
	if !strings.HasPrefix(cleaned, "{") || !strings.HasSuffix(cleaned, "}") {
		return fmt.Errorf("bindings must be an object")
	}

	// Extract the content
	content := cleaned[1 : len(cleaned)-1]

	// Parse simple key-value pairs
	// This is a very basic parser that handles:
	// - pi: 3.14159...
	// - cosine: Math.cos (converts to a Go function)
	pairs := strings.Split(content, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Handle different value types
		switch {
		case value == "Math.cos":
			// Map Math.cos to Go's math.Cos
			bindings[key] = func(args []interface{}) (interface{}, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("cos expects 1 argument")
				}
				switch v := args[0].(type) {
				case float64:
					return math.Cos(v), nil
				case int:
					return math.Cos(float64(v)), nil
				default:
					return nil, fmt.Errorf("cos expects a number")
				}
			}
		case value == "Math.sin":
			// Map Math.sin to Go's math.Sin
			bindings[key] = func(args []interface{}) (interface{}, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("sin expects 1 argument")
				}
				switch v := args[0].(type) {
				case float64:
					return math.Sin(v), nil
				case int:
					return math.Sin(float64(v)), nil
				default:
					return nil, fmt.Errorf("sin expects a number")
				}
			}
		default:
			// Try to parse as JSON value
			var val interface{}
			if err := json.Unmarshal([]byte(value), &val); err == nil {
				bindings[key] = val
			}
		}
	}

	return nil
}

// handleSamples returns the list of sample data
func handleSamples(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if a specific sample is requested
	sampleName := r.URL.Query().Get("name")
	if sampleName != "" {
		for _, sample := range samples {
			if sample.Name == sampleName {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(sample)
				return
			}
		}
		http.Error(w, "Sample not found", http.StatusNotFound)
		return
	}

	// Return all samples
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(samples)
}
