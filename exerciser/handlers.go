package main

import (
	"encoding/json"
	"fmt"
	"net/http"

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
	if req.Bindings != "" {
		if err := json.Unmarshal([]byte(req.Bindings), &bindings); err != nil {
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
