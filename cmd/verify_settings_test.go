package cmd

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestFormatVerifySettingsResultHuman_Valid(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifySettingsResult{
		Valid:    true,
		SchemaID: "builtin:alerting.profile",
		Scope:    "environment",
		Mode:     "create",
	}

	err := formatVerifySettingsResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifySettingsResultHuman failed: %v", err)
	}

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "✔ Settings object is valid") {
		t.Errorf("Expected '✔ Settings object is valid' in output, got: %s", out)
	}
	if !strings.Contains(out, "Schema: builtin:alerting.profile") {
		t.Errorf("Expected 'Schema: builtin:alerting.profile' in output, got: %s", out)
	}
	if !strings.Contains(out, "Scope: environment") {
		t.Errorf("Expected 'Scope: environment' in output, got: %s", out)
	}
	if !strings.Contains(out, "Mode: create") {
		t.Errorf("Expected 'Mode: create' in output, got: %s", out)
	}
}

func TestFormatVerifySettingsResultHuman_Invalid(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifySettingsResult{
		Valid:    false,
		SchemaID: "builtin:alerting.profile",
		Scope:    "environment",
		Mode:     "create",
		Error:    "field 'displayName' is required",
	}

	err := formatVerifySettingsResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifySettingsResultHuman failed: %v", err)
	}

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "✖ Settings object validation failed") {
		t.Errorf("Expected '✖ Settings object validation failed' in output, got: %s", out)
	}
	if !strings.Contains(out, "Schema: builtin:alerting.profile") {
		t.Errorf("Expected 'Schema: builtin:alerting.profile' in output, got: %s", out)
	}
	if !strings.Contains(out, "field 'displayName' is required") {
		t.Errorf("Expected error message in output, got: %s", out)
	}
}

func TestGetVerifySettingsExitCode_Valid(t *testing.T) {
	exitCode := getVerifySettingsExitCode(nil, nil, false)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for nil error, got %d", exitCode)
	}
}

func TestGetVerifySettingsExitCode_ValidationFailed(t *testing.T) {
	err := errors.New("validation failed: field 'displayName' is required")
	exitCode := getVerifySettingsExitCode(err, nil, false)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for validation error, got %d", exitCode)
	}
}

func TestGetVerifySettingsExitCode_AuthError(t *testing.T) {
	// Test "access denied"
	accessDeniedErr := &testError{msg: "access denied"}
	exitCode := getVerifySettingsExitCode(accessDeniedErr, nil, false)
	if exitCode != 2 {
		t.Errorf("Expected exit code 2 for access denied error, got %d", exitCode)
	}

	// Test status 403
	forbiddenErr := &testError{msg: "validation failed: status 403: Forbidden"}
	exitCode = getVerifySettingsExitCode(forbiddenErr, nil, false)
	if exitCode != 2 {
		t.Errorf("Expected exit code 2 for 403 error, got %d", exitCode)
	}

	// Test status 401
	unauthorizedErr := &testError{msg: "validation failed with status 401: Unauthorized"}
	exitCode = getVerifySettingsExitCode(unauthorizedErr, nil, false)
	if exitCode != 2 {
		t.Errorf("Expected exit code 2 for 401 error, got %d", exitCode)
	}
}

func TestGetVerifySettingsExitCode_NetworkError(t *testing.T) {
	// Test 5xx error
	serverErr := &testError{msg: "validation failed: status 500: Internal Server Error"}
	exitCode := getVerifySettingsExitCode(serverErr, nil, false)
	if exitCode != 3 {
		t.Errorf("Expected exit code 3 for server error (5xx), got %d", exitCode)
	}

	// Test timeout
	timeoutErr := &testError{msg: "request timeout exceeded"}
	exitCode = getVerifySettingsExitCode(timeoutErr, nil, false)
	if exitCode != 3 {
		t.Errorf("Expected exit code 3 for timeout error, got %d", exitCode)
	}

	// Test connection error
	connErr := &testError{msg: "connection refused"}
	exitCode = getVerifySettingsExitCode(connErr, nil, false)
	if exitCode != 3 {
		t.Errorf("Expected exit code 3 for connection error, got %d", exitCode)
	}
}

