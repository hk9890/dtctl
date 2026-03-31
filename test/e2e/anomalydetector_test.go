//go:build integration
// +build integration

package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtctl/pkg/resources/anomalydetector"
	"github.com/dynatrace-oss/dtctl/test/integration"
)

func TestAnomalyDetectorLifecycle(t *testing.T) {
	env := integration.SetupIntegration(t)
	defer env.Cleanup.Cleanup(t)

	handler := anomalydetector.NewHandler(env.Client)

	t.Run("complete anomaly detector lifecycle", func(t *testing.T) {
		// Step 1: Create anomaly detector (flattened format)
		t.Log("Step 1: Creating anomaly detector...")
		createData := integration.AnomalyDetectorFixture(env.TestPrefix)

		created, err := handler.Create(createData)
		if err != nil {
			t.Fatalf("Failed to create anomaly detector: %v", err)
		}
		if created.ObjectID == "" {
			t.Fatal("Created anomaly detector has no ObjectID")
		}
		t.Logf("Created anomaly detector: %s (ObjectID: %s)", created.Title, created.ObjectID)

		// Track for cleanup
		env.Cleanup.Track("anomalydetector", created.ObjectID, created.Title)

		// Step 2: Get anomaly detector
		t.Log("Step 2: Getting anomaly detector...")
		retrieved, err := handler.Get(created.ObjectID)
		if err != nil {
			t.Fatalf("Failed to get anomaly detector: %v", err)
		}
		if retrieved.ObjectID != created.ObjectID {
			t.Errorf("Retrieved ObjectID mismatch: got %s, want %s", retrieved.ObjectID, created.ObjectID)
		}
		expectedTitle := env.TestPrefix + "-anomaly-detector"
		if retrieved.Title != expectedTitle {
			t.Errorf("Retrieved title mismatch: got %s, want %s", retrieved.Title, expectedTitle)
		}
		if !retrieved.Enabled {
			t.Error("Retrieved anomaly detector should be enabled")
		}
		if retrieved.Source != "dtctl" {
			t.Errorf("Retrieved source mismatch: got %s, want dtctl", retrieved.Source)
		}
		t.Logf("Retrieved anomaly detector: %s (enabled: %v, analyzer: %s, eventType: %s)",
			retrieved.Title, retrieved.Enabled, retrieved.AnalyzerShort, retrieved.EventType)

		// Step 3: List anomaly detectors (verify our detector appears)
		t.Log("Step 3: Listing anomaly detectors...")
		detectors, err := handler.List(anomalydetector.ListOptions{})
		if err != nil {
			t.Fatalf("Failed to list anomaly detectors: %v", err)
		}
		found := false
		for _, d := range detectors {
			if d.ObjectID == created.ObjectID {
				found = true
				break
			}
		}
		if !found {
			t.Error("Created anomaly detector not found in list")
		} else {
			t.Logf("Found anomaly detector in list (total: %d detectors)", len(detectors))
		}

		// Step 4: List with enabled filter
		t.Log("Step 4: Listing with enabled filter...")
		enabledTrue := true
		enabledDetectors, err := handler.List(anomalydetector.ListOptions{Enabled: &enabledTrue})
		if err != nil {
			t.Fatalf("Failed to list enabled anomaly detectors: %v", err)
		}
		foundEnabled := false
		for _, d := range enabledDetectors {
			if d.ObjectID == created.ObjectID {
				foundEnabled = true
			}
			if !d.Enabled {
				t.Errorf("Enabled filter returned disabled detector: %s", d.Title)
			}
		}
		if !foundEnabled {
			t.Error("Created (enabled) detector not found in enabled-only list")
		}
		t.Logf("Enabled filter returned %d detectors", len(enabledDetectors))

		// Step 5: FindByName
		t.Log("Step 5: Finding by name...")
		foundByName, err := handler.FindByName(expectedTitle)
		if err != nil {
			t.Fatalf("Failed to find anomaly detector by name: %v", err)
		}
		if foundByName.ObjectID != created.ObjectID {
			t.Errorf("FindByName ObjectID mismatch: got %s, want %s", foundByName.ObjectID, created.ObjectID)
		}
		t.Logf("Found by name: %s (ObjectID: %s)", foundByName.Title, foundByName.ObjectID)

		// Step 6: GetRaw (for edit command flow)
		t.Log("Step 6: Getting raw anomaly detector...")
		raw, err := handler.GetRaw(created.ObjectID)
		if err != nil {
			t.Fatalf("Failed to get raw anomaly detector: %v", err)
		}
		if len(raw) == 0 {
			t.Error("Raw anomaly detector is empty")
		}
		// Verify raw is valid JSON
		var rawCheck map[string]interface{}
		if err := json.Unmarshal(raw, &rawCheck); err != nil {
			t.Errorf("Raw anomaly detector is not valid JSON: %v", err)
		}
		// Verify raw contains expected fields
		if _, ok := rawCheck["title"]; !ok {
			t.Error("Raw anomaly detector missing 'title' field")
		}
		if _, ok := rawCheck["analyzer"]; !ok {
			t.Error("Raw anomaly detector missing 'analyzer' field")
		}
		t.Logf("Got raw anomaly detector (%d bytes)", len(raw))

		// Step 7: Update anomaly detector
		t.Log("Step 7: Updating anomaly detector...")
		updateData := integration.AnomalyDetectorFixtureModified(env.TestPrefix)
		updated, err := handler.Update(created.ObjectID, updateData)
		if err != nil {
			t.Fatalf("Failed to update anomaly detector: %v", err)
		}
		expectedModifiedTitle := env.TestPrefix + "-anomaly-detector-modified"
		if updated.Title != expectedModifiedTitle {
			t.Errorf("Updated title mismatch: got %s, want %s", updated.Title, expectedModifiedTitle)
		}
		if updated.Enabled {
			t.Error("Updated anomaly detector should be disabled")
		}
		t.Logf("Updated anomaly detector: %s (enabled: %v)", updated.Title, updated.Enabled)

		// Step 8: Verify update via GET
		t.Log("Step 8: Verifying update...")
		verified, err := handler.Get(created.ObjectID)
		if err != nil {
			t.Fatalf("Failed to get updated anomaly detector: %v", err)
		}
		if verified.Title != expectedModifiedTitle {
			t.Errorf("Verified title mismatch: got %s, want %s", verified.Title, expectedModifiedTitle)
		}
		if verified.Enabled {
			t.Error("Verified anomaly detector should be disabled after update")
		}
		t.Logf("Verified update: %s (enabled: %v)", verified.Title, verified.Enabled)

		// Step 9: List with disabled filter (should find our updated detector)
		t.Log("Step 9: Listing with disabled filter...")
		enabledFalse := false
		disabledDetectors, err := handler.List(anomalydetector.ListOptions{Enabled: &enabledFalse})
		if err != nil {
			t.Fatalf("Failed to list disabled anomaly detectors: %v", err)
		}
		foundDisabled := false
		for _, d := range disabledDetectors {
			if d.ObjectID == created.ObjectID {
				foundDisabled = true
			}
			if d.Enabled {
				t.Errorf("Disabled filter returned enabled detector: %s", d.Title)
			}
		}
		if !foundDisabled {
			t.Error("Updated (disabled) detector not found in disabled-only list")
		}
		t.Logf("Disabled filter returned %d detectors", len(disabledDetectors))

		// Step 10: Delete anomaly detector
		t.Log("Step 10: Deleting anomaly detector...")
		err = handler.Delete(created.ObjectID)
		if err != nil {
			t.Fatalf("Failed to delete anomaly detector: %v", err)
		}
		t.Logf("Deleted anomaly detector: %s", created.ObjectID)

		// Untrack from cleanup since we manually deleted
		env.Cleanup.Untrack("anomalydetector", created.ObjectID)

		// Step 11: Verify deletion (should get error/404)
		t.Log("Step 11: Verifying deletion...")
		_, err = handler.Get(created.ObjectID)
		if err == nil {
			t.Error("Expected error when getting deleted anomaly detector, got nil")
		} else {
			t.Logf("Verified deletion (got expected error: %v)", err)
		}
	})
}

