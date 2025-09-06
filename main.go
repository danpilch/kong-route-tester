package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v4"
)

// Kong configuration structures
type KongConfig struct {
	Services []Service `yaml:"services"`
}

type Service struct {
	Name    string   `yaml:"name"`
	URL     string   `yaml:"url"`
	Plugins []Plugin `yaml:"plugins"`
	Routes  []Route  `yaml:"routes"`
}

type Route struct {
	Name     string   `yaml:"name"`
	Paths    []string `yaml:"paths"`
	Methods  []string `yaml:"methods"`
	Hosts    []string `yaml:"hosts"`
	Plugins  []Plugin `yaml:"plugins"`
	Priority int      `yaml:"regex_priority"`
}

type Plugin struct {
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config"`
}

// Test result structures
type TestResult struct {
	Service      string
	Route        string
	Path         string
	Method       string
	RequiresAuth bool
	StatusCode   int
	Error        error
	Message      string
}

// Configuration flags
var (
	kongFile    = pflag.String("file", "kong.yaml", "Path to Kong configuration file")
	baseURL     = pflag.String("url", "https://api.dev.community.com", "Base URL for testing")
	authToken   = pflag.String("token", "", "Authentication token for testing authenticated routes")
	testAuth    = pflag.Bool("test-auth", true, "Test authenticated routes")
	testUnauth  = pflag.Bool("test-unauth", true, "Test unauthenticated routes")
	verbose     = pflag.Bool("verbose", false, "Verbose output")
	dryRun      = pflag.Bool("dry-run", false, "Dry run - show what would be tested without making requests")
	maxRequests = pflag.Int("max", 0, "Maximum number of requests to make (0 = unlimited)")
)

