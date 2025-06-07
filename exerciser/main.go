package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

// Sample represents a sample dataset with expression and bindings
type Sample struct {
	Name     string          `json:"name"`
	JSON     json.RawMessage `json:"json"`
	JSONata  string          `json:"jsonata"`
	Bindings string          `json:"bindings"`
}

var samples = []Sample{
	{
		Name:     "Event",
		JSONata:  "body",
		Bindings: defaultBindingsText,
	},
	{
		Name:     "Route",
		JSONata:  "body",
		Bindings: defaultBindingsText,
	},
	{
		Name:     "Invoice",
		JSONata:  "$sum(Account.Order.Product.(Price * Quantity))",
		Bindings: defaultBindingsText,
	},
	{
		Name: "Address",
		JSONata: `{
  "name": FirstName & " " & Surname,
  "mobile": Phone[type = "mobile"].number
}`,
		Bindings: defaultBindingsText,
	},
	{
		Name:     "Schema",
		JSONata:  "**.properties ~> $keys()",
		Bindings: defaultBindingsText,
	},
	{
		Name: "Library",
		JSONata: `library.loans@$L.books@$B[$L.isbn=$B.isbn].customers[$L.customer=id].{
  'customer': name,
  'book': $B.title,
  'due': $L.return
}`,
		Bindings: defaultBindingsText,
	},
	{
		Name:    "Bindings",
		JSONata: "$cosine(angle * $pi/180)\n\n/*\nJSONata can be extended by binding variables to external functions and values.\nExpand the 'Bindings' panel to bind variables to Javascript code for use in your expression.\nThis is useful for experimenting with functions that are not built into JSONata.\n*/",
		Bindings: `{
  pi: 3.1415926535898,
  cosine: Math.cos
}`,
	},
}

const defaultBindingsText = `// Define bindings as properties of the below object
// Its possible to write any javascript expression here that evaluates to an object
{
 // name: value 
}`

func main() {
	// Initialize sample data
	initializeSamples()

	// Set up HTTP routes
	mux := http.NewServeMux()

	// Serve static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// API endpoints
	mux.HandleFunc("/api/versions", handleVersions)
	mux.HandleFunc("/api/evaluate", handleEvaluate)
	mux.HandleFunc("/api/samples", handleSamples)

	// Start server
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Starting JSONata Exerciser on http://localhost:%s", port)

	// Open browser after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		openBrowser("http://localhost:" + port)
	}()

	// Start server
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// openBrowser opens the URL in the default browser
func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)

	if err := exec.Command(cmd, args...).Start(); err != nil {
		log.Printf("Failed to open browser: %v", err)
		log.Printf("Please open %s in your browser", url)
	}
}

