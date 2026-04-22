//go:build windows

package output

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVTProcessing enables ANSI/VT escape sequence processing on the
// Windows console. Returns true if VT sequences are supported, false otherwise.
//
// On Windows 10+, the console supports VT sequences but they must be
// explicitly enabled via SetConsoleMode with ENABLE_VIRTUAL_TERMINAL_PROCESSING.
// See: https://learn.microsoft.com/en-us/windows/console/console-virtual-terminal-sequences
func enableVTProcessing() bool {
	handle := windows.Handle(os.Stdout.Fd())

	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}

	if err := windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return false
	}

	return true
}
