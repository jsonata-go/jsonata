# JSONata in Go

Package jsonata is a query and transformation language for JSON.
It's a Go port of the JavaScript library [JSONata](http://jsonata.org/).

It currently has feature parity with jsonata-js 1.5.4. As well as a most of the functions added in newer versions. You can see potentially missing functions by looking at the [jsonata-js changelog](https://github.com/jsonata-js/jsonata/blob/master/CHANGELOG.md).

## Install

    go get github.com/jsonata-go/jsonata

## Version Management

This library provides a unified interface for managing multiple versions of JSONata implementations, similar to how the JavaScript JSONata Exerciser allows users to select different versions.

### Version Management API

#### Core Types

- **`JSONataInstance`**: Interface representing a specific JSONata version
- **`Expression`**: Interface for compiled JSONata expressions

#### Main Functions

- **`AvailableVersions() []string`**: Returns sorted list of available versions
- **`Open(version string) (JSONataInstance, error)`**: Opens a specific version
- **`OpenLatest() (JSONataInstance, error)`**: Opens the latest version
- **`LatestVersion() string`**: Returns the latest version string

### Version Management Usage

```go
// List available versions
versions := jsonata.AvailableVersions()
fmt.Println("Available versions:", versions)

// Open a specific version
instance, err := jsonata.Open("v2.0.6")
if err != nil {
    log.Fatal(err)
}

// Compile an expression
expr, err := instance.Compile("$.name", false)
if err != nil {
    log.Fatal(err)
}

// Evaluate against JSON input
input := []byte(`{"name": "John Doe"}`)
result, err := expr.Evaluate(input, nil)
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(result)) // Output: "John Doe"
```

## Direct Usage (Legacy)

```Go
import (
	"encoding/json"
	"fmt"
	"log"

	jsonata "github.com/jsonata-go/jsonata"
)

const jsonString = `
    {
        "orders": [
            {"price": 10, "quantity": 3},
            {"price": 0.5, "quantity": 10},
            {"price": 100, "quantity": 1}
        ]
    }
`

func main() {

	var data interface{}

	// Decode JSON.
	err := json.Unmarshal([]byte(jsonString), &data)
	if err != nil {
		log.Fatal(err)
	}

	// Create expression.
	e := jsonata.MustCompile("$sum(orders.(price*quantity))")

	// Evaluate.
	res, err := e.Eval(data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(res)
	// Output: 135
}
```

## Contributing

We love issues, fixes, and pull requests from everyone. Please run the
unit-tests, staticcheck, and goimports prior to submitting your PR. By participating in this project, you agree to abide by
the Blues Inc [code of conduct](https://blues.github.io/opensource/code-of-conduct).

For details on contributions we accept and the process for contributing, see our
[contribution guide](CONTRIBUTING.md).

In addition to the Go unit tests there is also a test runner that will run against the jsonata-js test
suite in the [jsonata-test](./jsonata-test) directory. A number of these tests currently fail, but we're working towards feature parity with the jsonata-js reference implementation. Pull requests welcome!

If you would like to contribute to this library a good first issue would be to run the jsonata-test suite,
and fix any of the tests not passing.
