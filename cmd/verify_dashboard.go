package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/dynatrace-oss/dtctl/pkg/output"
	"github.com/dynatrace-oss/dtctl/pkg/util/format"
	"github.com/dynatrace-oss/dtctl/pkg/util/template"
	"github.com/spf13/cobra"
)

// VerifyDashboardResult holds the result of a dashboard structural verification.
type VerifyDashboardResult struct {
	Valid     bool     `json:"valid" yaml:"valid"`
	Name      string   `json:"name,omitempty" yaml:"name,omitempty"`
	TileCount int      `json:"tileCount" yaml:"tileCount"`
	Errors    []string `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings  []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// verifyDashboardCmd represents the verify dashboard subcommand
var verifyDashboardCmd = &cobra.Command{
	Use:     "dashboard -f <file>",
	Aliases: []string{"dash", "db"},
	Short:   "Verify dashboard definition structure",
	Long: `Verify a dashboard document definition locally without contacting the Dynatrace API.

This command performs Tier 1 (format) and Tier 2 (structural) validation on dashboard
JSON/YAML definitions. Because the Document API has no server-side validation endpoint,
all checks are performed client-side.

Checks performed:
  - Valid JSON or YAML syntax (Tier 1)
  - type field is "dashboard" if present
  - Presence of content or tiles field
  - tiles field inside content (or at root for direct format)
  - version field inside dashboard content
  - Double-nested content detection (.content.content)

The command returns different exit codes based on the result:
  0 - Dashboard structure is valid
  1 - Dashboard has errors (or warnings with --fail-on-warn)

Examples:
  # Verify a dashboard file
  dtctl verify dashboard -f dashboard.json
  dtctl verify dashboard -f dashboard.yaml

  # Read from stdin
  cat dashboard.json | dtctl verify dashboard -f -

  # Verify with template variables
  dtctl verify dashboard -f dashboard.yaml --set env=prod

  # Strict mode: fail on warnings
  dtctl verify dashboard -f dashboard.json --fail-on-warn

  # Structured output
  dtctl verify dashboard -f dashboard.json -o json
  dtctl verify dashboard -f dashboard.yaml -o yaml

  # CI/CD: validate all dashboards in a directory
  for file in dashboards/*.json; do
    dtctl verify dashboard -f "$file" --fail-on-warn || exit 1
  done
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dashFile, _ := cmd.Flags().GetString("file")
		setFlags, _ := cmd.Flags().GetStringArray("set")
		failOnWarn, _ := cmd.Flags().GetBool("fail-on-warn")

		if dashFile == "" {
			return fmt.Errorf("--file is required")
		}

		// Read file content
		var rawContent []byte
		var err error

		if dashFile == "-" {
			rawContent, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read dashboard from stdin: %w", err)
			}
		} else {
			rawContent, err = os.ReadFile(dashFile)
			if err != nil {
				return fmt.Errorf("failed to read dashboard file: %w", err)
			}
		}

		// Apply template rendering if --set flags are provided
		if len(setFlags) > 0 {
			vars, err := template.ParseSetFlags(setFlags)
			if err != nil {
				return fmt.Errorf("invalid --set flag: %w", err)
			}

			rendered, err := template.RenderTemplate(string(rawContent), vars)
			if err != nil {
				return fmt.Errorf("template rendering failed: %w", err)
			}

			rawContent = []byte(rendered)
		}

		// Tier 1: validate format and convert to JSON
		jsonData, err := format.ValidateAndConvert(rawContent)
		if err != nil {
			return fmt.Errorf("invalid dashboard file: %w", err)
		}

		// Unmarshal into generic map for structural inspection
		var doc map[string]interface{}
		if err := json.Unmarshal(jsonData, &doc); err != nil {
			return fmt.Errorf("failed to parse dashboard content: %w", err)
		}

		// Tier 2: structural validation
		errs, warnings, name, tileCount := validateDashboardStructure(doc)

		valid := len(errs) == 0
		result := VerifyDashboardResult{
			Valid:     valid,
			Name:      name,
			TileCount: tileCount,
			Errors:    errs,
			Warnings:  warnings,
		}

		// Determine exit code
		exitCode := 0
		if !valid {
			exitCode = 1
		} else if failOnWarn && len(warnings) > 0 {
			exitCode = 1
		}

		// Format output based on --output flag
		outputFmt, _ := cmd.Flags().GetString("output")

		switch outputFmt {
		case "json":
			printer := output.NewPrinter("json")
			if err := printer.Print(result); err != nil {
				return fmt.Errorf("failed to print JSON output: %w", err)
			}
		case "yaml", "yml":
			printer := output.NewPrinter("yaml")
			if err := printer.Print(result); err != nil {
				return fmt.Errorf("failed to print YAML output: %w", err)
			}
		default:
			// Default: human-readable format to stderr
			if err := formatVerifyDashboardResultHuman(result); err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}
		}

		// Exit with appropriate code if non-zero
		if exitCode != 0 {
			os.Exit(exitCode)
		}

		return nil
	},
}

// validateDashboardStructure performs Tier 2 structural checks on a parsed dashboard document.
// It replicates the validation logic from extractDocumentContent() in cmd/create_documents.go
// but returns structured errors and warnings instead of side-effects, keeping this command
// purely offline (no client needed).
func validateDashboardStructure(doc map[string]interface{}) (errs []string, warnings []string, name string, tileCount int) {
	name, _ = doc["name"].(string)

	// Check type field if present — must be "dashboard" when set
	if typeField, ok := doc["type"].(string); ok && typeField != "dashboard" {
		errs = append(errs, fmt.Sprintf("type field is %q, expected \"dashboard\"", typeField))
	}

	// Check for content (wrapped format) vs direct content (tiles at root)
	if content, hasContent := doc["content"]; hasContent {
		contentMap, ok := content.(map[string]interface{})
		if !ok {
			errs = append(errs, "content field must be an object")
			return
		}

		// Detect double-nested content (common copy/paste mistake)
		if innerContent, hasInner := contentMap["content"]; hasInner {
			if inner, ok := innerContent.(map[string]interface{}); ok {
				warnings = append(warnings, "detected double-nested content (.content.content) - using inner content")
				contentMap = inner
			}
		}

		// Check for tiles field (array or map/object form)
		if tiles, ok := contentMap["tiles"].([]interface{}); ok {
			tileCount = len(tiles)
		} else if tilesMap, ok := contentMap["tiles"].(map[string]interface{}); ok {
			tileCount = len(tilesMap)
		} else {
			warnings = append(warnings, "dashboard content has no 'tiles' field - dashboard may be empty")
		}

		// Check for version field
		if _, hasVersion := contentMap["version"]; !hasVersion {
			warnings = append(warnings, "dashboard content has no 'version' field")
		}
	} else if tiles, ok := doc["tiles"].([]interface{}); ok {
		// Direct content format: tiles at root level
		tileCount = len(tiles)
		if _, hasVersion := doc["version"]; !hasVersion {
			warnings = append(warnings, "dashboard content has no 'version' field")
		}
	} else if tilesMap, ok := doc["tiles"].(map[string]interface{}); ok {
		// Direct content format: tiles as map/object
		tileCount = len(tilesMap)
		if _, hasVersion := doc["version"]; !hasVersion {
			warnings = append(warnings, "dashboard content has no 'version' field")
		}
	} else {
		warnings = append(warnings, "no 'content' or 'tiles' field found - structure may be incorrect")
	}

	return
}

// formatVerifyDashboardResultHuman prints dashboard verification results in human-readable format
// to os.Stderr. Color output is used when stderr is a terminal.
func formatVerifyDashboardResultHuman(result VerifyDashboardResult) error {
	useColor := isStderrTerminal()

	// Print validation status line
	if len(result.Errors) == 0 {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✔%s Dashboard is valid\n", colorGreen, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✔ Dashboard is valid\n")
		}
	} else {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✖%s Dashboard has errors\n", colorRed, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✖ Dashboard has errors\n")
		}
	}

	// Show name and tile count
	if result.Name != "" {
		fmt.Fprintf(os.Stderr, "  Name: %q\n", result.Name)
	}
	fmt.Fprintf(os.Stderr, "  Tiles: %d\n", result.TileCount)

	// Print errors
	for _, e := range result.Errors {
		if useColor {
			fmt.Fprintf(os.Stderr, "  %sERROR:%s %s\n", colorRed, colorReset, e)
		} else {
			fmt.Fprintf(os.Stderr, "  ERROR: %s\n", e)
		}
	}

	// Print warnings
	for _, w := range result.Warnings {
		if useColor {
			fmt.Fprintf(os.Stderr, "  %sWARN:%s %s\n", colorYellow, colorReset, w)
		} else {
			fmt.Fprintf(os.Stderr, "  WARN: %s\n", w)
		}
	}

	return nil
}

func init() {
	verifyCmd.AddCommand(verifyDashboardCmd)

	verifyDashboardCmd.Flags().StringP("file", "f", "", "dashboard definition file (JSON/YAML, use '-' for stdin)")
	verifyDashboardCmd.Flags().StringArray("set", []string{}, "set template variable (key=value)")
	verifyDashboardCmd.Flags().Bool("fail-on-warn", false, "exit with non-zero status on warnings")
}
