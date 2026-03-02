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

// VerifyWorkflowResult holds the result of a workflow structural verification
type VerifyWorkflowResult struct {
	Valid    bool     `json:"valid" yaml:"valid"`
	Title    string   `json:"title,omitempty" yaml:"title,omitempty"`
	Tasks    int      `json:"taskCount" yaml:"taskCount"`
	Errors   []string `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// verifyWorkflowCmd represents the verify workflow subcommand
var verifyWorkflowCmd = &cobra.Command{
	Use:     "workflow -f <file>",
	Aliases: []string{"wf"},
	Short:   "Verify workflow definition structure",
	Long: `Verify a workflow definition file for structural correctness.

This command performs client-side structural validation of workflow definitions
(JSON or YAML) without making any API calls. It checks required fields, task
structure, and common configuration issues.

No Dynatrace API access is required — validation is entirely offline.

The verify command returns different exit codes based on the result:
  0 - Workflow is valid (no errors)
  1 - Workflow is invalid (has errors, or warnings with --fail-on-warn)

Checks performed:
  - Valid JSON or YAML syntax
  - Required field: title (non-empty string)
  - Required field: tasks (must be an object/map)
  - Each task: must have an action field
  - Warning: no trigger defined (workflow can only be executed manually)
  - Warning: empty tasks map
  - Warning: owner field present but empty

Template variables (--set) can be used to substitute values before validation,
matching the same template syntax used in create/apply commands.

Examples:
  # Verify a workflow from YAML file
  dtctl verify workflow -f workflow.yaml

  # Verify a workflow from JSON file
  dtctl verify workflow -f workflow.json

  # Read from stdin
  dtctl verify workflow -f - < workflow.yaml

  # Verify with template variables substituted first
  dtctl verify workflow -f workflow.yaml --set env=prod --set owner=team-a

  # Fail on warnings (strict validation for CI/CD)
  dtctl verify workflow -f workflow.yaml --fail-on-warn

  # Get structured output (JSON)
  dtctl verify workflow -f workflow.yaml -o json

  # Get structured output (YAML)
  dtctl verify workflow -f workflow.yaml -o yaml

  # CI/CD: Validate before creating
  if dtctl verify workflow -f workflow.yaml --fail-on-warn 2>/dev/null; then
    dtctl create workflow -f workflow.yaml
  else
    echo "Workflow validation failed"
    exit 1
  fi

  # Validate all workflow files in a directory
  for file in workflows/*.yaml; do
    echo "Verifying $file..."
    dtctl verify workflow -f "$file" --fail-on-warn || exit 1
  done
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowFile, _ := cmd.Flags().GetString("file")
		if workflowFile == "" {
			return fmt.Errorf("--file is required")
		}

		setFlags, _ := cmd.Flags().GetStringArray("set")
		failOnWarn, _ := cmd.Flags().GetBool("fail-on-warn")

		// Read file content (support "-" for stdin)
		var fileData []byte
		var err error
		if workflowFile == "-" {
			fileData, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read workflow from stdin: %w", err)
			}
		} else {
			fileData, err = os.ReadFile(workflowFile)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
		}

		// Tier 1: format validation — converts YAML to JSON if needed
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

		// Parse JSON into a generic map for structural inspection
		var doc map[string]interface{}
		if err := json.Unmarshal(jsonData, &doc); err != nil {
			return fmt.Errorf("failed to parse workflow definition: %w", err)
		}

		// Tier 2: structural validation
		errors, warnings := validateWorkflowStructure(doc)

		// Extract title and task count for the result
		title, _ := doc["title"].(string)
		taskCount := 0
		if tasks, ok := doc["tasks"].(map[string]interface{}); ok {
			taskCount = len(tasks)
		}

		result := VerifyWorkflowResult{
			Valid:    len(errors) == 0,
			Title:    title,
			Tasks:    taskCount,
			Errors:   errors,
			Warnings: warnings,
		}

		// Determine exit code
		exitCode := 0
		if len(errors) > 0 {
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
			if err := formatVerifyWorkflowResultHuman(result); err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}
		}

		if exitCode != 0 {
			os.Exit(exitCode)
		}

		return nil
	},
}

// validateWorkflowStructure performs structural validation of a workflow definition.
// It returns a list of errors (blocking) and warnings (advisory).
func validateWorkflowStructure(doc map[string]interface{}) (errors []string, warnings []string) {
	// Required: title field (non-empty string)
	if title, ok := doc["title"].(string); !ok || title == "" {
		errors = append(errors, "missing or empty required field: title")
	}

	// Required: tasks field (must be a map/object)
	tasks, ok := doc["tasks"].(map[string]interface{})
	if !ok {
		errors = append(errors, "missing or invalid required field: tasks (must be an object)")
	} else {
		if len(tasks) == 0 {
			warnings = append(warnings, "workflow has no tasks defined")
		}
		for taskName, taskVal := range tasks {
			task, ok := taskVal.(map[string]interface{})
			if !ok {
				errors = append(errors, fmt.Sprintf("task %q: must be an object", taskName))
				continue
			}
			if _, hasAction := task["action"].(string); !hasAction {
				errors = append(errors, fmt.Sprintf("task %q: missing required field: action", taskName))
			}
		}
	}

	// Optional but expected: trigger
	if _, hasTrigger := doc["trigger"]; !hasTrigger {
		warnings = append(warnings, "no trigger defined - workflow can only be executed manually")
	}

	// Optional: owner — if present, must be a non-empty string
	if owner, hasOwner := doc["owner"]; hasOwner {
		if ownerStr, ok := owner.(string); !ok || ownerStr == "" {
			warnings = append(warnings, "owner field is present but empty")
		}
	}

	return errors, warnings
}

// formatVerifyWorkflowResultHuman prints verification results in human-readable format to stderr.
func formatVerifyWorkflowResultHuman(result VerifyWorkflowResult) error {
	useColor := isStderrTerminal()

	// Print validation status
	if result.Valid {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✔%s Workflow is valid\n", colorGreen, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✔ Workflow is valid\n")
		}
	} else {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✖%s Workflow has errors\n", colorRed, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✖ Workflow has errors\n")
		}
	}

	// Print title and task count when valid (summary info)
	if result.Valid {
		if result.Title != "" {
			fmt.Fprintf(os.Stderr, "  Title: %q\n", result.Title)
		}
		fmt.Fprintf(os.Stderr, "  Tasks: %d\n", result.Tasks)
	}

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
	verifyCmd.AddCommand(verifyWorkflowCmd)

	verifyWorkflowCmd.Flags().StringP("file", "f", "", "workflow definition file (JSON/YAML, use '-' for stdin)")
	verifyWorkflowCmd.Flags().StringArray("set", []string{}, "set template variable (key=value)")
	verifyWorkflowCmd.Flags().Bool("fail-on-warn", false, "exit with non-zero status on warnings")
}
