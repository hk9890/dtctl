package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestValidateNotebookStructure_Valid tests a valid notebook with sections in wrapped content format
func TestValidateNotebookStructure_Valid(t *testing.T) {
	doc := map[string]interface{}{
		"name": "My Notebook",
		"type": "notebook",
		"content": map[string]interface{}{
			"sections": []interface{}{
				map[string]interface{}{"type": "markdown"},
				map[string]interface{}{"type": "dql"},
			},
		},
	}

	errs, warnings, name, sectionCount := validateNotebookStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings, got: %v", warnings)
	}
	if name != "My Notebook" {
		t.Errorf("Expected name 'My Notebook', got: %q", name)
	}
	if sectionCount != 2 {
		t.Errorf("Expected sectionCount 2, got: %d", sectionCount)
	}
}

// TestValidateNotebookStructure_DirectFormat tests sections at root level (direct content format)
func TestValidateNotebookStructure_DirectFormat(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Direct Notebook",
		"sections": []interface{}{
			map[string]interface{}{"type": "markdown"},
			map[string]interface{}{"type": "dql"},
			map[string]interface{}{"type": "markdown"},
		},
	}

	errs, warnings, name, sectionCount := validateNotebookStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings, got: %v", warnings)
	}
	if name != "Direct Notebook" {
		t.Errorf("Expected name 'Direct Notebook', got: %q", name)
	}
	if sectionCount != 3 {
		t.Errorf("Expected sectionCount 3, got: %d", sectionCount)
	}
}

// TestValidateNotebookStructure_WrappedFormat tests sections in content field (wrapped format)
func TestValidateNotebookStructure_WrappedFormat(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Wrapped Notebook",
		"content": map[string]interface{}{
			"sections": []interface{}{
				map[string]interface{}{"type": "markdown"},
			},
		},
	}

	errs, warnings, name, sectionCount := validateNotebookStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings, got: %v", warnings)
	}
	if name != "Wrapped Notebook" {
		t.Errorf("Expected name 'Wrapped Notebook', got: %q", name)
	}
	if sectionCount != 1 {
		t.Errorf("Expected sectionCount 1, got: %d", sectionCount)
	}
}

// TestValidateNotebookStructure_MissingSections tests warning when content has no sections field
func TestValidateNotebookStructure_MissingSections(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Empty Notebook",
		"type": "notebook",
		"content": map[string]interface{}{
			"someOtherField": "value",
		},
	}

	errs, warnings, _, sectionCount := validateNotebookStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	if len(warnings) != 1 {
		t.Errorf("Expected 1 warning, got: %v", warnings)
	}
	if !strings.Contains(warnings[0], "sections") {
		t.Errorf("Expected warning about missing sections, got: %s", warnings[0])
	}
	if sectionCount != 0 {
		t.Errorf("Expected sectionCount 0, got: %d", sectionCount)
	}
}

// TestValidateNotebookStructure_DoubleNested tests warning about double-nested content
func TestValidateNotebookStructure_DoubleNested(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Double Nested Notebook",
		"content": map[string]interface{}{
			"content": map[string]interface{}{
				"sections": []interface{}{
					map[string]interface{}{"type": "markdown"},
					map[string]interface{}{"type": "dql"},
				},
			},
		},
	}

	errs, warnings, _, sectionCount := validateNotebookStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	if len(warnings) != 1 {
		t.Errorf("Expected 1 warning about double-nesting, got: %v", warnings)
	}
	if !strings.Contains(warnings[0], "double-nested") {
		t.Errorf("Expected warning about double-nested content, got: %s", warnings[0])
	}
	if sectionCount != 2 {
		t.Errorf("Expected sectionCount 2 from inner content, got: %d", sectionCount)
	}
}

// TestValidateNotebookStructure_WrongType tests error for wrong type field
func TestValidateNotebookStructure_WrongType(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Wrong Type",
		"type": "dashboard",
		"content": map[string]interface{}{
			"sections": []interface{}{},
		},
	}

	errs, _, _, _ := validateNotebookStructure(doc)

	if len(errs) != 1 {
		t.Errorf("Expected 1 error for wrong type, got: %v", errs)
	}
	if !strings.Contains(errs[0], "dashboard") || !strings.Contains(errs[0], "notebook") {
		t.Errorf("Expected error mentioning 'dashboard' and 'notebook', got: %s", errs[0])
	}
}

// TestValidateNotebookStructure_ContentNotObject tests error when content is not an object
func TestValidateNotebookStructure_ContentNotObject(t *testing.T) {
	doc := map[string]interface{}{
		"name":    "Bad Content",
		"content": "this is a string, not an object",
	}

	errs, _, _, _ := validateNotebookStructure(doc)

	if len(errs) != 1 {
		t.Errorf("Expected 1 error for non-object content, got: %v", errs)
	}
	if !strings.Contains(errs[0], "content field must be an object") {
		t.Errorf("Expected error about content field, got: %s", errs[0])
	}
}

// TestValidateNotebookStructure_NoContentNoSections tests warning when neither content nor sections exists
func TestValidateNotebookStructure_NoContentNoSections(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Incomplete Notebook",
		"type": "notebook",
	}

	errs, warnings, _, sectionCount := validateNotebookStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	if len(warnings) != 1 {
		t.Errorf("Expected 1 warning about missing structure, got: %v", warnings)
	}
	if !strings.Contains(warnings[0], "no 'content' or 'sections' field") {
		t.Errorf("Expected warning about missing content/sections, got: %s", warnings[0])
	}
	if sectionCount != 0 {
		t.Errorf("Expected sectionCount 0, got: %d", sectionCount)
	}
}

// TestFormatVerifyNotebookResultHuman_Valid tests human output for a valid notebook
func TestFormatVerifyNotebookResultHuman_Valid(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifyNotebookResult{
		Valid:        true,
		Name:         "My Notebook",
		SectionCount: 5,
	}

	err := formatVerifyNotebookResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifyNotebookResultHuman failed: %v", err)
	}

	// Close writer and restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "✔ Notebook is valid") {
		t.Errorf("Expected '✔ Notebook is valid' in output, got: %s", out)
	}
	if !strings.Contains(out, "My Notebook") {
		t.Errorf("Expected notebook name in output, got: %s", out)
	}
	if !strings.Contains(out, "Sections: 5") {
		t.Errorf("Expected 'Sections: 5' in output, got: %s", out)
	}
}

// TestFormatVerifyNotebookResultHuman_WithErrors tests human output when there are errors and warnings
func TestFormatVerifyNotebookResultHuman_WithErrors(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifyNotebookResult{
		Valid:        false,
		SectionCount: 0,
		Errors:       []string{`type field is "dashboard", expected "notebook"`},
		Warnings:     []string{"notebook content has no 'sections' field - notebook may be empty"},
	}

	err := formatVerifyNotebookResultHuman(result)
	if err != nil {
		t.Fatalf("formatVerifyNotebookResultHuman failed: %v", err)
	}

	// Close writer and restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "✖ Notebook has errors") {
		t.Errorf("Expected '✖ Notebook has errors' in output, got: %s", out)
	}
	if !strings.Contains(out, "ERROR:") {
		t.Errorf("Expected 'ERROR:' prefix in output, got: %s", out)
	}
	if !strings.Contains(out, "dashboard") {
		t.Errorf("Expected error message with 'dashboard' in output, got: %s", out)
	}
	if !strings.Contains(out, "WARN:") {
		t.Errorf("Expected 'WARN:' prefix in output, got: %s", out)
	}
	if !strings.Contains(out, "sections") {
		t.Errorf("Expected warning message with 'sections' in output, got: %s", out)
	}
}
