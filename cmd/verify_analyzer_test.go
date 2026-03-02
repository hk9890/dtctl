package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtctl/pkg/resources/analyzer"
)

func TestFormatVerifyAnalyzerResultHuman_Valid(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := &analyzer.ValidationResult{
		Valid: true,
	}

	err := formatVerifyAnalyzerResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifyAnalyzerResultHuman failed: %v", err)
	}

	// Close writer and restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "✔ Analyzer input is valid") {
		t.Errorf("Expected '✔ Analyzer input is valid' in output, got: %s", output)
	}
}

func TestFormatVerifyAnalyzerResultHuman_Invalid(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := &analyzer.ValidationResult{
		Valid: false,
	}

	err := formatVerifyAnalyzerResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifyAnalyzerResultHuman failed: %v", err)
	}

	// Close writer and restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "✖ Analyzer input is invalid") {
		t.Errorf("Expected '✖ Analyzer input is invalid' in output, got: %s", output)
	}
}

func TestFormatVerifyAnalyzerResultHuman_WithDetails(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := &analyzer.ValidationResult{
		Valid: false,
		Details: map[string]interface{}{
			"error": "missing required field: timeSeriesData",
			"field": "timeSeriesData",
		},
	}

	err := formatVerifyAnalyzerResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifyAnalyzerResultHuman failed: %v", err)
	}

	// Close writer and restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "Details:") {
		t.Errorf("Expected 'Details:' in output, got: %s", output)
	}
	if !strings.Contains(output, "error") {
		t.Errorf("Expected 'error' key in output, got: %s", output)
	}
	if !strings.Contains(output, "timeSeriesData") {
		t.Errorf("Expected 'timeSeriesData' in output, got: %s", output)
	}
}

func TestGetVerifyAnalyzerExitCode_Valid(t *testing.T) {
	result := &analyzer.ValidationResult{
		Valid: true,
	}

	exitCode := getVerifyAnalyzerExitCode(result, nil, false)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for valid input, got %d", exitCode)
	}
}

func TestGetVerifyAnalyzerExitCode_Invalid(t *testing.T) {
	result := &analyzer.ValidationResult{
		Valid: false,
		Details: map[string]interface{}{
			"error": "missing required field",
		},
	}

	exitCode := getVerifyAnalyzerExitCode(result, nil, false)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for invalid input, got %d", exitCode)
	}
}

func TestGetVerifyAnalyzerExitCode_AuthError(t *testing.T) {
	// Test 401
	authErr := &testError{msg: "failed to validate analyzer input: status 401: Unauthorized"}
	exitCode := getVerifyAnalyzerExitCode(nil, authErr, false)
	if exitCode != 2 {
		t.Errorf("Expected exit code 2 for auth error (401), got %d", exitCode)
	}

	// Test 403
	forbiddenErr := &testError{msg: "failed to validate analyzer input: status 403: Forbidden"}
	exitCode = getVerifyAnalyzerExitCode(nil, forbiddenErr, false)
	if exitCode != 2 {
		t.Errorf("Expected exit code 2 for auth error (403), got %d", exitCode)
	}
}

func TestGetVerifyAnalyzerExitCode_NetworkError(t *testing.T) {
	// Test 5xx error
	serverErr := &testError{msg: "failed to validate analyzer input: status 500: Internal Server Error"}
	exitCode := getVerifyAnalyzerExitCode(nil, serverErr, false)
	if exitCode != 3 {
		t.Errorf("Expected exit code 3 for server error (5xx), got %d", exitCode)
	}

	// Test timeout
	timeoutErr := &testError{msg: "request timeout exceeded"}
	exitCode = getVerifyAnalyzerExitCode(nil, timeoutErr, false)
	if exitCode != 3 {
		t.Errorf("Expected exit code 3 for timeout error, got %d", exitCode)
	}

	// Test connection error
	connErr := &testError{msg: "connection refused"}
	exitCode = getVerifyAnalyzerExitCode(nil, connErr, false)
	if exitCode != 3 {
		t.Errorf("Expected exit code 3 for connection error, got %d", exitCode)
	}
}

func TestGetVerifyAnalyzerExitCode_NilResult(t *testing.T) {
	exitCode := getVerifyAnalyzerExitCode(nil, nil, false)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for nil result, got %d", exitCode)
	}
}

func TestGetVerifyAnalyzerExitCode_FailOnWarn(t *testing.T) {
	// Valid result with warning-like detail key
	result := &analyzer.ValidationResult{
		Valid: true,
		Details: map[string]interface{}{
			"warning": "deprecated field usage",
		},
	}

	// Without --fail-on-warn, should return 0
	exitCode := getVerifyAnalyzerExitCode(result, nil, false)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for warning without --fail-on-warn, got %d", exitCode)
	}

	// With --fail-on-warn, should return 1
	exitCode = getVerifyAnalyzerExitCode(result, nil, true)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for warning with --fail-on-warn, got %d", exitCode)
	}
}
