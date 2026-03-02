package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dynatrace-oss/dtctl/pkg/output"
	"github.com/dynatrace-oss/dtctl/pkg/resources/analyzer"
	"github.com/dynatrace-oss/dtctl/pkg/util/format"
	"github.com/dynatrace-oss/dtctl/pkg/util/template"
	"github.com/spf13/cobra"
)

// verifyAnalyzerCmd represents the verify analyzer subcommand
var verifyAnalyzerCmd = &cobra.Command{
	Use:     "analyzer <analyzer-name>",
	Aliases: []string{"az"},
	Short:   "Verify Davis AI analyzer input without executing",
	Long: `Verify Davis AI analyzer input without executing the analyzer.

This command validates analyzer input against the analyzer's schema using the
server-side :validate endpoint. It checks for input errors and returns validation
details without running the analyzer.

The verify command returns different exit codes based on the result:
  0 - Input is valid
  1 - Input is invalid or has errors (or warnings with --fail-on-warn)
  2 - Authentication/permission error
  3 - Network/server error

Input can be provided as a JSON or YAML file, inline JSON, or a DQL query shorthand
for timeseries analyzers.

Examples:
  # Verify analyzer input from JSON file
  dtctl verify analyzer dt.statistics.GenericForecastAnalyzer -f input.json

  # Verify analyzer input from YAML file
  dtctl verify analyzer dt.statistics.GenericForecastAnalyzer -f input.yaml

  # Verify with inline JSON input
  dtctl verify analyzer dt.statistics.GenericForecastAnalyzer --input '{"timeSeriesData":"timeseries avg(dt.host.cpu.usage)"}'

  # Verify with DQL query shorthand (for timeseries analyzers)
  dtctl verify analyzer dt.statistics.GenericForecastAnalyzer --query "timeseries avg(dt.host.cpu.usage)"

  # Read input from stdin
  cat input.json | dtctl verify analyzer dt.statistics.GenericForecastAnalyzer -f -

  # Get structured output (JSON or YAML)
  dtctl verify analyzer dt.statistics.GenericForecastAnalyzer -f input.json -o json
  dtctl verify analyzer dt.statistics.GenericForecastAnalyzer -f input.json -o yaml

  # CI/CD: Fail on warnings (strict validation)
  dtctl verify analyzer dt.statistics.GenericForecastAnalyzer -f input.json --fail-on-warn
  if [ $? -eq 0 ]; then echo "Input is valid"; fi

  # Check exit codes for different scenarios
  dtctl verify analyzer dt.statistics.GenericForecastAnalyzer -f bad_input.json  # Exit 1: invalid input
  dtctl verify analyzer dt.statistics.GenericForecastAnalyzer -f input.json --fail-on-warn  # Exit 0 or 1 based on warnings

  # Script usage: verify before executing
  if dtctl verify analyzer dt.statistics.GenericForecastAnalyzer -f input.json 2>/dev/null; then
    dtctl exec analyzer dt.statistics.GenericForecastAnalyzer -f input.json
  else
    echo "Input validation failed"
    exit 1
  fi
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		analyzerName := args[0]

		inputFile, _ := cmd.Flags().GetString("file")
		inputJSON, _ := cmd.Flags().GetString("input")
		query, _ := cmd.Flags().GetString("query")
		setFlags, _ := cmd.Flags().GetStringArray("set")
		failOnWarn, _ := cmd.Flags().GetBool("fail-on-warn")

		var input map[string]interface{}

		if inputFile != "" {
			// Read input from file (use "-" for stdin)
			var rawContent []byte
			var err error

			if inputFile == "-" {
				rawContent, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read input from stdin: %w", err)
				}
			} else {
				rawContent, err = os.ReadFile(inputFile)
				if err != nil {
					return fmt.Errorf("failed to read input file: %w", err)
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

			// Convert JSON/YAML to JSON
			jsonData, err := format.ValidateAndConvert(rawContent)
			if err != nil {
				return fmt.Errorf("failed to parse input file: %w", err)
			}

			if err := json.Unmarshal(jsonData, &input); err != nil {
				return fmt.Errorf("failed to unmarshal input: %w", err)
			}
		} else if inputJSON != "" {
			if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
				return fmt.Errorf("failed to parse input JSON: %w", err)
			}
		} else if query != "" {
			// Shorthand for timeseries query
			input = map[string]interface{}{
				"timeSeriesData": query,
			}
		} else {
			return fmt.Errorf("input is required: use --file, --input, or --query")
		}

		cfg, err := LoadConfig()
		if err != nil {
			return err
		}

		c, err := NewClientFromConfig(cfg)
		if err != nil {
			return err
		}

		handler := analyzer.NewHandler(c)

		result, err := handler.Validate(analyzerName, input)

		// Get exit code first (needed for all output formats)
		exitCode := getVerifyAnalyzerExitCode(result, err, failOnWarn)

		// Handle errors (network, auth, API)
		if err != nil {
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return err
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
			if err := formatVerifyAnalyzerResultHuman(result); err != nil {
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

// formatVerifyAnalyzerResultHuman prints analyzer validation results in human-readable format
func formatVerifyAnalyzerResultHuman(result *analyzer.ValidationResult) error {
	useColor := isStderrTerminal()

	// Print validation status
	if result.Valid {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✔%s Analyzer input is valid\n", colorGreen, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✔ Analyzer input is valid\n")
		}
	} else {
		if useColor {
			fmt.Fprintf(os.Stderr, "%s✖%s Analyzer input is invalid\n", colorRed, colorReset)
		} else {
			fmt.Fprintf(os.Stderr, "✖ Analyzer input is invalid\n")
		}
	}

	// Print details if available
	if len(result.Details) > 0 {
		fmt.Fprintf(os.Stderr, "\nDetails:\n")
		for k, v := range result.Details {
			// Format value as string
			var valStr string
			switch tv := v.(type) {
			case string:
				valStr = tv
			default:
				b, err := json.Marshal(v)
				if err != nil {
					valStr = fmt.Sprintf("%v", v)
				} else {
					valStr = string(b)
				}
			}
			fmt.Fprintf(os.Stderr, "  %s: %s\n", k, valStr)
		}
	}

	return nil
}

// getVerifyAnalyzerExitCode determines the exit code based on analyzer validation results and errors
func getVerifyAnalyzerExitCode(result *analyzer.ValidationResult, err error, failOnWarn bool) int {
	// Handle errors first
	if err != nil {
		errMsg := err.Error()

		// Check for auth/permission errors (401, 403)
		if strings.Contains(errMsg, "status 401") || strings.Contains(errMsg, "status 403") {
			return 2
		}

		// Check for network/server errors (timeout, 5xx)
		if strings.Contains(errMsg, "status 5") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "connection") {
			return 3
		}

		// Other errors (likely client-side issues)
		return 1
	}

	// No error from API call, check validation result
	if result == nil {
		return 1
	}

	// Check if input is invalid
	if !result.Valid {
		return 1
	}

	// Check for warning-like entries in details if --fail-on-warn is set
	if failOnWarn && len(result.Details) > 0 {
		for k, v := range result.Details {
			keyLower := strings.ToLower(k)
			valLower := strings.ToLower(fmt.Sprintf("%v", v))
			if strings.Contains(keyLower, "warn") || strings.Contains(valLower, "warn") {
				return 1
			}
		}
	}

	// Valid input with no errors
	return 0
}

func init() {
	verifyCmd.AddCommand(verifyAnalyzerCmd)

	// Flags for verify analyzer command
	verifyAnalyzerCmd.Flags().StringP("file", "f", "", "read input from JSON/YAML file (use '-' for stdin)")
	verifyAnalyzerCmd.Flags().String("input", "", "inline JSON input")
	verifyAnalyzerCmd.Flags().String("query", "", "DQL query shorthand (for timeseries analyzers)")
	verifyAnalyzerCmd.Flags().StringArray("set", []string{}, "set template variable (key=value)")
	verifyAnalyzerCmd.Flags().Bool("fail-on-warn", false, "exit with non-zero status on warnings")
}
