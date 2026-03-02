package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dynatrace-oss/dtctl/pkg/output"
	"github.com/dynatrace-oss/dtctl/pkg/resources/settings"
	"github.com/dynatrace-oss/dtctl/pkg/util/format"
	"github.com/dynatrace-oss/dtctl/pkg/util/template"
	"github.com/spf13/cobra"
)

// VerifySettingsResult holds the result of a settings validation for structured output
type VerifySettingsResult struct {
	Valid    bool     `json:"valid" yaml:"valid"`
	SchemaID string   `json:"schemaId" yaml:"schemaId"`
	Scope    string   `json:"scope" yaml:"scope"`
	Mode     string   `json:"mode" yaml:"mode"` // "create" or "update"
	Error    string   `json:"error,omitempty" yaml:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

// verifySettingsCmd represents the verify settings subcommand
var verifySettingsCmd = &cobra.Command{
	Use:     "settings -f <file>",
	Aliases: []string{"setting", "s"},
	Short:   "Verify settings objects against schema",
	Long: `Verify a settings object definition against the Dynatrace Settings v2 schema.

This command validates the settings object without creating or modifying anything.
It uses the server-side ?validateOnly=true API parameter for full schema validation.

The verify command returns different exit codes based on the result:
  0 - Settings object is valid
  1 - Settings object is invalid (schema violation, missing required fields, etc.)
  2 - Authentication/permission error
  3 - Network/server error

Examples:
  # Verify a settings object before creation
  dtctl verify settings -f profile.yaml

  # Verify with schema and scope overrides
  dtctl verify settings -f profile.yaml --schema builtin:alerting.profile --scope environment

  # Verify an update for an existing object
  dtctl verify settings -f profile.yaml --object-id vu9U3hXa3q0AAAABABlidWlsdGluOmFsZXJ0aW5nLnByb2ZpbGU

  # Verify with template variables
  dtctl verify settings -f profile.yaml --set name=prod --set env=production

  # Read from stdin
  cat profile.yaml | dtctl verify settings -f -

  # Get structured output (JSON or YAML)
  dtctl verify settings -f profile.yaml -o json
  dtctl verify settings -f profile.yaml -o yaml

  # CI/CD: Fail fast if settings are invalid
  if dtctl verify settings -f profile.yaml 2>/dev/null; then
    dtctl create settings -f profile.yaml
  else
    echo "Settings validation failed"
    exit 1
  fi

  # CI/CD: Validate all settings files
  for file in settings/*.yaml; do
    echo "Verifying $file..."
    dtctl verify settings -f "$file" || exit 1
  done
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fileFlag, _ := cmd.Flags().GetString("file")
		schemaFlag, _ := cmd.Flags().GetString("schema")
		scopeFlag, _ := cmd.Flags().GetString("scope")
		objectID, _ := cmd.Flags().GetString("object-id")
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
				return fmt.Errorf("failed to read settings from stdin: %w", err)
			}
		} else {
			fileData, err = os.ReadFile(fileFlag)
			if err != nil {
				return fmt.Errorf("failed to read settings file: %w", err)
			}
		}

		// Convert YAML to JSON if needed
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

		// Parse the JSON into a map
		var input map[string]any
		if err := json.Unmarshal(jsonData, &input); err != nil {
			return fmt.Errorf("failed to parse settings definition: %w", err)
		}

		// Load configuration and create client
		cfg, err := LoadConfig()
		if err != nil {
			return err
		}

		c, err := NewClientFromConfig(cfg)
		if err != nil {
			return err
		}

		handler := settings.NewHandler(c)

		// Determine validation mode and execute
		var validationErr error
		var mode, schemaID, scope string

		if objectID != "" {
			// UPDATE validation mode
			mode = "update"

			// Extract value: use input["value"] if present, otherwise use whole input as value
			var value map[string]any
			if v, ok := input["value"]; ok {
				if valueMap, ok := v.(map[string]any); ok {
					value = valueMap
				} else {
					return fmt.Errorf("field \"value\" must be an object")
				}
			} else {
				value = input
			}

			// Get schemaID and scope from flags or input
			schemaID = schemaFlag
			if schemaID == "" {
				if s, ok := input["schemaId"].(string); ok {
					schemaID = s
				}
			}
			scope = scopeFlag
			if scope == "" {
				if s, ok := input["scope"].(string); ok {
					scope = s
				}
			}

			validationErr = handler.ValidateUpdateWithContext(objectID, value, schemaID, scope)
		} else {
			// CREATE validation mode
			mode = "create"

			// Get schemaID: flag overrides file value
			schemaID = schemaFlag
			if schemaID == "" {
				if s, ok := input["schemaId"].(string); ok {
					schemaID = s
				}
			}

			// Get scope: flag overrides file value
			scope = scopeFlag
			if scope == "" {
				if s, ok := input["scope"].(string); ok {
					scope = s
				}
			}

			// schemaId is required for create
			if schemaID == "" {
				return fmt.Errorf("schemaId is required: provide --schema flag or include schemaId in the file")
			}

			// Extract value: use input["value"] if present, otherwise use whole input as value
			var value map[string]any
			if v, ok := input["value"]; ok {
				if valueMap, ok := v.(map[string]any); ok {
					value = valueMap
				} else {
					return fmt.Errorf("field \"value\" must be an object")
				}
			} else {
				value = input
			}

			validationErr = handler.ValidateCreate(settings.SettingsObjectCreate{
				SchemaID: schemaID,
				Scope:    scope,
				Value:    value,
			})
		}

		// Build the result struct
		result := VerifySettingsResult{
			Valid:    validationErr == nil,
			SchemaID: schemaID,
			Scope:    scope,
			Mode:     mode,
		}
		if validationErr != nil {
			result.Error = validationErr.Error()
		}

		// Determine exit code
		exitCode := getVerifySettingsExitCode(validationErr, result.Warnings, failOnWarn)

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
			if err := formatVerifySettingsResultHuman(result); err != nil {
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

// getVerifySettingsExitCode determines the exit code for settings validation
func getVerifySettingsExitCode(err error, warnings []string, failOnWarn bool) int {
	if err != nil {
		errMsg := err.Error()

		// Auth/permission errors (access denied, 401, 403)
		if strings.Contains(errMsg, "access denied") ||
			strings.Contains(errMsg, "status 401") ||
			strings.Contains(errMsg, "status 403") {
			return 2
		}

		// Network/server errors (5xx, timeout, connection)
		if strings.Contains(errMsg, "status 5") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "connection") {
			return 3
		}

		// Validation errors
		return 1
	}

	// No error, but warnings + --fail-on-warn → exit 1
	if failOnWarn && len(warnings) > 0 {
		return 1
	}

	return 0
}

// formatVerifySettingsResultHuman prints the verification result in human-readable format to stderr
func formatVerifySettingsResultHuman(result VerifySettingsResult) error {
	useColor := isStderrTerminal()

	if result.Valid {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✔%s Settings object is valid\n", colorGreen, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✔ Settings object is valid\n")
		}
	} else {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✖%s Settings object validation failed\n", colorRed, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✖ Settings object validation failed\n")
		}
	}

	if result.SchemaID != "" {
		fmt.Fprintf(os.Stderr, "  Schema: %s\n", result.SchemaID)
	}
	if result.Scope != "" {
		fmt.Fprintf(os.Stderr, "  Scope: %s\n", result.Scope)
	}
	fmt.Fprintf(os.Stderr, "  Mode: %s\n", result.Mode)

	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "  Error: %s\n", result.Error)
	}

	for _, w := range result.Warnings {
		if useColor {
			fmt.Fprintf(os.Stderr, "  %s⚠%s %s\n", colorYellow, colorReset, w)
		} else {
			fmt.Fprintf(os.Stderr, "  ⚠ %s\n", w)
		}
	}

	return nil
}

func init() {
	verifyCmd.AddCommand(verifySettingsCmd)

	verifySettingsCmd.Flags().StringP("file", "f", "", "settings definition file (JSON/YAML, use '-' for stdin)")
	verifySettingsCmd.Flags().String("schema", "", "schema ID (overrides file value)")
	verifySettingsCmd.Flags().String("scope", "", "scope (overrides file value)")
	verifySettingsCmd.Flags().String("object-id", "", "existing object ID for update validation")
	verifySettingsCmd.Flags().StringArray("set", []string{}, "set template variable (key=value)")
	verifySettingsCmd.Flags().Bool("fail-on-warn", false, "exit with non-zero status on warnings")
}
