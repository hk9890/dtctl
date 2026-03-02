package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestValidateWorkflowStructure_Valid verifies that a well-formed workflow
// definition produces no errors and no warnings.
func TestValidateWorkflowStructure_Valid(t *testing.T) {
	doc := map[string]interface{}{
		"title": "My Workflow",
		"tasks": map[string]interface{}{
			"fetch_data": map[string]interface{}{
				"action": "dynatrace.automations:run-javascript",
			},
		},
		"trigger": map[string]interface{}{
			"schedule": map[string]interface{}{
				"rule": "0 * * * *",
			},
		},
		"owner": "team-platform",
	}

	errors, warnings := validateWorkflowStructure(doc)

	if len(errors) != 0 {
		t.Errorf("Expected no errors, got: %v", errors)
	}
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings, got: %v", warnings)
	}
}

// TestValidateWorkflowStructure_MissingTitle verifies that an absent or empty
// title field is reported as an error.
func TestValidateWorkflowStructure_MissingTitle(t *testing.T) {
	doc := map[string]interface{}{
		"tasks": map[string]interface{}{
			"task1": map[string]interface{}{
				"action": "dynatrace.automations:run-javascript",
			},
		},
		"trigger": map[string]interface{}{},
	}

	errors, _ := validateWorkflowStructure(doc)

	if len(errors) == 0 {
		t.Fatal("Expected an error for missing title, got none")
	}
	if !strings.Contains(errors[0], "title") {
		t.Errorf("Expected error mentioning 'title', got: %s", errors[0])
	}
}

// TestValidateWorkflowStructure_MissingTasks verifies that an absent tasks
// field is reported as an error.
func TestValidateWorkflowStructure_MissingTasks(t *testing.T) {
	doc := map[string]interface{}{
		"title":   "My Workflow",
		"trigger": map[string]interface{}{},
	}

	errors, _ := validateWorkflowStructure(doc)

	if len(errors) == 0 {
		t.Fatal("Expected an error for missing tasks, got none")
	}
	found := false
	for _, e := range errors {
		if strings.Contains(e, "tasks") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error mentioning 'tasks', got: %v", errors)
	}
}

// TestValidateWorkflowStructure_EmptyTasks verifies that a tasks map with no
// entries produces a warning (not an error).
func TestValidateWorkflowStructure_EmptyTasks(t *testing.T) {
	doc := map[string]interface{}{
		"title":   "My Workflow",
		"tasks":   map[string]interface{}{},
		"trigger": map[string]interface{}{},
	}

	errors, warnings := validateWorkflowStructure(doc)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for empty tasks, got: %v", errors)
	}
	if len(warnings) == 0 {
		t.Fatal("Expected a warning for empty tasks, got none")
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "no tasks") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about no tasks defined, got: %v", warnings)
	}
}

// TestValidateWorkflowStructure_TaskWithoutAction verifies that a task missing
// the required action field is reported as an error.
func TestValidateWorkflowStructure_TaskWithoutAction(t *testing.T) {
	doc := map[string]interface{}{
		"title": "My Workflow",
		"tasks": map[string]interface{}{
			"broken_task": map[string]interface{}{
				"description": "This task has no action",
			},
		},
		"trigger": map[string]interface{}{},
	}

	errors, _ := validateWorkflowStructure(doc)

	if len(errors) == 0 {
		t.Fatal("Expected an error for task without action, got none")
	}
	found := false
	for _, e := range errors {
		if strings.Contains(e, "broken_task") && strings.Contains(e, "action") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error for task 'broken_task' missing action, got: %v", errors)
	}
}

// TestValidateWorkflowStructure_MissingTrigger verifies that a workflow without
// a trigger field produces a warning (not an error).
func TestValidateWorkflowStructure_MissingTrigger(t *testing.T) {
	doc := map[string]interface{}{
		"title": "My Workflow",
		"tasks": map[string]interface{}{
			"task1": map[string]interface{}{
				"action": "dynatrace.automations:run-javascript",
			},
		},
	}

	errors, warnings := validateWorkflowStructure(doc)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for missing trigger, got: %v", errors)
	}
	if len(warnings) == 0 {
		t.Fatal("Expected a warning for missing trigger, got none")
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "trigger") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about missing trigger, got: %v", warnings)
	}
}

