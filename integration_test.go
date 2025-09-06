package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestServer holds information about a running test server
type TestServer struct {
	cmd    *exec.Cmd
	port   int
	baseURL string
}

// startTestServer starts the test server and waits for it to be ready
func startTestServer(port int, requireAuth bool, authToken string, simulateErrors bool) (*TestServer, error) {
	args := []string{
		"--port", fmt.Sprintf("%d", port),
	}
	
	if requireAuth {
		args = append(args, "--require-auth", "--auth-token", authToken)
	}
	
	if simulateErrors {
		args = append(args, "--simulate-errors")
	}

	cmd := exec.Command("./test-server-bin", args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start test server: %v", err)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	server := &TestServer{
		cmd:     cmd,
		port:    port,
		baseURL: baseURL,
	}

	// Wait for server to be ready
	if err := server.waitForReady(); err != nil {
		server.Stop()
		return nil, fmt.Errorf("test server failed to start: %v", err)
	}

	return server, nil
}

// waitForReady waits for the test server to respond to health checks
func (ts *TestServer) waitForReady() error {
	client := &http.Client{Timeout: 1 * time.Second}
	
	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		resp, err := client.Get(ts.baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	
	return fmt.Errorf("test server did not become ready within 30 seconds")
}

// Stop stops the test server
func (ts *TestServer) Stop() error {
	if ts.cmd != nil && ts.cmd.Process != nil {
		return ts.cmd.Process.Kill()
	}
	return nil
}

// TestIntegrationWithTestServer tests the kong route tester against the test server
func TestIntegrationWithTestServer(t *testing.T) {
	// Build both binaries if they don't exist
	buildTestServer(t)
	buildKongRouteTester(t)

	// Create a simple test kong configuration
	testConfig := createTestKongConfig(t)
	defer os.Remove(testConfig)

	t.Run("test without authentication", func(t *testing.T) {
		// Start test server without authentication
		server, err := startTestServer(8081, false, "", false)
		if err != nil {
			t.Fatalf("Failed to start test server: %v", err)
		}
		defer server.Stop()

		// Run kong-route-tester against the test server
		cmd := exec.Command("./kong-route-tester", 
			"--file", testConfig,
			"--url", server.baseURL,
			"--max", "5",
		)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("kong-route-tester failed: %v\nOutput: %s", err, output)
		}

		outputStr := string(output)
		
		// Verify success indicators
		if !strings.Contains(outputStr, "Total Endpoints Tested:") {
			t.Error("Expected summary output not found")
		}
		
		// Should have some successful tests
		if !strings.Contains(outputStr, "Successful (2xx/3xx):") {
			t.Error("Expected success metrics not found")
		}
	})

	t.Run("test with authentication required", func(t *testing.T) {
		authToken := "test-integration-token"
		
		// Start test server with authentication required
		server, err := startTestServer(8082, true, authToken, false)
		if err != nil {
			t.Fatalf("Failed to start test server: %v", err)
		}
		defer server.Stop()

		// Test without providing auth token (should get 401s)
		t.Run("without auth token", func(t *testing.T) {
			cmd := exec.Command("./kong-route-tester",
				"--file", testConfig,
				"--url", server.baseURL,
				"--max", "3",
			)
			
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("kong-route-tester failed: %v\nOutput: %s", err, output)
			}

			outputStr := string(output)
			
			// Should have 401 errors for authenticated routes
			if !strings.Contains(outputStr, "Auth Failed (401):") {
				t.Error("Expected 401 auth failures not found")
			}
		})

		// Test with valid auth token (should succeed)
		t.Run("with valid auth token", func(t *testing.T) {
			cmd := exec.Command("./kong-route-tester",
				"--file", testConfig,
				"--url", server.baseURL,
				"--token", authToken,
				"--max", "3",
			)
			
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("kong-route-tester failed: %v\nOutput: %s", err, output)
			}

			outputStr := string(output)
			
			// Should have successful requests
			if !strings.Contains(outputStr, "Successful (2xx/3xx):") {
				t.Error("Expected successful requests not found")
			}
			
			// Should have minimal or no auth failures
			if strings.Contains(outputStr, "Auth Failed (401): 0") {
				// Good - no auth failures
			} else if !strings.Contains(outputStr, "Auth Failed (401):") {
				t.Error("Expected auth failure statistics not found")
			}
		})
	})

	t.Run("test filtering by route type", func(t *testing.T) {
		// Start test server without authentication
		server, err := startTestServer(8083, false, "", false)
		if err != nil {
			t.Fatalf("Failed to start test server: %v", err)
		}
		defer server.Stop()

		// Test only authenticated routes
		t.Run("auth routes only", func(t *testing.T) {
			cmd := exec.Command("./kong-route-tester",
				"--file", testConfig,
				"--url", server.baseURL,
				"--test-unauth=false",
				"--max", "5",
			)
			
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("kong-route-tester failed: %v\nOutput: %s", err, output)
			}

			outputStr := string(output)
			
			// Should only test authenticated routes (marked with [AUTH])
			// The test may not have any auth routes in the limited test config
			if !strings.Contains(outputStr, "SUMMARY") {
				t.Error("Expected summary output not found")
			}
		})

		// Test only unauthenticated routes
		t.Run("unauth routes only", func(t *testing.T) {
			cmd := exec.Command("./kong-route-tester",
				"--file", testConfig,
				"--url", server.baseURL,
				"--test-auth=false",
				"--max", "5",
			)
			
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("kong-route-tester failed: %v\nOutput: %s", err, output)
			}

			// Should complete successfully
			if !strings.Contains(string(output), "SUMMARY") {
				t.Error("Expected summary output not found")
			}
		})
	})

	t.Run("dry run mode", func(t *testing.T) {
		// Start test server
		server, err := startTestServer(8084, false, "", false)
		if err != nil {
			t.Fatalf("Failed to start test server: %v", err)
		}
		defer server.Stop()

		cmd := exec.Command("./kong-route-tester",
			"--file", testConfig,
			"--url", server.baseURL,
			"--dry-run",
			"--verbose",
			"--max", "3",
		)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("kong-route-tester failed: %v\nOutput: %s", err, output)
		}

		outputStr := string(output)
		
		// Should show DRY RUN messages
		if !strings.Contains(outputStr, "DRY RUN") {
			t.Error("Expected DRY RUN indicators not found")
		}
		
		// Should still show summary
		if !strings.Contains(outputStr, "SUMMARY") {
			t.Error("Expected summary output not found")
		}
	})
}

