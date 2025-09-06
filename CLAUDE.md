# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kong Route Tester written in Go that validates routes from Kong declarative configuration files by making HTTP requests to test endpoints. The tool parses YAML configuration files, handles template substitution, and provides detailed testing results.

## Common Commands

### Building and Running
```bash
# Build the binary
go build -o kong-route-tester main.go

# Run with default settings (tests kong.yaml against https://api.dev.community.com)
./kong-route-tester

# Run with custom configuration file and base URL
./kong-route-tester -file=custom-kong.yaml -url=https://api.staging.example.com

# Test only authenticated routes with token
./kong-route-tester -test-unauth=false -token=your-auth-token

# Dry run to see what would be tested
./kong-route-tester -dry-run -verbose

# Limit requests and enable verbose output
./kong-route-tester -max=50 -verbose
```

### Test Server for Local Development
```bash
# Build and run the test server (separate terminal)
go build -o test-server test-server.go
./test-server --port=8080 --verbose

# Run test server with authentication enabled
./test-server --port=8080 --require-auth --auth-token=test-token-123

# Run test server with error simulation
./test-server --port=8080 --simulate-errors --verbose

# Test against local server
./kong-route-tester --url=http://127.0.0.1:8080 --token=test-token-123
```

### Dependencies
```bash
# Download dependencies
go mod tidy

# Verify dependencies
go mod verify
```

## Architecture

### Core Components

- **Kong Configuration Parser** (`readKongConfig`): Reads YAML files and handles environment variable templating using `${VAR:?}` and `${VAR:=default}` patterns
- **Route Testing Engine** (`testRoutes`): Iterates through services and routes, applying filters and making HTTP requests
- **Authentication Detection** (`hasAuthPlugin`): Identifies routes requiring authentication by checking for "auth" plugins at route or service level
- **Regex Path Expansion** (`expandRegexPath`): Converts Kong regex patterns to testable example paths using predefined replacements
- **Result Reporting** (`printSummary`): Provides detailed statistics and identifies problematic routes

### Key Data Structures

- `KongConfig`: Root configuration containing services
- `Service`: Kong service with URL, plugins, and routes
- `Route`: Kong route with paths, methods, hosts, and plugins
- `TestResult`: Test outcome with service, route, path, method, status code, and error information

### Configuration Flags

The tool supports extensive configuration through command-line flags:
- `-file`: Kong configuration file path (default: "kong.yaml")
- `-url`: Base URL for testing (default: "https://api.dev.community.com")
- `-token`: Authentication token for protected routes
- `-test-auth`/`-test-unauth`: Control which route types to test
- `-verbose`: Enable detailed output
- `-dry-run`: Show test plan without making requests
- `-max`: Limit number of requests

### Template Handling

The tool performs basic environment variable substitution in YAML files and provides fallback values for testing (e.g., SERVICE_ADDRESS variables default to "http://services.sms.community:10000").

## Test Server

The repository includes `test-server.go`, a mock API server for local testing that:

- **Simulates Kong Route Behavior**: Handles all routes defined in kong.yaml with appropriate responses
- **Authentication Testing**: Supports Bearer token authentication (configurable via `--require-auth` flag)
- **Error Simulation**: Can simulate various HTTP error codes (403, 404, 405, 429, 500) via `--simulate-errors` flag
- **Flexible Responses**: Returns JSON responses with request metadata for debugging
- **Pattern Matching**: Extracts path parameters from URLs to test regex pattern expansion
- **Method Validation**: Enforces HTTP method restrictions per route
- **CORS Support**: Handles OPTIONS preflight requests appropriately

The test server supports the same pflag CLI interface as the main application for consistency.