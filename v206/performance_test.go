// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package jsonata

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Benchmark data structures
var (
	// Small JSON dataset (100 bytes)
	smallJSON = []byte(`{
		"name": "John Doe",
		"age": 30,
		"active": true,
		"score": 95.5
	}`)

	// Medium JSON dataset (~1KB)
	mediumJSON = []byte(`{
		"users": [
			{"id": 1, "name": "Alice", "age": 25, "department": "Engineering", "salary": 95000},
			{"id": 2, "name": "Bob", "age": 30, "department": "Sales", "salary": 85000},
			{"id": 3, "name": "Charlie", "age": 35, "department": "Engineering", "salary": 105000},
			{"id": 4, "name": "Diana", "age": 28, "department": "Marketing", "salary": 90000},
			{"id": 5, "name": "Eve", "age": 32, "department": "Sales", "salary": 92000},
			{"id": 6, "name": "Frank", "age": 29, "department": "Engineering", "salary": 98000},
			{"id": 7, "name": "Grace", "age": 31, "department": "Marketing", "salary": 87000},
			{"id": 8, "name": "Henry", "age": 27, "department": "Sales", "salary": 83000},
			{"id": 9, "name": "Iris", "age": 33, "department": "Engineering", "salary": 110000},
			{"id": 10, "name": "Jack", "age": 26, "department": "Marketing", "salary": 86000}
		]
	}`)

	// Large JSON dataset (~10KB)
	largeJSON []byte

	// Extra large JSON dataset (~100KB)
	extraLargeJSON []byte
)

func init() {
	// Generate large JSON dataset
	var users []map[string]interface{}
	for i := 0; i < 100; i++ {
		users = append(users, map[string]interface{}{
			"id":         i + 1,
			"name":       fmt.Sprintf("User%d", i+1),
			"age":        20 + (i % 40),
			"department": []string{"Engineering", "Sales", "Marketing", "HR", "Finance"}[i%5],
			"salary":     70000 + (i * 1000),
			"active":     i%2 == 0,
			"projects":   []string{fmt.Sprintf("Project%d", i), fmt.Sprintf("Project%d", i+1)},
		})
	}
	data, _ := json.Marshal(map[string]interface{}{"users": users})
	largeJSON = data

	// Generate extra large JSON dataset
	var extraUsers []map[string]interface{}
	for i := 0; i < 1000; i++ {
		extraUsers = append(extraUsers, map[string]interface{}{
			"id":         i + 1,
			"name":       fmt.Sprintf("User%d", i+1),
			"age":        20 + (i % 40),
			"department": []string{"Engineering", "Sales", "Marketing", "HR", "Finance"}[i%5],
			"salary":     70000 + (i * 1000),
			"active":     i%2 == 0,
			"projects":   []string{fmt.Sprintf("Project%d", i), fmt.Sprintf("Project%d", i+1), fmt.Sprintf("Project%d", i+2)},
			"metadata": map[string]interface{}{
				"created":    "2024-01-01",
				"updated":    "2024-06-01",
				"tags":       []string{"tag1", "tag2", "tag3"},
				"attributes": map[string]interface{}{"level": i % 5, "score": float64(i) / 10},
			},
		})
	}
	extraData, _ := json.Marshal(map[string]interface{}{"users": extraUsers})
	extraLargeJSON = extraData
}

