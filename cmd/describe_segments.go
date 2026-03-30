package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtctl/pkg/output"
	"github.com/dynatrace-oss/dtctl/pkg/resources/segment"
)

// describeSegmentCmd shows detailed info about a segment
var describeSegmentCmd = &cobra.Command{
	Use:     "segment <uid>",
	Aliases: []string{"seg", "filter-segment"},
	Short:   "Show details of a Grail filter segment",
	Long: `Show detailed information about a Grail filter segment.

Examples:
  # Describe a segment
  dtctl describe segment <uid>
  dtctl describe seg <uid>
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uid := args[0]

		cfg, err := LoadConfig()
		if err != nil {
			return err
		}

		c, err := NewClientFromConfig(cfg)
		if err != nil {
			return err
		}

		handler := segment.NewHandler(c)

		seg, err := handler.Get(uid)
		if err != nil {
			return err
		}

		// For table output, show detailed human-readable information
		if outputFormat == "table" {
			const w = 16
			output.DescribeKV("Name:", w, "%s", seg.Name)
			output.DescribeKV("UID:", w, "%s", seg.UID)
			if seg.Description != "" {
				output.DescribeKV("Description:", w, "%s", seg.Description)
			}
			if seg.IsPublic {
				output.DescribeKV("Public:", w, "Yes")
			} else {
				output.DescribeKV("Public:", w, "No")
			}
			if seg.Owner != "" {
				output.DescribeKV("Owner:", w, "%s", seg.Owner)
			}
			output.DescribeKV("Version:", w, "%d", seg.Version)

			// Includes
			if len(seg.Includes) > 0 {
				fmt.Println()
				output.DescribeSection("Includes:")
				fmt.Printf("  %-20s %s\n", "TYPE", "FILTER")
				for _, inc := range seg.Includes {
					dataType := inc.DataType
					if strings.EqualFold(dataType, "all") {
						dataType = "All data types"
					} else {
						dataType = strings.Title(dataType) //nolint:staticcheck
					}
					fmt.Printf("  %-20s %s\n", dataType, inc.Filter)
				}
			}

			// Variables
			if seg.Variables != nil {
				fmt.Println()
				output.DescribeSection("Variables:")
				if seg.Variables.Query != "" {
					output.DescribeKV("  Query:", 12, "%s", seg.Variables.Query)
				}
				if len(seg.Variables.Columns) > 0 {
					output.DescribeKV("  Columns:", 12, "%s", strings.Join(seg.Variables.Columns, ", "))
				}
			}

			// Allowed operations
			if len(seg.AllowedOperations) > 0 {
				fmt.Println()
				output.DescribeKV("Operations:", w, "%s", strings.Join(seg.AllowedOperations, ", "))
			}

			return nil
		}

		// For other formats, use standard printer
		printer := NewPrinter()
		enrichAgent(printer, "describe", "segment")
		return printer.Print(seg)
	},
}
