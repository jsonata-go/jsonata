# JSONata Exerciser (Go Version)

A Go implementation of the JSONata Exerciser, providing a web-based interface for testing JSONata expressions with multiple version support.

## Features

- **Multi-version Support**: Switch between different JSONata versions (v1.5.4, v2.0.6)
- **Live Evaluation**: See results update as you type
- **Sample Data**: Pre-loaded sample datasets (Invoice, Address, Schema, Library, Bindings)
- **Split Pane Interface**: Resizable panes for optimal workspace
- **JSON Formatting**: Built-in JSON formatter
- **Bindings Support**: Define custom variables and functions
- **Cross-platform**: Works on Windows, macOS, and Linux

## Installation

```bash
go build -o exerciser .
```

## Usage

Simply run the exerciser:

```bash
./exerciser
```

This will:
1. Start a local web server on port 8080
2. Automatically open your default browser
3. Load the exerciser interface

## Interface Layout

- **Left Top**: JSON input data
- **Left Bottom**: Bindings (collapsible)
- **Right Top**: JSONata expression
- **Right Bottom**: Result

## Keyboard Shortcuts

- No special shortcuts currently implemented

## API Endpoints

The exerciser exposes the following REST API:

- `GET /api/versions` - List available JSONata versions
- `POST /api/evaluate` - Evaluate a JSONata expression
- `GET /api/samples` - Get all sample datasets
- `GET /api/samples?name={name}` - Get a specific sample

## Development

The exerciser is built with:
- Go standard library for the backend
- Vanilla JavaScript for the frontend
- CodeMirror for syntax highlighting
- Split.js for resizable panes

## Differences from JavaScript Version

- Runs locally (no external services)
- Simplified bindings parser (supports basic Math functions)
- No save/share functionality (yet)
- No external libraries support (yet)

## Future Enhancements

- [ ] JSONata syntax highlighting
- [ ] Save/share functionality
- [ ] External library support
- [ ] More comprehensive bindings parser
- [ ] Error position highlighting