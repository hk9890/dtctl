package output

import (
	"fmt"
	"io"
	"os"
)

// PrintSuccess prints a success message with a green checkmark prefix.
// Output goes to stderr so it doesn't interfere with structured stdout.
func PrintSuccess(format string, args ...interface{}) {
	FprintSuccess(os.Stderr, format, args...)
}

// FprintSuccess prints a success message to the given writer.
func FprintSuccess(w io.Writer, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	prefix := Colorize(Green, "OK")
	fmt.Fprintf(w, "%s %s\n", prefix, msg)
}

// PrintWarning prints a warning message with a yellow prefix.
// Output goes to stderr so it doesn't interfere with structured stdout.
func PrintWarning(format string, args ...interface{}) {
	FprintWarning(os.Stderr, format, args...)
}

// FprintWarning prints a warning message to the given writer.
func FprintWarning(w io.Writer, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	prefix := Colorize(Yellow, "Warning:")
	fmt.Fprintf(w, "%s %s\n", prefix, msg)
}
