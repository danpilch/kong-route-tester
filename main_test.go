package main

import (
	"os"
	"reflect"
	"testing"
)

func TestHandleTemplating(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "simple environment variable substitution",
			input:    "url: ${API_URL}",
			envVars:  map[string]string{"API_URL": "https://api.example.com"},
			expected: "url: https://api.example.com",
		},
		{
			name:     "environment variable with default value",
			input:    "url: ${API_URL:=http://localhost:8080}",
			envVars:  map[string]string{},
			expected: "url: http://placeholder",
		},
		{
			name:     "environment variable overrides default",
			input:    "url: ${API_URL:=http://localhost:8080}",
			envVars:  map[string]string{"API_URL": "https://production.com"},
			expected: "url: https://production.com",
		},
		{
			name:     "SERVICE_ADDRESS special handling",
			input:    "url: ${MY_SERVICE_ADDRESS}",
			envVars:  map[string]string{},
			expected: "url: http://services.sms.community:10000",
		},
		{
			name:     "multiple variable substitution",
			input:    "host: ${HOST} port: ${PORT:=8080}",
			envVars:  map[string]string{"HOST": "localhost"},
			expected: "host: localhost port: http://placeholder",
		},
		{
			name:     "no substitution needed",
			input:    "url: http://static.example.com",
			envVars:  map[string]string{},
			expected: "url: http://static.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			result := handleTemplating([]byte(tt.input))

			// Clean up environment variables
			for key := range tt.envVars {
				os.Unsetenv(key)
			}

			if string(result) != tt.expected {
				t.Errorf("handleTemplating() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestHasAuthPlugin(t *testing.T) {
	tests := []struct {
		name     string
		route    Route
		service  Service
		expected bool
	}{
		{
			name: "route has auth plugin",
			route: Route{
				Name: "test-route",
				Plugins: []Plugin{
					{Name: "auth"},
				},
			},
			service:  Service{},
			expected: true,
		},
		{
			name:  "service has auth plugin",
			route: Route{Name: "test-route"},
			service: Service{
				Plugins: []Plugin{
					{Name: "auth"},
				},
			},
			expected: true,
		},
		{
			name: "both route and service have auth plugin",
			route: Route{
				Name: "test-route",
				Plugins: []Plugin{
					{Name: "auth"},
				},
			},
			service: Service{
				Plugins: []Plugin{
					{Name: "auth"},
				},
			},
			expected: true,
		},
		{
			name: "route has other plugins but not auth",
			route: Route{
				Name: "test-route",
				Plugins: []Plugin{
					{Name: "rate-limiting"},
					{Name: "cors"},
				},
			},
			service:  Service{},
			expected: false,
		},
		{
			name:  "service has other plugins but not auth",
			route: Route{Name: "test-route"},
			service: Service{
				Plugins: []Plugin{
					{Name: "prometheus"},
					{Name: "cors"},
				},
			},
			expected: false,
		},
		{
			name:     "no plugins on route or service",
			route:    Route{Name: "test-route"},
			service:  Service{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAuthPlugin(tt.route, tt.service)
			if result != tt.expected {
				t.Errorf("hasAuthPlugin() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExpandRegexPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "generic hex pattern",
			input:    "/items/[0-9a-fA-F-]+/details",
			expected: "/items/abc123def456/details",
		},
		{
			name:     "alphanumeric pattern",
			input:    "/slugs/[a-zA-Z0-9_-]+",
			expected: "/slugs/test-value",
		},
		{
			name:     "any non-slash pattern",
			input:    "/dynamic/[^/]+/path",
			expected: "/dynamic/example/path",
		},
		{
			name:     "wildcard pattern",
			input:    "/catchall/(.*)",
			expected: "/catchall/path",
		},
		{
			name:     "no regex patterns",
			input:    "/api/v1/static/endpoint",
			expected: "/api/v1/static/endpoint",
		},
		{
			name:     "test_id pattern works",
			input:    "/tests/(?<test_id>[0-9a-fA-F-]+)",
			expected: "/tests/test-id-123",
		},
		{
			name:     "embed_id pattern works",
			input:    "/embeds/(?<embed_id>[0-9a-fA-F-]+)",
			expected: "/embeds/embed-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandRegexPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandRegexPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestReadKongConfig(t *testing.T) {
	// Create a temporary test file
	testYAML := `_format_version: "1.1"
services:
  - name: test-service
    url: http://localhost:8080
    plugins:
      - name: auth
    routes:
      - name: test-route
        paths:
          - /api/v1/test
        methods: ["GET", "POST"]
        plugins:
          - name: rate-limiting
            config:
              minute: 100
`

	// Write test file
	tmpFile, err := os.CreateTemp("", "kong-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(testYAML)); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close test file: %v", err)
	}

	// Test reading the configuration
	config, err := readKongConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("readKongConfig() error = %v", err)
	}

	// Verify the parsed configuration
	if len(config.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(config.Services))
	}

	service := config.Services[0]
	if service.Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got %q", service.Name)
	}

	if service.URL != "http://localhost:8080" {
		t.Errorf("Expected service URL 'http://localhost:8080', got %q", service.URL)
	}

	if len(service.Plugins) != 1 {
		t.Errorf("Expected 1 service plugin, got %d", len(service.Plugins))
	}

	if service.Plugins[0].Name != "auth" {
		t.Errorf("Expected service plugin 'auth', got %q", service.Plugins[0].Name)
	}

	if len(service.Routes) != 1 {
		t.Errorf("Expected 1 route, got %d", len(service.Routes))
	}

	route := service.Routes[0]
	if route.Name != "test-route" {
		t.Errorf("Expected route name 'test-route', got %q", route.Name)
	}

	expectedPaths := []string{"/api/v1/test"}
	if !reflect.DeepEqual(route.Paths, expectedPaths) {
		t.Errorf("Expected paths %v, got %v", expectedPaths, route.Paths)
	}

	expectedMethods := []string{"GET", "POST"}
	if !reflect.DeepEqual(route.Methods, expectedMethods) {
		t.Errorf("Expected methods %v, got %v", expectedMethods, route.Methods)
	}

	if len(route.Plugins) != 1 {
		t.Errorf("Expected 1 route plugin, got %d", len(route.Plugins))
	}

	if route.Plugins[0].Name != "rate-limiting" {
		t.Errorf("Expected route plugin 'rate-limiting', got %q", route.Plugins[0].Name)
	}
}

func TestReadKongConfigWithTemplating(t *testing.T) {
	// Set environment variable for test
	os.Setenv("TEST_SERVICE_URL", "http://test.example.com")
	defer os.Unsetenv("TEST_SERVICE_URL")

	testYAML := `_format_version: "1.1"
services:
  - name: templated-service
    url: ${TEST_SERVICE_URL}
    routes:
      - name: templated-route
        paths:
          - /api/v1/templated
`

	// Write test file
	tmpFile, err := os.CreateTemp("", "kong-templated-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(testYAML)); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close test file: %v", err)
	}

	// Test reading the configuration
	config, err := readKongConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("readKongConfig() error = %v", err)
	}

	// Verify template was processed
	if config.Services[0].URL != "http://test.example.com" {
		t.Errorf("Expected templated URL 'http://test.example.com', got %q", config.Services[0].URL)
	}
}

func TestReadKongConfigError(t *testing.T) {
	// Test with non-existent file
	_, err := readKongConfig("non-existent-file.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	// Test with invalid YAML
	invalidYAML := `invalid: yaml: content:
    - missing: bracket
  malformed`

	tmpFile, err := os.CreateTemp("", "kong-invalid-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(invalidYAML)); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close test file: %v", err)
	}

	_, err = readKongConfig(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		length   int
		expected string
	}{
		{
			name:     "string shorter than length",
			input:    "hello",
			length:   10,
			expected: "hello",
		},
		{
			name:     "string equal to length",
			input:    "hello",
			length:   5,
			expected: "hello",
		},
		{
			name:     "string longer than length",
			input:    "hello world",
			length:   8,
			expected: "hello...",
		},
		{
			name:     "very short length",
			input:    "hello world",
			length:   3,
			expected: "...",
		},
		{
			name:     "empty string",
			input:    "",
			length:   5,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.length)
			if result != tt.expected {
				t.Errorf("truncate() = %q, want %q", result, tt.expected)
			}
		})
	}
}