func TestAnomalyDetectorCreateInvalid(t *testing.T) {
	env := integration.SetupIntegration(t)
	defer env.Cleanup.Cleanup(t)

	handler := anomalydetector.NewHandler(env.Client)

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "invalid json",
			data:    []byte(`{"invalid": json`),
			wantErr: true,
		},
		{
			name:    "empty object",
			data:    []byte(`{}`),
			wantErr: true,
		},
		{
			name:    "missing analyzer",
			data:    []byte(`{"title": "test-no-analyzer", "eventTemplate": {"event.type": "CUSTOM_ALERT"}}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := handler.Create(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				t.Logf("Got expected error: %v", err)
			}
			// If creation succeeded unexpectedly, clean up
			if err == nil && created != nil {
				env.Cleanup.Track("anomalydetector", created.ObjectID, created.Title)
			}
		})
	}
}

func TestAnomalyDetectorGetNonExistent(t *testing.T) {
	env := integration.SetupIntegration(t)
	defer env.Cleanup.Cleanup(t)

	handler := anomalydetector.NewHandler(env.Client)

	_, err := handler.Get("non-existent-object-id-12345")
	if err == nil {
		t.Error("Expected error when getting non-existent anomaly detector, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestAnomalyDetectorDeleteNonExistent(t *testing.T) {
	env := integration.SetupIntegration(t)
	defer env.Cleanup.Cleanup(t)

	handler := anomalydetector.NewHandler(env.Client)

	err := handler.Delete("non-existent-object-id-12345")
	if err == nil {
		t.Error("Expected error when deleting non-existent anomaly detector, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestAnomalyDetectorFindByNameNotFound(t *testing.T) {
	env := integration.SetupIntegration(t)
	defer env.Cleanup.Cleanup(t)

	handler := anomalydetector.NewHandler(env.Client)

	_, err := handler.FindByName("non-existent-anomaly-detector-name-12345")
	if err == nil {
		t.Error("Expected error when finding non-existent anomaly detector by name, got nil")
	} else {
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}
		t.Logf("Got expected error: %v", err)
	}
}

func TestAnomalyDetectorRawSettingsFormat(t *testing.T) {
	env := integration.SetupIntegration(t)
	defer env.Cleanup.Cleanup(t)

	handler := anomalydetector.NewHandler(env.Client)

	// Create using raw Settings API format
	t.Log("Creating anomaly detector using raw Settings API format...")
	rawData := map[string]interface{}{
		"schemaId": "builtin:davis.anomaly-detectors",
		"scope":    "environment",
		"value": map[string]interface{}{
			"title":       env.TestPrefix + "-raw-format-detector",
			"description": "Integration test - raw Settings format",
			"enabled":     true,
			"source":      "dtctl",
			"analyzer": map[string]interface{}{
				"name": "dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer",
				"input": []map[string]interface{}{
					{"key": "query", "value": "timeseries avg_cpu=avg(dt.host.cpu.usage), interval:5m"},
					{"key": "threshold", "value": "99"},
					{"key": "alertCondition", "value": "ABOVE"},
					{"key": "violatingSamples", "value": "3"},
					{"key": "slidingWindow", "value": "5"},
					{"key": "dealertingSamples", "value": "5"},
					{"key": "alertOnMissingData", "value": "false"},
				},
			},
			"eventTemplate": map[string]interface{}{
				"properties": []map[string]interface{}{
					{"key": "event.type", "value": "CUSTOM_ALERT"},
					{"key": "event.name", "value": env.TestPrefix + " raw format test alert"},
				},
			},
			"executionSettings": map[string]interface{}{
				"queryOffset": 7,
			},
		},
	}
	data, _ := json.Marshal(rawData)

	created, err := handler.Create(data)
	if err != nil {
		t.Fatalf("Failed to create anomaly detector with raw format: %v", err)
	}
	env.Cleanup.Track("anomalydetector", created.ObjectID, created.Title)

	expectedTitle := env.TestPrefix + "-raw-format-detector"
	if created.Title != expectedTitle {
		t.Errorf("Title mismatch: got %s, want %s", created.Title, expectedTitle)
	}
	t.Logf("Created anomaly detector with raw format: %s (ObjectID: %s)", created.Title, created.ObjectID)

	// Verify it can be retrieved
	retrieved, err := handler.Get(created.ObjectID)
	if err != nil {
		t.Fatalf("Failed to get anomaly detector: %v", err)
	}
	if retrieved.Title != expectedTitle {
		t.Errorf("Retrieved title mismatch: got %s, want %s", retrieved.Title, expectedTitle)
	}
	t.Logf("Verified raw format detector: %s (analyzer: %s)", retrieved.Title, retrieved.AnalyzerShort)
}