// initializeSamples loads the JSON data for each sample
func initializeSamples() {
	// Event sample data
	samples[0].JSON = json.RawMessage(`{
  "event": "44a8d355-b288-8084-9260-e7995d2d88a8",
  "when": 1749292879,
  "file": "_air.qo",
  "body": {
    "aqi": 164,
    "aqi_algorithm": "cf-atm",
    "aqi_level": "unhealthy",
    "c00_30": 31241,
    "c00_50": 7473,
    "c01_00": 45,
    "c02_50": 2,
    "c05_00": 1,
    "csamples": 48,
    "csecs": 60,
    "humidity": 100,
    "pm01_0": 78.1875,
    "pm01_0_rstd": 0.111694336,
    "pm02_5": 79.770836,
    "pm02_5_rstd": 0.107421875,
    "pm10_0": 81.916664,
    "pm10_0_rstd": 0.076049805,
    "pressure": 101476.09,
    "sensor": "pms7003",
    "temperature": 11.851773,
    "voltage": 4.25
  },
  "session": "8bf9c487-46c2-4364-80bb-7354a43da00f",
  "transport": "cell:emtc",
  "best_id": "MyAirnoteSouth",
  "device": "dev:864475044206720",
  "sn": "MyAirnoteSouth",
  "product": "product:org.airnote.solar.air.v1",
  "app": "app:2606f411-dea6-44a0-9743-1130f57d77d8",
  "received": 1749293763.539238,
  "req": "note.add",
  "batch_received": 1749293763.538996,
  "batch_number": 1,
  "batch_total": 1,
  "best_location_type": "gps",
  "best_lat": 48.0176675,
  "best_lon": -122.58795703125,
  "best_location": "Freeland WA",
  "best_country": "US",
  "best_timezone": "America/Los_Angeles",
  "where_olc": "84WV2C96+3R7M",
  "where_lat": 48.0176675,
  "where_lon": -122.58795703125,
  "where_location": "Freeland WA",
  "where_country": "US",
  "where_timezone": "America/Los_Angeles",
  "tower_when": 1749293763,
  "tower_lat": 47.937854,
  "tower_lon": -122.716769,
  "tower_country": "US",
  "tower_location": "Port Ludlow Washington",
  "tower_timezone": "America/Los_Angeles",
  "tower_id": "310,410,37126,107725841",
  "status": "success",
  "fleets": [
    "fleet:f9045782-6ad1-483c-a4d2-ab859f4adc04"
  ]
}`)

	// Route sample data
	samples[1].JSON = json.RawMessage(`{}`)

	// Invoice sample data
	samples[2].JSON = json.RawMessage(`{
  "Account": {
    "Account Name": "Firefly",
    "Order": [
      {
        "OrderID": "order103",
        "Product": [
          {
            "Product Name": "Bowler Hat",
            "ProductID": 858383,
            "SKU": "0406654608",
            "Description": {
              "Colour": "Purple",
              "Width": 300,
              "Height": 200,
              "Depth": 210,
              "Weight": 0.75
            },
            "Price": 34.45,
            "Quantity": 2
          },
          {
            "Product Name": "Trilby hat",
            "ProductID": 858236,
            "SKU": "0406634348",
            "Description": {
              "Colour": "Orange",
              "Width": 300,
              "Height": 200,
              "Depth": 210,
              "Weight": 0.6
            },
            "Price": 21.67,
            "Quantity": 1
          }
        ]
      },
      {
        "OrderID": "order104",
        "Product": [
          {
            "Product Name": "Bowler Hat",
            "ProductID": 858383,
            "SKU": "040657863",
            "Description": {
              "Colour": "Purple",
              "Width": 300,
              "Height": 200,
              "Depth": 210,
              "Weight": 0.75
            },
            "Price": 34.45,
            "Quantity": 4
          },
          {
            "ProductID": 345664,
            "SKU": "0406654603",
            "Product Name": "Cloak",
            "Description": {
              "Colour": "Black",
              "Width": 30,
              "Height": 20,
              "Depth": 210,
              "Weight": 2.0
            },
            "Price": 107.99,
            "Quantity": 1
          }
        ]
      }
    ]
  }
}`)

	// Address sample data
	samples[3].JSON = json.RawMessage(`{
  "FirstName": "Fred",
  "Surname": "Smith",
  "Age": 28,
  "Address": {
    "Street": "Hursley Park",
    "City": "Winchester",
    "Postcode": "SO21 2JN"
  },
  "Phone": [
    {
      "type": "home",
      "number": "0203 544 1234"
    },
    {
      "type": "office",
      "number": "01962 001234"
    },
    {
      "type": "office",
      "number": "01962 001235"
    },
    {
      "type": "mobile",
      "number": "077 7700 1234"
    }
  ],
  "Email": [
    {
      "type": "office",
      "address": ["fred.smith@my-work.com", "fsmith@my-work.com"]
    },
    {
      "type": "home",
      "address": ["freddy@my-social.com", "frederic.smith@very-serious.com"]
    }
  ],
  "Other": {
    "Over 18 ?": true,
    "Misc": null,
    "Alternative.Address": {
      "Street": "Brick Lane",
      "City": "London",
      "Postcode": "E1 6RF"
    }
  }
}`)

	// Schema sample data (truncated for brevity)
	samples[4].JSON = json.RawMessage(`{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "required": ["Account"],
  "type": "object",
  "id": "file://input-schema.json",
  "properties": {
    "Account": {
      "required": ["Order"],
      "type": "object",
      "properties": {
        "Order": {
          "items": {
            "required": ["OrderID", "Product"],
            "type": "object",
            "properties": {
              "OrderID": {"type": "string"},
              "Product": {"type": "array"}
            }
          },
          "type": "array"
        }
      }
    }
  }
}`)

	// Library sample data
	samples[5].JSON = json.RawMessage(`{
  "library": {
    "books": [
      {
        "title": "Structure and Interpretation of Computer Programs",
        "authors": ["Abelson", "Sussman"],
        "isbn": "9780262510875",
        "price": 38.9,
        "copies": 2
      },
      {
        "title": "The C Programming Language",
        "authors": ["Kernighan", "Richie"],
        "isbn": "9780131103627",
        "price": 33.59,
        "copies": 3
      },
      {
        "title": "The AWK Programming Language",
        "authors": ["Aho", "Kernighan", "Weinberger"],
        "isbn": "9780201079814",
        "copies": 1
      },
      {
        "title": "Compilers: Principles, Techniques, and Tools",
        "authors": ["Aho", "Lam", "Sethi", "Ullman"],
        "isbn": "9780201100884",
        "price": 23.38,
        "copies": 1
      }
    ],
    "loans": [
      {
        "customer": "10001",
        "isbn": "9780262510875",
        "return": "2016-12-05"
      },
      {
        "customer": "10003",
        "isbn": "9780201100884",
        "return": "2016-10-22"
      },
      {
        "customer": "10003",
        "isbn": "9780262510875",
        "return": "2016-12-22"
      }
    ],
    "customers": [
      {
        "id": "10001",
        "name": "Joe Doe",
        "address": {
          "street": "2 Long Road",
          "city": "Winchester",
          "postcode": "SO22 5PU"
        }
      },
      {
        "id": "10002",
        "name": "Fred Bloggs",
        "address": {
          "street": "56 Letsby Avenue",
          "city": "Winchester",
          "postcode": "SO22 4WD"
        }
      },
      {
        "id": "10003",
        "name": "Jason Arthur",
        "address": {
          "street": "1 Preddy Gate",
          "city": "Southampton",
          "postcode": "SO14 0MG"
        }
      }
    ]
  }
}`)

	// Bindings sample data
	samples[6].JSON = json.RawMessage(`{"angle": 60}`)
}