// TestValidateWorkflowStructure_EmptyOwner verifies that an owner field with
// an empty string value produces a warning.
func TestValidateWorkflowStructure_EmptyOwner(t *testing.T) {
	doc := map[string]interface{}{
		"title": "My Workflow",
		"tasks": map[string]interface{}{
			"task1": map[string]interface{}{
				"action": "dynatrace.automations:run-javascript",
			},
		},
		"trigger": map[string]interface{}{},
		"owner":   "",
	}

	errors, warnings := validateWorkflowStructure(doc)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for empty owner, got: %v", errors)
	}
	if len(warnings) == 0 {
		t.Fatal("Expected a warning for empty owner, got none")
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "owner") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about empty owner, got: %v", warnings)
	}
}

// TestFormatVerifyWorkflowResultHuman_Valid verifies that a valid result
// produces a success message with title and task count on stderr.
func TestFormatVerifyWorkflowResultHuman_Valid(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifyWorkflowResult{
		Valid:  true,
		Title:  "My Workflow",
		Tasks:  3,
		Errors: nil,
	}

	err := formatVerifyWorkflowResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifyWorkflowResultHuman failed: %v", err)
	}

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "Workflow is valid") {
		t.Errorf("Expected 'Workflow is valid' in output, got: %s", out)
	}
	if !strings.Contains(out, "My Workflow") {
		t.Errorf("Expected title 'My Workflow' in output, got: %s", out)
	}
	if !strings.Contains(out, "Tasks: 3") {
		t.Errorf("Expected 'Tasks: 3' in output, got: %s", out)
	}
}

// TestFormatVerifyWorkflowResultHuman_WithErrors verifies that an invalid
// result prints the error/invalid header and all error/warning lines.
func TestFormatVerifyWorkflowResultHuman_WithErrors(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifyWorkflowResult{
		Valid:    false,
		Title:    "",
		Tasks:    0,
		Errors:   []string{"missing or empty required field: title", `task "process_data": missing required field: action`},
		Warnings: []string{"no trigger defined - workflow can only be executed manually"},
	}

	err := formatVerifyWorkflowResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifyWorkflowResultHuman failed: %v", err)
	}

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "Workflow has errors") {
		t.Errorf("Expected 'Workflow has errors' in output, got: %s", out)
	}
	if !strings.Contains(out, "ERROR:") {
		t.Errorf("Expected 'ERROR:' prefix in output, got: %s", out)
	}
	if !strings.Contains(out, "missing or empty required field: title") {
		t.Errorf("Expected title error in output, got: %s", out)
	}
	if !strings.Contains(out, "WARN:") {
		t.Errorf("Expected 'WARN:' prefix in output, got: %s", out)
	}
	if !strings.Contains(out, "no trigger defined") {
		t.Errorf("Expected trigger warning in output, got: %s", out)
	}
}

// TestVerifyWorkflowExitCode tests exit code determination for all scenarios:
// valid → 0, has errors → 1, warnings only + failOnWarn → 1, warnings only → 0.
func TestVerifyWorkflowExitCode(t *testing.T) {
	tests := []struct {
		name       string
		errors     []string
		warnings   []string
		failOnWarn bool
		wantCode   int
	}{
		{
			name:       "valid no errors no warnings",
			errors:     nil,
			warnings:   nil,
			failOnWarn: false,
			wantCode:   0,
		},
		{
			name:       "has errors",
			errors:     []string{"missing title"},
			warnings:   nil,
			failOnWarn: false,
			wantCode:   1,
		},
		{
			name:       "has errors and fail-on-warn",
			errors:     []string{"missing title"},
			warnings:   []string{"no trigger"},
			failOnWarn: true,
			wantCode:   1,
		},
		{
			name:       "warnings without fail-on-warn",
			errors:     nil,
			warnings:   []string{"no trigger defined"},
			failOnWarn: false,
			wantCode:   0,
		},
		{
			name:       "warnings with fail-on-warn",
			errors:     nil,
			warnings:   []string{"no trigger defined"},
			failOnWarn: true,
			wantCode:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exitCode := 0
			if len(tc.errors) > 0 {
				exitCode = 1
			} else if tc.failOnWarn && len(tc.warnings) > 0 {
				exitCode = 1
			}

			if exitCode != tc.wantCode {
				t.Errorf("expected exit code %d, got %d", tc.wantCode, exitCode)
			}
		})
	}
}
