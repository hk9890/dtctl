package output

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

type DescribeTestResource struct {
	Name        string `table:"NAME"`
	ID          string `table:"ID"`
	Status      string `table:"STATUS"`
	Description string `table:"DESCRIPTION,wide"`
}

func TestDescribePrinter_Print_Struct(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	p := &DescribePrinter{writer: &buf, wide: false}

	obj := DescribeTestResource{
		Name:        "my-workflow",
		ID:          "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
		Status:      "active",
		Description: "A workflow for testing",
	}

	err := p.Print(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()

	// Should contain key-value pairs
	if !strings.Contains(got, "Name") {
		t.Error("expected 'Name' label")
	}
	if !strings.Contains(got, "my-workflow") {
		t.Error("expected 'my-workflow' value")
	}
	if !strings.Contains(got, "Id") {
		t.Error("expected 'Id' label")
	}
	if !strings.Contains(got, "Status") {
		t.Error("expected 'Status' label")
	}
	if !strings.Contains(got, "active") {
		t.Error("expected 'active' value")
	}

	// In normal mode, wide-only fields should be excluded
	if strings.Contains(got, "Description") {
		t.Error("wide-only field should not be shown in normal mode")
	}
}

func TestDescribePrinter_Print_WideMode(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	p := &DescribePrinter{writer: &buf, wide: true}

	obj := DescribeTestResource{
		Name:        "my-workflow",
		ID:          "abc123",
		Status:      "active",
		Description: "A workflow for testing",
	}

	err := p.Print(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()

	// In wide mode, Description should be included
	if !strings.Contains(got, "Description") {
		t.Error("wide-only field should be shown in wide mode")
	}
	if !strings.Contains(got, "A workflow for testing") {
		t.Error("expected description value in wide mode")
	}
}

func TestDescribePrinter_Print_NonStruct(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	p := &DescribePrinter{writer: &buf}

	err := p.Print("just a string")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "just a string") {
		t.Errorf("expected plain string output, got: %s", got)
	}
}

func TestDescribePrinter_Print_Pointer(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	p := &DescribePrinter{writer: &buf}

	obj := &DescribeTestResource{
		Name:   "ptr-workflow",
		ID:     "xyz",
		Status: "inactive",
	}

	err := p.Print(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "ptr-workflow") {
		t.Errorf("expected pointer dereference to work, got: %s", got)
	}
}

func TestDescribePrinter_PrintList_DelegatesToTable(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	p := &DescribePrinter{writer: &buf}

	items := []DescribeTestResource{
		{Name: "wf-1", ID: "1", Status: "active"},
		{Name: "wf-2", ID: "2", Status: "failed"},
	}

	err := p.PrintList(items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()

	// Should produce table output with headers (uppercase from table tags)
	if !strings.Contains(got, "NAME") {
		t.Error("expected table header NAME")
	}
	if !strings.Contains(got, "wf-1") {
		t.Error("expected first item in table")
	}
	if !strings.Contains(got, "wf-2") {
		t.Error("expected second item in table")
	}
}

func TestDescribePrinter_Print_Alignment(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	p := &DescribePrinter{writer: &buf}

	obj := DescribeTestResource{
		Name:   "test",
		ID:     "123",
		Status: "ok",
	}

	err := p.Print(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that labels are padded for alignment
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// All labels should have the same padding width (aligned)
	// The longest label is "Status" (6 chars), so "Name" and "Id" should be padded
	for _, line := range lines {
		// Each line should have "   " separator between label and value
		if !strings.Contains(line, "   ") {
			t.Errorf("expected 3-space separator in line: %s", line)
		}
	}
}

func TestFormatDescribeLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"NAME", "Name"},
		{"DISPLAY NAME", "Display Name"},
		{"ID", "Id"},
		{"STATUS", "Status"},
		{"RETENTION DAYS", "Retention Days"},
		{"", ""},
	}

	for _, tc := range tests {
		got := formatDescribeLabel(tc.input)
		if got != tc.expected {
			t.Errorf("formatDescribeLabel(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

type DescribeComplexResource struct {
	Name   string            `table:"NAME"`
	Tags   map[string]string `table:"TAGS"`
	Items  []string          `table:"ITEMS"`
	Active bool              `table:"ACTIVE"`
	When   time.Time         `table:"WHEN"`
}

func TestFormatDescribeValue_ComplexTypes(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	p := &DescribePrinter{writer: &buf}

	obj := DescribeComplexResource{
		Name:   "complex",
		Tags:   map[string]string{"env": "prod", "team": "sre"},
		Items:  []string{"a", "b", "c"},
		Active: true,
		When:   time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
	}

	err := p.Print(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()

	// Map should show item count
	if !strings.Contains(got, "<2 items>") {
		t.Errorf("expected map to show '<2 items>', got: %s", got)
	}

	// Small string slice should be inline
	if !strings.Contains(got, "a, b, c") {
		t.Errorf("expected small slice to be inline, got: %s", got)
	}

	// Bool should be formatted
	if !strings.Contains(got, "true") {
		t.Errorf("expected bool value, got: %s", got)
	}

	// Time should be formatted
	if !strings.Contains(got, "2025-06-15 10:30:00") {
		t.Errorf("expected formatted time, got: %s", got)
	}
}

func TestFormatDescribeValue_NilAndEmpty(t *testing.T) {
	ResetColorCache()
	t.Setenv("NO_COLOR", "1")
	defer ResetColorCache()

	var buf bytes.Buffer
	p := &DescribePrinter{writer: &buf}

	obj := DescribeComplexResource{
		Name: "empty-test",
		// Tags, Items left nil; When left zero
	}

	err := p.Print(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()

	// Nil map and nil slice and zero time should produce empty values
	if !strings.Contains(got, "empty-test") {
		t.Errorf("expected name value, got: %s", got)
	}
}
