# Kong Route Tester

A comprehensive Go tool that validates routes from Kong declarative configuration files by making HTTP requests to test endpoints. The tool parses YAML configuration files, handles template substitution, and provides detailed testing results with authentication detection and error analysis.

## Features

- **Route Discovery**: Automatically discovers all routes from Kong YAML configuration
- **Authentication Detection**: Identifies authenticated vs unauthenticated routes
- **HTTP Method Testing**: Tests all specified HTTP methods (GET, POST, PUT, DELETE, PATCH, etc.)
- **Regex Pattern Support**: Expands Kong regex patterns to testable example paths
- **Detailed Reporting**: Comprehensive summary with success rates and error categorization
- **Template Processing**: Handles [sigil](https://github.com/gliderlabs/sigil) template variables with fallbacks
- **Flexible Configuration**: Extensive CLI options for customized testing scenarios
- **Mock Server Included**: Built-in test server for local development and testing

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/danpilch/kong-route-tester
cd kong-route-tester

# Install dependencies
go mod tidy

# Build the tools
go build -o kong-route-tester main.go
go build -o test-server test-server.go
```

### Basic Usage

```bash
# Using Make (recommended)
make demo                    # Quick demo against local test server
make demo-auth               # Demo with authentication testing
make dev                     # Full development workflow (lint + test)

# Direct usage
./kong-route-tester --file=kong.yaml --url=https://api.example.com
./kong-route-tester --token=your-bearer-token
./kong-route-tester --dry-run --verbose
```

## Configuration

### Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `--file` | `kong.yaml` | Path to Kong configuration file |
| `--url` | `https://api.dev.community.com` | Base URL for testing |
| `--token` | `""` | Bearer token for authenticated routes |
| `--test-auth` | `true` | Test authenticated routes |
| `--test-unauth` | `true` | Test unauthenticated routes |
| `--verbose` | `false` | Enable verbose output |
| `--dry-run` | `false` | Show test plan without making requests |
| `--max` | `0` | Maximum number of requests (0 = unlimited) |

### Example Kong Configuration

The tool supports standard Kong declarative configuration with sigil templating:

```yaml
_format_version: "1.1"

services:
  - name: api-service
    url: ${API_SERVICE_ADDRESS:=http://127.0.0.1:8001}
    routes:
      # Authenticated route
      - name: protected-endpoint
        paths:
          - /api/v1/users/(?<user_id>[0-9a-fA-F-]+)/profile
        methods: ["GET", "PUT"]
        plugins:
          - name: auth

      # Public route
      - name: public-endpoint  
        paths:
          - /api/v1/public/health
        methods: ["GET"]
```

## Local Development with Test Server

The included test server provides a realistic testing environment:

### Start Test Server

```bash
# Basic server (no auth required)
./test-server --port=8080 --verbose

# Server with authentication
./test-server --port=8080 --require-auth --auth-token=test-token-123

# Server with error simulation
./test-server --port=8080 --simulate-errors --verbose
```

### Test Against Local Server

```bash
# Test all routes
./kong-route-tester --url=http://127.0.0.1:8080 --token=test-token-123

# Test only public routes
./kong-route-tester --url=http://127.0.0.1:8080 --test-auth=false

# Limit requests and show details
./kong-route-tester --url=http://127.0.0.1:8080 --max=10 --verbose
```

## Understanding Output

### Success Indicators

```bash
✓ service-name    /api/v1/users/123        GET    200      # Success
✓ service-name    /api/v1/admin           POST   201 [AUTH] # Authenticated success
```

### Error Indicators

```bash
✗ service-name    /api/v1/protected       GET    401 [AUTH] - Bearer token required
✗ service-name    /api/v1/invalid         POST   404      - Not found
```

### Summary Report

```
================================================================================
SUMMARY  
================================================================================
Total Endpoints Tested: 25
Successful (2xx/3xx):   20 (80.0%)
Auth Failed (401):      3 (12.0%)
Other Errors:           2 (8.0%)

By Status Code:
  200: 18
  201: 2
  401: 3
  404: 2

By Service:
  api-service          : 15
  auth-service         : 10

Potentially Problematic Routes (401 errors on unauthenticated routes):
  - GET /api/v1/should-be-public (auth-service)
```

## Authentication Testing

The tool automatically detects authentication requirements by analyzing Kong plugins:

### Route-Level Authentication
```yaml
routes:
  - name: protected-route
    paths: ["/api/v1/protected"]
    plugins:
      - name: auth  # Tool detects this requires authentication
```

### Service-Level Authentication
```yaml
services:
  - name: protected-service
    plugins:
      - name: auth  # All routes in this service require auth
    routes:
      - name: all-protected
        paths: ["/api/v1/data"]
```

## Advanced Features

### Regex Pattern Expansion

Kong regex patterns are automatically converted to testable paths:

| Kong Pattern | Expanded Example |
|--------------|------------------|
| `(?<user_id>[0-9a-fA-F-]+)` | `abc123def456` |
| `(?<client_id>[0-9a-fA-F-]+)` | `3a45625e-fd29-47a5-8294-e30fe2d3d391` |
| `[a-zA-Z0-9_-]+` | `test-value` |
| `(.*)` | `path` |

### Template Variable Handling

Sigil template variables are processed with fallbacks:

```yaml
services:
  - name: api-service
    url: ${API_SERVICE_ADDRESS:=http://127.0.0.1:8001}  # Fallback to localhost
    # If SERVICE_ADDRESS contains 'SERVICE_ADDRESS', defaults to services.sms.community
```

### Error Simulation

The test server can simulate various error conditions:

- **403 Forbidden**: Routes containing "admin/delete" or "backdoor"
- **405 Method Not Allowed**: DELETE requests to user endpoints  
- **429 Rate Limited**: Intermittent rate limiting on test endpoints
- **500 Internal Error**: Configurable internal server errors

## Docker Usage

Build and run with Docker:

```dockerfile
# Build
docker build -t kong-route-tester .

# Run with mounted config
docker run -v $(pwd)/kong.yaml:/root/kong.yaml kong-route-tester --file=kong.yaml --url=https://api.example.com

# Test with authentication
docker run kong-route-tester --url=https://api.example.com --token=your-token
```

## Troubleshooting

### Common Issues

**Route not found (404)**
- Verify the base URL is correct
- Check that the service is running and accessible
- Ensure Kong regex patterns are properly expanded

**Authentication failures (401)**
- Verify the Bearer token is valid and not expired
- Check that the token has appropriate scopes/permissions
- Ensure the `Authorization: Bearer <token>` header format

**Template expansion errors**
- Set required environment variables before running
- Check sigil template syntax in YAML configuration
- Verify fallback values are appropriate for testing

### Verbose Mode

Enable verbose output to see detailed request/response information:

```bash
./kong-route-tester --verbose --max=5
```

This shows:
- Exact URLs being tested
- HTTP methods being used  
- Response status codes and messages
- Authentication status for each request

## Make Targets

The project includes a comprehensive Makefile with the following targets:

### Build and Development
```bash
make build          # Build all binaries  
make clean          # Clean build artifacts
make dev            # Full development workflow (lint + test)
make deps           # Download dependencies
make tidy           # Tidy go modules
```

### Testing
```bash
make test           # Run all tests
make test-unit      # Run unit tests only  
make test-integration # Run integration tests only
make test-coverage  # Generate test coverage report
make test-race      # Run tests with race detection
```

### Code Quality
```bash
make fmt            # Format Go code
make vet            # Run go vet
make lint           # Run linters (fmt + vet + golangci-lint)
```

### Demos
```bash
make demo           # Quick demo against local test server
make demo-auth      # Demo with authentication testing
make start-test-server # Start test server in background
make stop-test-server  # Stop background test server
```

### Release
```bash
make release-build  # Build cross-platform release binaries
make docker-build   # Build Docker image
make help           # Show all available targets
```

## Development

### Project Structure

```
├── main.go              # Main Kong route tester application
├── test-server/         # Mock API server package for testing  
├── kong.yaml           # Example Kong configuration
├── kong.yaml.example   # Production Kong configuration template
├── Makefile           # Build and development automation
├── Dockerfile          # Container build configuration
├── go.mod             # Go module dependencies
├── go.sum             # Go module checksums
├── CLAUDE.md          # Claude Code guidance
└── README.md          # This file
```

### Adding New Features

1. **Route Patterns**: Add new regex patterns to `expandRegexPath()` function
2. **Authentication**: Extend `hasAuthPlugin()` to detect new auth plugin types
3. **Error Handling**: Add new error conditions to `shouldSimulateError()` in test server
4. **Output Formats**: Extend `printSummary()` for additional reporting formats

### Testing

```bash
# Run full test suite against mock server
./test-server --port=8080 --require-auth --simulate-errors &
./kong-route-tester --url=http://127.0.0.1:8080 --token=test-token-123 --verbose
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)  
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Kong](https://konghq.com/) for the API gateway inspiration
- [Sigil](https://github.com/gliderlabs/sigil) for template processing
- [pflag](https://github.com/spf13/pflag) for CLI flag handling
