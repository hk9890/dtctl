package cmd

import "github.com/spf13/cobra"

// annotationLocal marks a command that never reaches the platform: it works
// on the host (config, contexts, credentials, installed tools) or on dtctl
// itself (version, the commands catalog). Parse reports maximum scopes [] for
// it and for every command below it. Namespaced like pkg/stability's keys; its
// value is "true".
//
// TestLocalCommandsArePinned lists every command it covers, so a subcommand
// that inherits the mark from its group is a decision, not an accident.
const annotationLocal = "dtctl.io/local"

// markLocal declares that cmd, and every command below it, never reaches the
// platform.
func markLocal(cmd *cobra.Command) {
	setCommandAnnotation(cmd, annotationLocal, "true")
}

// isLocal reports whether c or one of its ancestors is declared with
// markLocal.
func isLocal(c *cobra.Command) bool {
	for n := c; n != nil; n = n.Parent() {
		if n.Annotations[annotationLocal] == "true" {
			return true
		}
	}
	return false
}
