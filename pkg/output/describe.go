package output

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
)

// DescribePrinter prints a single object as a vertical key-value layout,
// similar to kubectl describe. Keys are bold, values are plain.
// For list output, it delegates to a TablePrinter.
type DescribePrinter struct {
	writer io.Writer
	wide   bool
}

// describeFieldInfo holds metadata for a describe field
type describeFieldInfo struct {
	label   string
	indices []int
}

// getDescribeFields extracts fields for describe display.
// Uses the "table" struct tag for field names (same source of truth as tables).
func getDescribeFields(t reflect.Type, wide bool) []describeFieldInfo {
	tableFields := getTableFields(t, wide)
	fields := make([]describeFieldInfo, len(tableFields))
	for i, tf := range tableFields {
		fields[i] = describeFieldInfo{
			label:   formatDescribeLabel(tf.name),
			indices: tf.indices,
		}
	}
	return fields
}

// formatDescribeLabel converts a table header like "DISPLAY NAME" to "Display Name"
func formatDescribeLabel(header string) string {
	words := strings.Fields(header)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

// Print prints a single object as vertical key-value pairs.
func (p *DescribePrinter) Print(obj interface{}) error {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		// For non-struct types, just print the value
		_, _ = fmt.Fprintln(p.writer, obj)
		return nil
	}

	t := v.Type()
	fields := getDescribeFields(t, p.wide)

	if len(fields) == 0 {
		return nil
	}

	// Calculate max label width for alignment
	maxWidth := 0
	for _, f := range fields {
		if len(f.label) > maxWidth {
			maxWidth = len(f.label)
		}
	}

	for _, f := range fields {
		value := getFieldByPath(v, f.indices)
		formatted := formatDescribeValue(value)

		label := Colorize(Bold, fmt.Sprintf("%-*s", maxWidth, f.label))
		fmt.Fprintf(p.writer, "%s   %s\n", label, formatted)
	}

	return nil
}

// PrintList delegates to TablePrinter for list output —
// the vertical key-value format only makes sense for single objects.
func (p *DescribePrinter) PrintList(obj interface{}) error {
	table := &TablePrinter{writer: p.writer, wide: p.wide}
	return table.PrintList(obj)
}

// formatDescribeValue formats a value for describe display.
// Similar to formatValue but with richer handling for complex types.
func formatDescribeValue(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	// Handle time.Time
	if v.Type() == reflect.TypeOf(time.Time{}) {
		t := v.Interface().(time.Time)
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02 15:04:05")
	}

	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() || v.Len() == 0 {
			return ""
		}
		return fmt.Sprintf("<%d items>", v.Len())
	case reflect.Slice:
		if v.IsNil() || v.Len() == 0 {
			return ""
		}
		// For small string slices, show inline
		if v.Len() <= 5 && v.Type().Elem().Kind() == reflect.String {
			items := make([]string, v.Len())
			for i := 0; i < v.Len(); i++ {
				items[i] = v.Index(i).String()
			}
			return strings.Join(items, ", ")
		}
		return fmt.Sprintf("<%d items>", v.Len())
	case reflect.Bool:
		val := fmt.Sprintf("%v", v.Bool())
		return colorizeTableValue(val)
	default:
		val := fmt.Sprintf("%v", v.Interface())
		return colorizeTableValue(val)
	}
}
