package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// --- validateDashboardStructure tests ---

func TestValidateDashboardStructure_Valid(t *testing.T) {
	// Wrapped format with tiles array and version
	doc := map[string]interface{}{
		"name": "My Dashboard",
		"type": "dashboard",
		"content": map[string]interface{}{
			"version": 5,
			"tiles": []interface{}{
				map[string]interface{}{"id": "tile-1"},
				map[string]interface{}{"id": "tile-2"},
			},
		},
	}

	errs, warnings, name, tileCount := validateDashboardStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings, got: %v", warnings)
	}
	if name != "My Dashboard" {
		t.Errorf("Expected name 'My Dashboard', got: %q", name)
	}
	if tileCount != 2 {
		t.Errorf("Expected tileCount 2, got: %d", tileCount)
	}
}

func TestValidateDashboardStructure_DirectFormat(t *testing.T) {
	// Tiles at root level (direct content format)
	doc := map[string]interface{}{
		"name":    "Direct Dashboard",
		"version": 3,
		"tiles": []interface{}{
			map[string]interface{}{"id": "tile-1"},
			map[string]interface{}{"id": "tile-2"},
			map[string]interface{}{"id": "tile-3"},
		},
	}

	errs, warnings, name, tileCount := validateDashboardStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings, got: %v", warnings)
	}
	if name != "Direct Dashboard" {
		t.Errorf("Expected name 'Direct Dashboard', got: %q", name)
	}
	if tileCount != 3 {
		t.Errorf("Expected tileCount 3, got: %d", tileCount)
	}
}

func TestValidateDashboardStructure_WrappedFormat(t *testing.T) {
	// Wrapped format: content.tiles
	doc := map[string]interface{}{
		"name": "Wrapped Dashboard",
		"content": map[string]interface{}{
			"version": 1,
			"tiles": []interface{}{
				map[string]interface{}{"id": "t1"},
			},
		},
	}

	errs, warnings, name, tileCount := validateDashboardStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings, got: %v", warnings)
	}
	if name != "Wrapped Dashboard" {
		t.Errorf("Expected name 'Wrapped Dashboard', got: %q", name)
	}
	if tileCount != 1 {
		t.Errorf("Expected tileCount 1, got: %d", tileCount)
	}
}

func TestValidateDashboardStructure_MissingTiles(t *testing.T) {
	doc := map[string]interface{}{
		"name": "No Tiles",
		"content": map[string]interface{}{
			"version": 1,
			// no tiles field
		},
	}

	errs, warnings, _, _ := validateDashboardStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "tiles") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about missing 'tiles' field, got: %v", warnings)
	}
}

func TestValidateDashboardStructure_MissingVersion(t *testing.T) {
	doc := map[string]interface{}{
		"name": "No Version",
		"content": map[string]interface{}{
			// no version field
			"tiles": []interface{}{
				map[string]interface{}{"id": "t1"},
			},
		},
	}

	errs, warnings, _, _ := validateDashboardStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "version") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about missing 'version' field, got: %v", warnings)
	}
}

func TestValidateDashboardStructure_DoubleNested(t *testing.T) {
	// Double-nested content: .content.content.tiles
	doc := map[string]interface{}{
		"name": "Double Nested",
		"content": map[string]interface{}{
			"content": map[string]interface{}{
				"version": 2,
				"tiles": []interface{}{
					map[string]interface{}{"id": "t1"},
					map[string]interface{}{"id": "t2"},
				},
			},
		},
	}

	errs, warnings, _, tileCount := validateDashboardStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "double-nested") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about double-nested content, got: %v", warnings)
	}
	// Should still count tiles from the inner content
	if tileCount != 2 {
		t.Errorf("Expected tileCount 2 from inner content, got: %d", tileCount)
	}
}

func TestValidateDashboardStructure_WrongType(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Not a Dashboard",
		"type": "notebook",
		"content": map[string]interface{}{
			"version": 1,
			"tiles":   []interface{}{},
		},
	}

	errs, _, _, _ := validateDashboardStructure(doc)

	if len(errs) == 0 {
		t.Errorf("Expected error for wrong type field, got none")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "notebook") && strings.Contains(e, "dashboard") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error mentioning wrong type, got: %v", errs)
	}
}

func TestValidateDashboardStructure_ContentNotObject(t *testing.T) {
	doc := map[string]interface{}{
		"name":    "Bad Content",
		"content": "this is not an object",
	}

	errs, _, _, _ := validateDashboardStructure(doc)

	if len(errs) == 0 {
		t.Errorf("Expected error when content is not an object, got none")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "content field must be an object") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'content field must be an object' error, got: %v", errs)
	}
}

func TestValidateDashboardStructure_NoContentOrTiles(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Empty Dashboard",
	}

	errs, warnings, _, _ := validateDashboardStructure(doc)

	if len(errs) != 0 {
		t.Errorf("Expected no errors, got: %v", errs)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "no 'content' or 'tiles' field") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about missing content/tiles, got: %v", warnings)
	}
}

// --- formatVerifyDashboardResultHuman tests ---

func TestFormatVerifyDashboardResultHuman_Valid(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifyDashboardResult{
		Valid:     true,
		Name:      "My Dashboard",
		TileCount: 12,
	}

	err := formatVerifyDashboardResultHuman(result)

	// Close writer and restore stderr before asserting
	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("formatVerifyDashboardResultHuman failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "✔ Dashboard is valid") {
		t.Errorf("Expected '✔ Dashboard is valid' in output, got: %s", got)
	}
	if !strings.Contains(got, `"My Dashboard"`) {
		t.Errorf("Expected dashboard name in output, got: %s", got)
	}
	if !strings.Contains(got, "Tiles: 12") {
		t.Errorf("Expected 'Tiles: 12' in output, got: %s", got)
	}
}

func TestFormatVerifyDashboardResultHuman_WithErrors(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifyDashboardResult{
		Valid:     false,
		TileCount: 0,
		Errors:    []string{`type field is "notebook", expected "dashboard"`},
		Warnings:  []string{"dashboard content has no 'version' field"},
	}

	err := formatVerifyDashboardResultHuman(result)

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("formatVerifyDashboardResultHuman failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "✖ Dashboard has errors") {
		t.Errorf("Expected '✖ Dashboard has errors' in output, got: %s", got)
	}
	if !strings.Contains(got, "ERROR:") {
		t.Errorf("Expected 'ERROR:' prefix in output, got: %s", got)
	}
	if !strings.Contains(got, "notebook") {
		t.Errorf("Expected error text in output, got: %s", got)
	}
	if !strings.Contains(got, "WARN:") {
		t.Errorf("Expected 'WARN:' prefix in output, got: %s", got)
	}
}

func TestFormatVerifyDashboardResultHuman_NoName(t *testing.T) {
	// When no name is present, the Name line should not appear
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	result := VerifyDashboardResult{
		Valid:     true,
		Name:      "",
		TileCount: 5,
	}

	err := formatVerifyDashboardResultHuman(result)

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("formatVerifyDashboardResultHuman failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	if strings.Contains(got, "Name:") {
		t.Errorf("Did not expect 'Name:' line when name is empty, got: %s", got)
	}
	if !strings.Contains(got, "Tiles: 5") {
		t.Errorf("Expected 'Tiles: 5' in output, got: %s", got)
	}
}