func TestGetVerifySettingsExitCode_FailOnWarn(t *testing.T) {
	// Without --fail-on-warn, warnings should return 0
	exitCode := getVerifySettingsExitCode(nil, []string{"deprecated scope"}, false)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for warning without --fail-on-warn, got %d", exitCode)
	}

	// With --fail-on-warn, warnings should return 1
	exitCode = getVerifySettingsExitCode(nil, []string{"deprecated scope"}, true)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for warning with --fail-on-warn, got %d", exitCode)
	}

	// No warnings + --fail-on-warn should still return 0
	exitCode = getVerifySettingsExitCode(nil, nil, true)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for no warnings with --fail-on-warn, got %d", exitCode)
	}

	// Error takes precedence over --fail-on-warn
	exitCode = getVerifySettingsExitCode(errors.New("validation failed"), []string{"some warning"}, true)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for error with --fail-on-warn, got %d", exitCode)
	}
}

func TestFormatVerifySettingsResultHuman_Warnings(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifySettingsResult{
		Valid:    true,
		SchemaID: "builtin:alerting.profile",
		Scope:    "environment",
		Mode:     "create",
		Warnings: []string{"scope is deprecated", "field will be removed"},
	}

	err := formatVerifySettingsResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifySettingsResultHuman failed: %v", err)
	}

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "✔ Settings object is valid") {
		t.Errorf("Expected '✔ Settings object is valid' in output, got: %s", out)
	}
	if !strings.Contains(out, "scope is deprecated") {
		t.Errorf("Expected warning 'scope is deprecated' in output, got: %s", out)
	}
	if !strings.Contains(out, "field will be removed") {
		t.Errorf("Expected warning 'field will be removed' in output, got: %s", out)
	}
}

func TestVerifySettingsParseInput_CreateMode(t *testing.T) {
	// Simulate what RunE does: parse input map for create mode
	input := map[string]any{
		"schemaId": "builtin:alerting.profile",
		"scope":    "environment",
		"value": map[string]any{
			"displayName":   "My Profile",
			"eventFilters":  []any{},
			"severityRules": []any{},
		},
	}

	// Extract schemaId
	schemaID, _ := input["schemaId"].(string)
	if schemaID != "builtin:alerting.profile" {
		t.Errorf("Expected schemaId 'builtin:alerting.profile', got %q", schemaID)
	}

	// Extract scope
	scope, _ := input["scope"].(string)
	if scope != "environment" {
		t.Errorf("Expected scope 'environment', got %q", scope)
	}

	// Extract value
	valueRaw, ok := input["value"]
	if !ok {
		t.Fatal("Expected 'value' key in input")
	}
	valueMap, ok := valueRaw.(map[string]any)
	if !ok {
		t.Fatal("Expected 'value' to be a map")
	}
	if _, ok := valueMap["displayName"]; !ok {
		t.Error("Expected 'displayName' key in value map")
	}
}

func TestVerifySettingsParseInput_UpdateMode(t *testing.T) {
	// Simulate what RunE does: parse input map for update mode (no schemaId/scope required at top level)
	input := map[string]any{
		"value": map[string]any{
			"displayName":   "Updated Profile",
			"eventFilters":  []any{},
			"severityRules": []any{},
		},
	}

	// Extract value
	valueRaw, ok := input["value"]
	if !ok {
		// Fall back to using whole input as value
		valueRaw = input
	}
	valueMap, ok := valueRaw.(map[string]any)
	if !ok {
		t.Fatal("Expected 'value' to be a map")
	}
	if _, ok := valueMap["displayName"]; !ok {
		t.Error("Expected 'displayName' key in value map")
	}

	// When there's no "value" key, use whole input
	inputNoValue := map[string]any{
		"displayName":   "Updated Profile",
		"eventFilters":  []any{},
		"severityRules": []any{},
	}
	valueRaw2, exists := inputNoValue["value"]
	if !exists {
		valueRaw2 = inputNoValue
	}
	valueMap2, ok := valueRaw2.(map[string]any)
	if !ok {
		t.Fatal("Expected fallback value to be a map")
	}
	if _, ok := valueMap2["displayName"]; !ok {
		t.Error("Expected 'displayName' key in fallback value map")
	}
}
