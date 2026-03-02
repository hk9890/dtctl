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

// VerifyNotebookResult holds the result of a notebook structural validation for structured output
type VerifyNotebookResult struct {
	Valid        bool     `json:"valid" yaml:"valid"`
	Name         string   `json:"name,omitempty" yaml:"name,omitempty"`
	SectionCount int      `json:"sectionCount" yaml:"sectionCount"`
	Errors       []string `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings     []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// verifyNotebookCmd represents the verify notebook subcommand
var verifyNotebookCmd = &cobra.Command{
	Use:     "notebook -f <file>",
	Aliases: []string{"nb"},
	Short:   "Verify notebook definition structure",
	Long: `Verify a notebook document definition file locally without contacting the Dynatrace API.

This command performs Tier 1 (format) and Tier 2 (structural) validation:
  - Tier 1: Valid JSON or YAML syntax
  - Tier 2: Structural checks (sections field, content nesting, type field)

No Dynatrace API access is required — all validation is performed offline.

The verify command returns different exit codes based on the result:
  0 - Notebook is valid (or has only warnings without --fail-on-warn)
  1 - Notebook is invalid or has errors (or warnings with --fail-on-warn)

Examples:
  # Verify a notebook definition file
  dtctl verify notebook -f notebook.yaml

  # Verify notebook from JSON file
  dtctl verify notebook -f notebook.json

  # Read from stdin
  cat notebook.yaml | dtctl verify notebook -f -

  # Verify with template variables
  dtctl verify notebook -f notebook.yaml --set name=MyNotebook

  # Get structured output (JSON or YAML)
  dtctl verify notebook -f notebook.yaml -o json
  dtctl verify notebook -f notebook.yaml -o yaml

  # CI/CD: Fail on warnings (strict validation)
  dtctl verify notebook -f notebook.yaml --fail-on-warn
  if [ $? -eq 0 ]; then echo "Notebook is valid"; fi

  # CI/CD: Validate all notebooks in a directory
  for file in notebooks/*.yaml; do
    echo "Verifying $file..."
    dtctl verify notebook -f "$file" --fail-on-warn || exit 1
  done
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fileFlag, _ := cmd.Flags().GetString("file")
		setFlags, _ := cmd.Flags().GetStringArray("set")
		failOnWarn, _ := cmd.Flags().GetBool("fail-on-warn")

		if fileFlag == "" {
			return fmt.Errorf("--file is required")
		}

		// Read file: support "-" for stdin
		var fileData []byte
		var err error
		if fileFlag == "-" {
			fileData, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read notebook from stdin: %w", err)
			}
		} else {
			fileData, err = os.ReadFile(fileFlag)
			if err != nil {
				return fmt.Errorf("failed to read notebook file: %w", err)
			}
		}

		// Tier 1: validate format and convert YAML to JSON if needed
		jsonData, err := format.ValidateAndConvert(fileData)
		if err != nil {
			return fmt.Errorf("invalid file format: %w", err)
		}

		// Apply template rendering if --set flags are provided
		if len(setFlags) > 0 {
			vars, err := template.ParseSetFlags(setFlags)
			if err != nil {
				return fmt.Errorf("invalid --set flag: %w", err)
			}
			rendered, err := template.RenderTemplate(string(jsonData), vars)
			if err != nil {
				return fmt.Errorf("template rendering failed: %w", err)
			}
			jsonData = []byte(rendered)
		}

		// Parse the JSON into a map for structural validation
		var doc map[string]interface{}
		if err := json.Unmarshal(jsonData, &doc); err != nil {
			return fmt.Errorf("failed to parse notebook definition: %w", err)
		}

		// Tier 2: structural validation
		errs, warnings, name, sectionCount := validateNotebookStructure(doc)

		// Build result
		result := VerifyNotebookResult{
			Valid:        len(errs) == 0,
			Name:         name,
			SectionCount: sectionCount,
			Errors:       errs,
			Warnings:     warnings,
		}

		// Determine exit code: errors → 1, warnings + failOnWarn → 1, else 0
		exitCode := 0
		if len(errs) > 0 {
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
			if err := formatVerifyNotebookResultHuman(result); err != nil {
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

// validateNotebookStructure performs structural validation on a parsed notebook document.
// Returns errors, warnings, the notebook name, and section count.
func validateNotebookStructure(doc map[string]interface{}) (errs []string, warnings []string, name string, sectionCount int) {
	name, _ = doc["name"].(string)

	// Check type field if present — must be "notebook" if set
	if typeField, ok := doc["type"].(string); ok && typeField != "notebook" {
		errs = append(errs, fmt.Sprintf("type field is %q, expected \"notebook\"", typeField))
	}

	// Check for content (wrapped format vs direct format)
	if content, hasContent := doc["content"]; hasContent {
		contentMap, ok := content.(map[string]interface{})
		if !ok {
			errs = append(errs, "content field must be an object")
			return
		}

		// Detect double-nested content (.content.content) — common copy/paste mistake
		if innerContent, hasInner := contentMap["content"]; hasInner {
			if inner, ok := innerContent.(map[string]interface{}); ok {
				warnings = append(warnings, "detected double-nested content (.content.content) - using inner content")
				contentMap = inner
			}
		}

		// Check for sections in the content object
		if sections, ok := contentMap["sections"].([]interface{}); ok {
			sectionCount = len(sections)
		} else if sectionsMap, ok := contentMap["sections"].(map[string]interface{}); ok {
			sectionCount = len(sectionsMap)
		} else {
			warnings = append(warnings, "notebook content has no 'sections' field - notebook may be empty")
		}
	} else if sections, ok := doc["sections"].([]interface{}); ok {
		// Direct content format: sections at root level
		sectionCount = len(sections)
	} else if sectionsMap, ok := doc["sections"].(map[string]interface{}); ok {
		sectionCount = len(sectionsMap)
	} else {
		warnings = append(warnings, "no 'content' or 'sections' field found - structure may be incorrect")
	}

	return
}

// formatVerifyNotebookResultHuman prints the notebook verification result in human-readable format to stderr
func formatVerifyNotebookResultHuman(result VerifyNotebookResult) error {
	useColor := isStderrTerminal()

	// Print validation status line
	if len(result.Errors) == 0 {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✔%s Notebook is valid\n", colorGreen, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✔ Notebook is valid\n")
		}
	} else {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✖%s Notebook has errors\n", colorRed, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✖ Notebook has errors\n")
		}
	}

	// Print name if available
	if result.Name != "" {
		fmt.Fprintf(os.Stderr, "  Name: %q\n", result.Name)
	}

	// Print section count
	fmt.Fprintf(os.Stderr, "  Sections: %d\n", result.SectionCount)

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
	verifyCmd.AddCommand(verifyNotebookCmd)

	verifyNotebookCmd.Flags().StringP("file", "f", "", "notebook definition file (JSON/YAML, use '-' for stdin)")
	verifyNotebookCmd.Flags().StringArray("set", []string{}, "set template variable (key=value)")
	verifyNotebookCmd.Flags().Bool("fail-on-warn", false, "exit with non-zero status on warnings")
}
