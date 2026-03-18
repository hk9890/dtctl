# Output Formatting Improvements

Improvements to dtctl's terminal output readability, guided by [clig.dev](https://clig.dev/) best practices.

## Design Principles

From clig.dev:
- **Human-first design**: color and formatting help humans scan output faster
- **Use color with intention**: highlight what matters, don't paint everything
- **Disable color gracefully**: respect `NO_COLOR`, `--plain`, non-TTY
- **Saying just enough**: don't overwhelm, but don't leave users guessing
- **Suggest next commands**: help users discover what to do next

All changes respect the existing color control system (`NO_COLOR`, `FORCE_COLOR`, `--plain`, TTY detection). Machine-readable formats (JSON, YAML, CSV) and agent mode are never affected.

## Changes

### 1. Bold Table Headers

**Problem:** Table headers (`ID`, `NAME`, `TITLE`) are visually indistinct from data rows.

**Change:** Render table headers in **bold** when color is enabled.

**Files:** `pkg/output/table.go`

### 2. Status-Aware Value Coloring

**Problem:** Status values like `SUCCEEDED`, `FAILED`, `true`, `false` are plain text with no visual distinction.

**Change:** Color values based on semantic meaning:
- **Green:** `true`, `active`, `SUCCEEDED`, `SUCCESS`, `healthy`, `enabled`
- **Red:** `false`, `FAILED`, `ERROR`, `disabled`, `inactive`
- **Yellow:** `WARNING`, `PENDING`, `RUNNING` (in-progress states)

Applied only to table cell values, only when color is enabled.

**Files:** `pkg/output/table.go`

### 3. Dim UUIDs

**Problem:** UUIDs dominate table visual space but are rarely what users scan for.

**Change:** Render UUID-format values in **dim** text. The information is still present but the eye is drawn to human-readable columns.

**Files:** `pkg/output/table.go`

### 4. Colored Error Output

**Problem:** Errors display as plain `Error: message` with no visual weight.

**Change:**
- Render the `Error:` prefix in **bold red**
- Keep the message in normal text for readability

**Files:** `cmd/root.go`

### 5. Improved Empty State Messages

**Problem:** Empty results show a generic `No resources found.` with no guidance.

**Change:** Display `No resources found.` in **dim** text to indicate it's an informational state rather than an error.

**Files:** `pkg/output/table.go`

### 6. Describe Key-Value Layout

**Problem:** `describe` renders identically to `get` -- a single-row horizontal table. This makes detailed resource inspection hard to read.

**Change:** Add a `DescribePrinter` that renders single objects as vertical key-value pairs:
```
Name:         Deploy to Production
ID:           a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d
Deployed:     true
Owner:        7a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d
Description:  Deploys latest build to prod environment
```

Keys are **bold**, values are normal. Sub-objects render as indented YAML. The `PrintList` method delegates to the existing table printer for list output.

**Files:** `pkg/output/describe.go` (new), `pkg/output/printer.go`

### 7. Mutation Confirmation Messages

**Problem:** Mutation commands (create, delete, edit) use inconsistent `fmt.Printf` patterns for success messages -- different formats, no color, no structure.

**Change:** Add `PrintSuccess` and `PrintWarning` helper functions in `pkg/output/` that format messages consistently:
- Success: green checkmark prefix
- Warning: yellow warning prefix

**Files:** `pkg/output/messages.go` (new)

## Test Strategy

- Golden tests in `pkg/output/` use `stripANSI()` before comparison, so color changes do not break existing golden files
- New golden tests added for describe key-value format
- Run `make test-update-golden` after changes, review diffs

## Files Changed

| File | Change |
|------|--------|
| `pkg/output/table.go` | Bold headers, status coloring, dim UUIDs, dim empty state |
| `pkg/output/describe.go` | New describe key-value printer |
| `pkg/output/messages.go` | New `PrintSuccess`/`PrintWarning` helpers |
| `pkg/output/printer.go` | Add `"describe"` format routing |
| `cmd/root.go` | Colored error prefix |
| `pkg/output/golden_test.go` | New describe golden tests |
| Golden files | Updated via `-update` flag |