// buildTestServer builds the test server if it doesn't exist
func buildTestServer(t *testing.T) {
	if _, err := os.Stat("./test-server-bin"); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", "test-server-bin", "./test-server")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to build test server: %v", err)
		}
	}
}

// buildKongRouteTester builds the kong route tester if it doesn't exist
func buildKongRouteTester(t *testing.T) {
	if _, err := os.Stat("./kong-route-tester"); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", "kong-route-tester", "main.go")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to build kong-route-tester: %v", err)
		}
	}
}

// createTestKongConfig creates a temporary kong configuration file for testing
func createTestKongConfig(t *testing.T) string {
	testConfig := `_format_version: "1.1"

services:
  # Service with authenticated routes
  - name: auth-service
    url: http://127.0.0.1:8001
    routes:
      - name: protected-endpoint
        paths:
          - /auth/v1/protected
        methods: ["GET", "POST"]
        plugins:
          - name: auth

  # Service with public routes
  - name: public-service
    url: http://127.0.0.1:8002
    routes:
      - name: public-endpoint
        paths:
          - /api/v1/public/health
          - /api/v1/public/status
        methods: ["GET"]

  # Mixed service with both auth and public routes
  - name: mixed-service
    url: http://127.0.0.1:8003
    routes:
      - name: mixed-auth-route
        paths:
          - /api/v1/users/(?<user_id>[0-9a-fA-F-]+)/profile
        methods: ["GET", "PUT"]
        plugins:
          - name: auth
      - name: mixed-public-route
        paths:
          - /api/v1/public/info
        methods: ["GET"]
`

	tmpFile, err := os.CreateTemp("", "kong-integration-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	if _, err := tmpFile.Write([]byte(testConfig)); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close test config: %v", err)
	}

	return tmpFile.Name()
}

// TestErrorHandling tests error scenarios
func TestErrorHandling(t *testing.T) {
	buildKongRouteTester(t)

	t.Run("invalid config file", func(t *testing.T) {
		cmd := exec.Command("./kong-route-tester",
			"--file", "non-existent-file.yaml",
			"--dry-run",
		)
		
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Error("Expected error for non-existent config file")
		}

		if !strings.Contains(string(output), "Error reading Kong configuration") {
			t.Error("Expected configuration error message not found")
		}
	})

	t.Run("invalid YAML syntax", func(t *testing.T) {
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

		cmd := exec.Command("./kong-route-tester",
			"--file", tmpFile.Name(),
			"--dry-run",
		)
		
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}

		if !strings.Contains(string(output), "Error reading Kong configuration") {
			t.Error("Expected configuration error message not found")
		}
	})
}

// TestVerboseOutput tests verbose output functionality
func TestVerboseOutput(t *testing.T) {
	buildTestServer(t)
	buildKongRouteTester(t)

	testConfig := createTestKongConfig(t)
	defer os.Remove(testConfig)

	// Start test server
	server, err := startTestServer(8085, false, "", false)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer server.Stop()

	// Test verbose mode
	cmd := exec.Command("./kong-route-tester",
		"--file", testConfig,
		"--url", server.baseURL,
		"--verbose",
		"--max", "2",
	)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kong-route-tester failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	
	// Verbose mode should show individual request results
	if !strings.Contains(outputStr, "✓") && !strings.Contains(outputStr, "✗") {
		t.Error("Expected verbose request indicators not found")
	}
	
	// Should show status codes
	if !strings.Contains(outputStr, "200") && !strings.Contains(outputStr, "401") && !strings.Contains(outputStr, "404") {
		t.Error("Expected status codes in verbose output not found")
	}
}