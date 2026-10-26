package cmd

import (
	"context"
	"sort"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// localCommands are the runnable commands declared, or inheriting the
// declaration, with markLocal. Parse reports maximum scopes [] for them, so a
// host gives them a token without scopes: a command listed here that does
// reach the platform fails for every caller.
var localCommands = []string{
	"alias delete",
	"alias export",
	"alias import",
	"alias list",
	"alias set",
	"auth login",
	"auth logout",
	"auth refresh",
	"auth status",
	"commands",
	"commands howto",
	"completion",
	"config current-context",
	"config delete-context",
	"config delete-credentials",
	"config describe-context",
	"config get-contexts",
	"config init",
	"config list-development",
	"config migrate-tokens",
	"config set",
	"config set-context",
	"config set-credentials",
	"config use-context",
	"config view",
	"ctx",
	"ctx current",
	"ctx delete",
	"ctx describe",
	"ctx set",
	"ctx token",
	"inspect",
	"plugin list",
	"skills",
	"skills install",
	"skills status",
	"skills uninstall",
	"update settings",
	"usage",
	"usage reset",
	"version",
}

// TestLocalCommandsArePinned holds the set of local commands to the list
// above. A new subcommand of a local group inherits the mark; the list makes
// that a decision someone took.
func TestLocalCommandsArePinned(t *testing.T) {
	root := newCommandTree(withInvocation(context.Background(), &invocation{concurrent: true})).root
	var got []string
	walkCommands(root, func(c *cobra.Command) {
		if isParseRunnable(c) && isLocal(c) {
			got = append(got, commandPathRelative(c, root))
		}
	})
	sort.Strings(got)
	assert.Equal(t, localCommands, got,
		"a command was added to or removed from the local set: update localCommands if it never reaches the platform, else remove its markLocal")
}

// isParseRunnable reports whether c has a body Parse records: runnable, and
// not a group that only refuses a missing subcommand.
func isParseRunnable(c *cobra.Command) bool {
	return c.HasParent() && c.Runnable() && (c.RunE == nil || !isRequireSubcommand(c.RunE))
}