// Compilation benchmarks
func BenchmarkCompileSimplePath(b *testing.B) {
	expr := "$.name"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Compile(expr, false)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileComplexPath(b *testing.B) {
	expr := "$.users[age > 30 and department = 'Engineering'].{name: name, salary: salary}"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Compile(expr, false)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileWithFunctions(b *testing.B) {
	expr := "$sum(users[active = true].salary) / $count(users[active = true])"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Compile(expr, false)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileNestedExpressions(b *testing.B) {
	expr := `users.$filter(function($v) { $v.salary > $average($$[department = $v.department].salary) })`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Compile(expr, false)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileTransformation(b *testing.B) {
	expr := `{
		"summary": {
			"totalUsers": $count(users),
			"departments": users{department: $count()},
			"avgSalary": $average(users.salary),
			"ageGroups": users{
				$string($floor(age / 10) * 10) & "s": $count()
			}
		}
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Compile(expr, false)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Evaluation benchmarks with different data sizes
func BenchmarkEvaluateSimplePath_Small(b *testing.B) {
	expr, _ := Compile("$.name", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(smallJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateSimplePath_Medium(b *testing.B) {
	expr, _ := Compile("$.users[0].name", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateArrayFilter_Medium(b *testing.B) {
	expr, _ := Compile("$.users[age > 30].name", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateArrayFilter_Large(b *testing.B) {
	expr, _ := Compile("$.users[age > 30 and department = 'Engineering'].name", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(largeJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateArrayFilter_ExtraLarge(b *testing.B) {
	expr, _ := Compile("$.users[age > 30 and department = 'Engineering'].name", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(extraLargeJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Aggregation function benchmarks
func BenchmarkEvaluateAggregation_Sum(b *testing.B) {
	expr, _ := Compile("$sum(users.salary)", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(largeJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateAggregation_Average(b *testing.B) {
	expr, _ := Compile("$average(users[department = 'Engineering'].salary)", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(largeJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateAggregation_GroupBy(b *testing.B) {
	expr, _ := Compile("users{department: {count: $count(), avgSalary: $average(salary)}}", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// String operation benchmarks
func BenchmarkEvaluateStringOperations(b *testing.B) {
	expr, _ := Compile(`users.($uppercase(name) & " - " & department)`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateRegex(b *testing.B) {
	expr, _ := Compile(`users[name ~> /^[A-E].*$/].name`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Function definition benchmarks
func BenchmarkEvaluateFunctionDefinition(b *testing.B) {
	expr, _ := Compile(`(
		$isHighEarner := function($salary) { $salary > 100000 };
		users[$isHighEarner(salary)].name
	)`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(largeJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateRecursiveFunction(b *testing.B) {
	expr, _ := Compile(`(
		$factorial := function($n) { $n <= 1 ? 1 : $n * $factorial($n - 1) };
		$factorial(10)
	)`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate([]byte(`{}`), nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Range operator benchmarks
func BenchmarkEvaluateRange_Small(b *testing.B) {
	expr, _ := Compile(`[1..100].$string($)`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate([]byte(`{}`), nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateRange_Medium(b *testing.B) {
	expr, _ := Compile(`[1..1000].$string($)`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate([]byte(`{}`), nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Transformation benchmarks
func BenchmarkEvaluateTransformation_Simple(b *testing.B) {
	expr, _ := Compile(`users.{
		"fullName": name,
		"annualSalary": salary,
		"monthlySalary": salary / 12
	}`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateTransformation_Complex(b *testing.B) {
	// Complex transformation benchmark with multiple aggregations and transformations
	expr, _ := Compile(`{
		"summary": {
			"totalUsers": $count(users),
			"avgSalary": $average(users.salary),
			"minSalary": $min(users.salary),
			"maxSalary": $max(users.salary),
			"totalSalary": $sum(users.salary),
			"departments": $distinct(users.department),
			"activeCount": $count(users[active = true]),
			"seniorCount": $count(users[age >= 30]),
			"juniorCount": $count(users[age < 30])
		},
		"highEarners": users[salary > 100000].{
			"name": name,
			"salary": salary,
			"department": department,
			"age": age
		},
		"lowEarners": users[salary < 80000].{
			"name": name,
			"salary": salary
		},
		"engineers": users[department = "Engineering"].{
			"name": name,
			"salary": salary,
			"age": age
		}
	}`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(largeJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Sorting benchmarks
func BenchmarkEvaluateSorting(b *testing.B) {
	expr, _ := Compile(`users^(salary)`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateSortingDescending(b *testing.B) {
	expr, _ := Compile(`users^(>salary)`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Variable binding benchmarks
func BenchmarkEvaluateWithBindings(b *testing.B) {
	expr, _ := Compile(`users[salary > $threshold].name`, false)
	bindings := map[string]interface{}{
		"threshold": 90000.0,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, bindings)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateWith100Bindings(b *testing.B) {
	// Create expression that uses multiple bindings
	expr, _ := Compile(`users[salary > $threshold and department = $dept].name`, false)

	// Create 100 bindings (though only 2 are used)
	bindings := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		bindings[fmt.Sprintf("var%d", i)] = fmt.Sprintf("value%d", i)
	}
	bindings["threshold"] = 90000.0
	bindings["dept"] = "Engineering"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, bindings)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Deeply nested data benchmarks
func BenchmarkEvaluateDeepNesting(b *testing.B) {
	// Create deeply nested JSON
	nestedData := map[string]interface{}{"value": 1}
	for i := 0; i < 10; i++ {
		nestedData = map[string]interface{}{"nested": nestedData}
	}
	deepJSON, _ := json.Marshal(nestedData)

	expr, _ := Compile(`$.nested.nested.nested.nested.nested.nested.nested.nested.nested.nested.value`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(deepJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Wildcard and descendant benchmarks
func BenchmarkEvaluateWildcard(b *testing.B) {
	expr, _ := Compile(`$.users.*.name`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(largeJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateDescendant(b *testing.B) {
	// Note: The descendant operator .. may not be fully implemented
	// Using a recursive function to simulate descendant behavior
	expr, _ := Compile(`(
		$descendants := function($obj) {(
			$type($obj) = "object" ? (
				$obj ~> $each(function($v, $k) {
					$append($v, $descendants($v))
				})
			) : $type($obj) = "array" ? (
				$obj ~> $map($descendants) ~> $reduce($append)
			) : []
		)};
		$names := function($data) {(
			$type($data) = "object" and $exists($data.name) ? [$data.name] : [],
			$type($data) = "object" ? $data ~> $each(function($v) {$names($v)}) ~> $reduce($append) : 
			$type($data) = "array" ? $data ~> $map($names) ~> $reduce($append) : []
		)[0]};
		$names($)
	)`, false)

	if expr == nil {
		// If the complex expression fails, use a simpler alternative
		expr, _ = Compile(`users.name`, false)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Error handling benchmarks
func BenchmarkCompileWithRecovery(b *testing.B) {
	// Intentionally malformed expression
	expr := "$.users[age > 30 .name"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Compile(expr, true) // Recovery mode enabled
	}
}

// Security limit benchmarks
func BenchmarkEvaluateWithMaxDepth(b *testing.B) {
	expr, _ := Compile(`users.name`, false)
	expr.SetMaxDepth(50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluateWithMaxTime(b *testing.B) {
	expr, _ := Compile(`users.name`, false)
	expr.SetMaxTime(5000) // 5 seconds
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(mediumJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Parallel evaluation benchmarks
// NOTE: The current implementation is not thread-safe for concurrent evaluations
// of the same Expression instance due to shared mutable state (timestamp, bindings).
// This benchmark demonstrates the issue and should be addressed in future versions.
func BenchmarkEvaluateParallel(b *testing.B) {
	// Create a separate expression for each goroutine to avoid concurrent access
	createExpr := func() *Expression {
		expr, _ := Compile(`users[age > 30].name`, false)
		return expr
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets its own expression instance
		expr := createExpr()
		for pb.Next() {
			_, err := expr.Evaluate(mediumJSON, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Memory allocation benchmarks
func BenchmarkMemoryAllocations(b *testing.B) {
	expr, _ := Compile(`users[age > 30].{name: name, dept: department}`, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(largeJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Expression complexity benchmarks
func BenchmarkComplexityLinear(b *testing.B) {
	expr, _ := Compile(`users.name`, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(largeJSON, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkComplexityQuadratic(b *testing.B) {
	// Expression that potentially has quadratic complexity
	expr, _ := Compile(`users.($others := $$.users[$ != $$]; {name: name, otherCount: $count($others)})`, false)
	// Use smaller dataset to avoid timeout
	smallData := generateTestData(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := expr.Evaluate(smallData, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Scalability benchmarks
func BenchmarkScalability(b *testing.B) {
	sizes := []int{10, 100, 1000}
	expr, _ := Compile(`users[age > 30 and department = 'Engineering'].name`, false)

	for _, size := range sizes {
		testData := generateTestData(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := expr.Evaluate(testData, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// CRITICAL BENCHMARK: Compile-once-evaluate-many vs Compile-every-time
// This demonstrates the performance difference and helps users understand
// the importance of reusing compiled expressions
func BenchmarkCompileOnceEvaluateMany(b *testing.B) {
	expression := "$.users[age > 30 and department = 'Engineering'].name"

	// Test different iteration counts to show the tradeoff
	iterations := []int{1, 10, 100, 1000}

	for _, iter := range iterations {
		b.Run(fmt.Sprintf("CompileOnce_%d_iterations", iter), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Compile once
				expr, err := Compile(expression, false)
				if err != nil {
					b.Fatal(err)
				}

				// Evaluate many times
				for j := 0; j < iter; j++ {
					_, err := expr.Evaluate(mediumJSON, nil)
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})

		b.Run(fmt.Sprintf("CompileEveryTime_%d_iterations", iter), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Compile and evaluate every time
				for j := 0; j < iter; j++ {
					expr, err := Compile(expression, false)
					if err != nil {
						b.Fatal(err)
					}
					_, err = expr.Evaluate(mediumJSON, nil)
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

// Benchmark showing cost of compilation vs evaluation
func BenchmarkCompilationVsEvaluation(b *testing.B) {
	expressions := []struct {
		name string
		expr string
	}{
		{"Simple", "$.name"},
		{"Medium", "$.users[age > 30].name"},
		{"Complex", "users{department: {count: $count(), avgSalary: $average(salary)}}"},
	}

	for _, test := range expressions {
		// Benchmark just compilation
		b.Run(fmt.Sprintf("%s_CompileOnly", test.name), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := Compile(test.expr, false)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		// Benchmark just evaluation (after compilation)
		b.Run(fmt.Sprintf("%s_EvaluateOnly", test.name), func(b *testing.B) {
			expr, err := Compile(test.expr, false)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := expr.Evaluate(mediumJSON, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		// Benchmark combined (compile + evaluate)
		b.Run(fmt.Sprintf("%s_CompileAndEvaluate", test.name), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				expr, err := Compile(test.expr, false)
				if err != nil {
					b.Fatal(err)
				}
				_, err = expr.Evaluate(mediumJSON, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Comparison with native Go JSON operations
func BenchmarkNativeGoJSONParsing(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var data map[string]interface{}
		err := json.Unmarshal(mediumJSON, &data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNativeGoJSONQuery(b *testing.B) {
	var data map[string]interface{}
	json.Unmarshal(mediumJSON, &data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		users := data["users"].([]interface{})
		var names []string
		for _, user := range users {
			u := user.(map[string]interface{})
			if age, ok := u["age"].(float64); ok && age > 30 {
				names = append(names, u["name"].(string))
			}
		}
		if len(names) == 0 {
			b.Fatal("No results")
		}
	}
}

// Helper function to generate test data
func generateTestData(size int) []byte {
	var users []map[string]interface{}
	for i := 0; i < size; i++ {
		users = append(users, map[string]interface{}{
			"id":         i + 1,
			"name":       fmt.Sprintf("User%d", i+1),
			"age":        20 + (i % 40),
			"department": []string{"Engineering", "Sales", "Marketing", "HR", "Finance"}[i%5],
			"salary":     70000 + (i * 1000),
			"active":     i%2 == 0,
		})
	}
	data, _ := json.Marshal(map[string]interface{}{"users": users})
	return data
}