func main() {
	pflag.Parse()

	// Read Kong configuration
	config, err := readKongConfig(*kongFile)
	if err != nil {
		fmt.Printf("Error reading Kong configuration: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	results := testRoutes(config)

	// Print summary
	printSummary(results)
}

func readKongConfig(filename string) (*KongConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Handle environment variable substitution (basic sigil templating)
	data = handleTemplating(data)

	var config KongConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func handleTemplating(data []byte) []byte {
	// Simple handling of ${VAR:?} and ${VAR:=default} patterns
	re := regexp.MustCompile(`\$\{([^:}]+)(:[?=][^}]*)?\}`)

	result := re.ReplaceAllFunc(data, func(match []byte) []byte {
		parts := strings.Split(string(match[2:len(match)-1]), ":")
		envVar := parts[0]

		value := os.Getenv(envVar)
		if value != "" {
			return []byte(value)
		}

		// For testing, use placeholder values
		if strings.Contains(envVar, "SERVICE_ADDRESS") {
			return []byte("http://services.sms.community:10000")
		}

		return []byte("http://placeholder")
	})

	return result
}

func testRoutes(config *KongConfig) []TestResult {
	var results []TestResult
	requestCount := 0

	for _, service := range config.Services {
		// Skip certain test services
		if strings.Contains(service.Name, "test") ||
			strings.Contains(service.Name, "health-check") ||
			service.Name == "atlantis" ||
			service.Name == "atlantis-legacy" {
			if *verbose {
				fmt.Printf("Skipping test service: %s\n", service.Name)
			}
			continue
		}

		for _, route := range service.Routes {
			hasAuth := hasAuthPlugin(route, service)

			// Check if we should test this route
			if hasAuth && !*testAuth {
				continue
			}
			if !hasAuth && !*testUnauth {
				continue
			}

			// Determine methods to test
			methods := route.Methods
			if len(methods) == 0 {
				// No methods specified means all methods in Kong 3.x
				methods = []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
			}

			// Test each path/method combination
			for _, path := range route.Paths {
				// Skip regex patterns for now unless we have specific test cases
				if strings.Contains(path, "(?<") {
					path = expandRegexPath(path)
				}

				for _, method := range methods {
					if *maxRequests > 0 && requestCount >= *maxRequests {
						return results
					}

					result := testEndpoint(service.Name, route.Name, path, method, hasAuth)
					results = append(results, result)
					requestCount++

					// Rate limiting
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}

	return results
}

func hasAuthPlugin(route Route, service Service) bool {
	// Check route plugins
	for _, plugin := range route.Plugins {
		if plugin.Name == "auth" {
			return true
		}
	}

	// Check service plugins
	for _, plugin := range service.Plugins {
		if plugin.Name == "auth" {
			return true
		}
	}

	return false
}

func expandRegexPath(path string) string {
	// Convert regex patterns to example paths for testing
	replacements := map[string]string{
		`(?<client_id>[0-9a-fA-F-]+)`:    "3a45625e-fd29-47a5-8294-e30fe2d3d391",
		`(?<seat_id>[0-9a-fA-F-]+)`:      "123e4567-e89b-12d3-a456-426614174000",
		`(?<invite_token>[0-9a-fA-F-]+)`: "987fcdeb-51a2-43e1-b210-0123456789ab",
		`(?<test_id>[0-9a-fA-F-]+)`:      "test-id-123",
		`(?<user_id>[^/]+)`:              "user123",
		`(?<embed_id>[0-9a-fA-F-]+)`:     "embed-456",
		`[0-9a-fA-F-]+`:                  "abc123def456",
		`[a-zA-Z0-9_-]+`:                 "test-value",
		`[^/]+`:                          "example",
		`(.*)`:                           "path",
	}

	result := path
	for pattern, replacement := range replacements {
		result = strings.ReplaceAll(result, pattern, replacement)
	}

	return result
}

func testEndpoint(service, route, path, method string, requiresAuth bool) TestResult {
	result := TestResult{
		Service:      service,
		Route:        route,
		Path:         path,
		Method:       method,
		RequiresAuth: requiresAuth,
	}

	if *dryRun {
		result.Message = "DRY RUN"
		printResult(result)
		return result
	}

	url := *baseURL + path

	// Create request
	var req *http.Request
	var err error

	// Add sample body for POST/PUT requests
	if method == "POST" || method == "PUT" || method == "PATCH" {
		body := bytes.NewBuffer([]byte(`{"test": "data"}`))
		req, err = http.NewRequest(method, url, body)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		result.Error = err
		printResult(result)
		return result
	}

	// Add auth header if required
	if requiresAuth && *authToken != "" {
		req.Header.Set("Authorization", "Bearer "+*authToken)
	}

	// Make request
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Error = err
		printResult(result)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	// Read response body for error messages
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			var errorResp map[string]interface{}
			if err := json.Unmarshal(body, &errorResp); err == nil {
				if errors, ok := errorResp["errors"].([]interface{}); ok && len(errors) > 0 {
					if errorMap, ok := errors[0].(map[string]interface{}); ok {
						if msg, ok := errorMap["message"].(string); ok {
							result.Message = msg
						}
					}
				} else if msg, ok := errorResp["message"].(string); ok {
					result.Message = msg
				}
			} else {
				result.Message = string(body)
			}
		}
	}

	printResult(result)
	return result
}

func printResult(result TestResult) {
	if !*verbose && result.StatusCode >= 200 && result.StatusCode < 400 {
		return // Only show errors in non-verbose mode
	}

	status := "✓"
	if result.StatusCode >= 400 || result.Error != nil {
		status = "✗"
	} else if result.StatusCode == 0 {
		status = "○"
	}

	authStr := ""
	if result.RequiresAuth {
		authStr = " [AUTH]"
	}

	fmt.Printf("%s %-30s %-40s %-6s %3d%s",
		status,
		result.Service,
		truncate(result.Path, 40),
		result.Method,
		result.StatusCode,
		authStr)

	if result.Error != nil {
		fmt.Printf(" ERROR: %v", result.Error)
	} else if result.Message != "" {
		fmt.Printf(" - %s", truncate(result.Message, 50))
	}

	fmt.Println()
}

func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

func printSummary(results []TestResult) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("SUMMARY")
	fmt.Println(strings.Repeat("=", 80))

	total := len(results)
	successful := 0
	authFailed := 0
	otherErrors := 0
	byService := make(map[string]int)
	byStatusCode := make(map[int]int)

	for _, result := range results {
		byService[result.Service]++
		byStatusCode[result.StatusCode]++

		if result.StatusCode >= 200 && result.StatusCode < 400 {
			successful++
		} else if result.StatusCode == 401 {
			authFailed++
		} else if result.StatusCode >= 400 || result.Error != nil {
			otherErrors++
		}
	}

	fmt.Printf("Total Endpoints Tested: %d\n", total)
	fmt.Printf("Successful (2xx/3xx):   %d (%.1f%%)\n", successful, float64(successful)/float64(total)*100)
	fmt.Printf("Auth Failed (401):      %d (%.1f%%)\n", authFailed, float64(authFailed)/float64(total)*100)
	fmt.Printf("Other Errors:           %d (%.1f%%)\n", otherErrors, float64(otherErrors)/float64(total)*100)

	fmt.Println("\nBy Status Code:")
	for code, count := range byStatusCode {
		fmt.Printf("  %d: %d\n", code, count)
	}

	fmt.Println("\nBy Service:")
	for service, count := range byService {
		fmt.Printf("  %-30s: %d\n", service, count)
	}

	// Show problematic routes
	fmt.Println("\nPotentially Problematic Routes (401 errors on unauthenticated routes):")
	for _, result := range results {
		if result.StatusCode == 401 && !result.RequiresAuth {
			fmt.Printf("  - %s %s (%s)\n", result.Method, result.Path, result.Service)
		}
	}
}
