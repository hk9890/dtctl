package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/dtctl/pkg/exec"
)

func TestIsSupportedQueryOutputFormat(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   bool
	}{
		{name: "default", format: "", want: true},
		{name: "json", format: "json", want: true},
		{name: "yaml alias", format: "yml", want: true},
		{name: "chart", format: "chart", want: true},
		{name: "spark alias", format: "spark", want: true},
		{name: "bar alias", format: "bar", want: true},
		{name: "braille alias", format: "br", want: true},
		{name: "toon", format: "toon", want: true},
		{name: "trimmed and mixed case", format: " Json ", want: true},
		{name: "unsupported", format: "xml", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSupportedQueryOutputFormat(tt.format); got != tt.want {
				t.Fatalf("isSupportedQueryOutputFormat(%q) = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}

func TestParseSegmentFlags(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []exec.FilterSegmentRef
		wantErr bool
	}{
		{
			name:  "single segment",
			input: []string{"seg-uid-1"},
			want:  []exec.FilterSegmentRef{{ID: "seg-uid-1"}},
		},
		{
			name:  "multiple segments",
			input: []string{"seg-uid-1", "seg-uid-2", "seg-uid-3"},
			want: []exec.FilterSegmentRef{
				{ID: "seg-uid-1"},
				{ID: "seg-uid-2"},
				{ID: "seg-uid-3"},
			},
		},
		{
			name:  "trims whitespace",
			input: []string{"  seg-uid-1  "},
			want:  []exec.FilterSegmentRef{{ID: "seg-uid-1"}},
		},
		{
			name:    "empty string rejected",
			input:   []string{""},
			wantErr: true,
		},
		{
			name:    "whitespace-only rejected",
			input:   []string{"  "},
			wantErr: true,
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSegmentFlags(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSegmentFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseSegmentFlags() got %d refs, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].ID != tt.want[i].ID {
					t.Errorf("ref[%d].ID = %q, want %q", i, got[i].ID, tt.want[i].ID)
				}
			}
		})
	}
}

func TestParseSegmentsFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []exec.FilterSegmentRef
		wantErr bool
	}{
		{
			name:    "simple segment list",
			content: "- id: seg-1\n- id: seg-2\n",
			want: []exec.FilterSegmentRef{
				{ID: "seg-1"},
				{ID: "seg-2"},
			},
		},
		{
			name: "segment with variables",
			content: `- id: seg-1
  variables:
    - name: host
      values:
        - HOST-001
        - HOST-002
- id: seg-2
`,
			want: []exec.FilterSegmentRef{
				{
					ID: "seg-1",
					Variables: []exec.FilterSegmentVariable{
						{Name: "host", Values: []string{"HOST-001", "HOST-002"}},
					},
				},
				{ID: "seg-2"},
			},
		},
		{
			name:    "missing id field",
			content: "- variables:\n    - name: x\n      values: [a]\n",
			wantErr: true,
		},
		{
			name:    "invalid YAML",
			content: "not: a: valid: yaml: [[[",
			wantErr: true,
		},
		{
			name:    "empty file",
			content: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "segments.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			got, err := parseSegmentsFile(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSegmentsFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseSegmentsFile() got %d refs, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].ID != tt.want[i].ID {
					t.Errorf("ref[%d].ID = %q, want %q", i, got[i].ID, tt.want[i].ID)
				}
				if len(got[i].Variables) != len(tt.want[i].Variables) {
					t.Errorf("ref[%d] has %d variables, want %d", i, len(got[i].Variables), len(tt.want[i].Variables))
					continue
				}
				for j := range got[i].Variables {
					if got[i].Variables[j].Name != tt.want[i].Variables[j].Name {
						t.Errorf("ref[%d].Variables[%d].Name = %q, want %q", i, j, got[i].Variables[j].Name, tt.want[i].Variables[j].Name)
					}
					if len(got[i].Variables[j].Values) != len(tt.want[i].Variables[j].Values) {
						t.Errorf("ref[%d].Variables[%d] has %d values, want %d", i, j, len(got[i].Variables[j].Values), len(tt.want[i].Variables[j].Values))
					}
				}
			}
		})
	}
}

func TestParseSegmentsFile_NotFound(t *testing.T) {
	_, err := parseSegmentsFile("/nonexistent/path/segments.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestMergeSegmentRefs(t *testing.T) {
	tests := []struct {
		name     string
		flagRefs []exec.FilterSegmentRef
		fileRefs []exec.FilterSegmentRef
		wantIDs  []string
	}{
		{
			name:     "flags only",
			flagRefs: []exec.FilterSegmentRef{{ID: "a"}, {ID: "b"}},
			fileRefs: nil,
			wantIDs:  []string{"a", "b"},
		},
		{
			name:     "file only",
			flagRefs: nil,
			fileRefs: []exec.FilterSegmentRef{{ID: "x"}, {ID: "y"}},
			wantIDs:  []string{"x", "y"},
		},
		{
			name:     "file wins on conflict",
			flagRefs: []exec.FilterSegmentRef{{ID: "a"}, {ID: "b"}},
			fileRefs: []exec.FilterSegmentRef{{ID: "b", Variables: []exec.FilterSegmentVariable{{Name: "v", Values: []string{"1"}}}}},
			wantIDs:  []string{"b", "a"},
		},
		{
			name:     "deduplicates by ID",
			flagRefs: []exec.FilterSegmentRef{{ID: "a"}, {ID: "a"}},
			fileRefs: nil,
			wantIDs:  []string{"a"},
		},
		{
			name:     "both empty",
			flagRefs: nil,
			fileRefs: nil,
			wantIDs:  nil,
		},
		{
			name:     "preserves file order then flag order",
			flagRefs: []exec.FilterSegmentRef{{ID: "c"}, {ID: "d"}},
			fileRefs: []exec.FilterSegmentRef{{ID: "a"}, {ID: "b"}},
			wantIDs:  []string{"a", "b", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeSegmentRefs(tt.flagRefs, tt.fileRefs)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("mergeSegmentRefs() got %d refs, want %d", len(got), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("merged[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestMergeSegmentRefs_FileWinsWithVariables(t *testing.T) {
	flagRefs := []exec.FilterSegmentRef{{ID: "seg-1"}}
	fileRefs := []exec.FilterSegmentRef{
		{ID: "seg-1", Variables: []exec.FilterSegmentVariable{
			{Name: "host", Values: []string{"HOST-001"}},
		}},
	}

	got := mergeSegmentRefs(flagRefs, fileRefs)
	if len(got) != 1 {
		t.Fatalf("expected 1 merged ref, got %d", len(got))
	}
	if len(got[0].Variables) != 1 {
		t.Fatalf("expected file entry to win with 1 variable, got %d", len(got[0].Variables))
	}
	if got[0].Variables[0].Name != "host" {
		t.Errorf("variable name = %q, want %q", got[0].Variables[0].Name, "host")
	}
}
