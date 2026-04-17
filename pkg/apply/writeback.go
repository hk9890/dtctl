package apply

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// writeIDToFile injects the given id into the top of a YAML or JSON source file
// so that subsequent applies update in place instead of creating duplicates.
//
// For YAML files: inserts "id: <id>" as the first non-comment, non-blank line.
// For JSON files: inserts "id" as the first key inside the opening brace.
//
// If the file already contains an id field the function is a no-op (returns nil).
// Errors writing the file are returned but do NOT affect the already-completed apply.
func writeIDToFile(filename, id string) error {
	if filename == "" {
		return fmt.Errorf("no source file to write ID back to")
	}

	original, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}

	updated, err := injectIDIntoFileContent(original, id)
	if err != nil {
		return err
	}
	if updated == nil {
		// already has an id — nothing to do
		return nil
	}

	return os.WriteFile(filename, updated, 0o644)
}

// injectIDIntoFileContent returns the file content with the id field injected,
// or nil if the file already contains an id field (no-op).
func injectIDIntoFileContent(content []byte, id string) ([]byte, error) {
	trimmed := bytes.TrimSpace(content)

	if isJSONContent(trimmed) {
		return injectIDIntoJSON(content, id)
	}
	return injectIDIntoYAML(content, id)
}

func isJSONContent(content []byte) bool {
	return len(content) > 0 && content[0] == '{'
}

// injectIDIntoJSON inserts "id": "<id>" as the first key in a JSON object.
// Returns nil if the file already has an "id" key.
func injectIDIntoJSON(content []byte, id string) ([]byte, error) {
	// Quick check: if "id" key already present, no-op
	if jsonHasIDKey(content) {
		return nil, nil
	}

	// Find the opening brace
	idx := bytes.IndexByte(content, '{')
	if idx < 0 {
		return nil, fmt.Errorf("no opening brace found in JSON file")
	}

	idEntry := fmt.Sprintf(`"id": %q`, id)

	// Peek at what follows the brace to decide formatting
	rest := bytes.TrimLeft(content[idx+1:], " \t\r\n")
	var insertion string
	if len(rest) == 0 || rest[0] == '}' {
		// Empty object
		insertion = idEntry + "\n"
	} else {
		// Non-empty — detect indentation of first real key
		indent := detectJSONIndent(content[idx+1:])
		insertion = "\n" + indent + idEntry + ","
	}

	var buf bytes.Buffer
	buf.Write(content[:idx+1])
	buf.WriteString(insertion)
	buf.Write(content[idx+1:])
	return buf.Bytes(), nil
}

// injectIDIntoYAML inserts "id: <id>" as the first non-comment, non-blank line.
// Returns nil if the file already has an "id:" key.
func injectIDIntoYAML(content []byte, id string) ([]byte, error) {
	lines := strings.Split(string(content), "\n")

	// Check for existing id key (simple scan — no full YAML parse needed)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "id:") || strings.HasPrefix(trimmed, "id :") {
			return nil, nil // already has id
		}
	}

	// Find insertion point: first non-comment, non-blank line
	insertAt := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			insertAt = i
			break
		}
	}

	idLine := fmt.Sprintf("id: %q", id)
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertAt]...)
	newLines = append(newLines, idLine)
	newLines = append(newLines, lines[insertAt:]...)

	return []byte(strings.Join(newLines, "\n")), nil
}

// jsonHasIDKey does a fast (non-parsing) check for a top-level "id" key.
func jsonHasIDKey(content []byte) bool {
	return bytes.Contains(content, []byte(`"id"`)) ||
		bytes.Contains(content, []byte(`"id" `)) ||
		bytes.Contains(content, []byte(`"id":"`))
}

// detectJSONIndent returns the leading whitespace of the first real key after '{'.
func detectJSONIndent(afterBrace []byte) string {
	lines := bytes.Split(afterBrace, []byte("\n"))
	for _, line := range lines {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(trimmed) > 0 {
			return string(line[:len(line)-len(trimmed)])
		}
	}
	return "  " // fallback: 2 spaces
}

// printWriteIDHint prints a stderr hint when a resource was created without --write-id.
// It suggests the exact command to recover without creating another duplicate.
func printWriteIDHint(sourceFile, resourceID, resourceType string) {
	if sourceFile == "" {
		return
	}
	fmt.Fprintf(os.Stderr,
		"Hint: to update this %s in future runs without creating duplicates:\n"+
			"  dtctl apply -f %s --write-id --id %s\n",
		resourceType, sourceFile, resourceID,
	)
}
