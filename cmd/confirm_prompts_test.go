package cmd

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// confirmationSites maps every call in cmd/ that asks for confirmation on
// stdin (prompt.Confirm*) to the command that asks, by function. An entry
// without a command names the reason it is not declared.
var confirmationSites = map[string]string{
	"create_breakpoints.go:newCreateBreakpointCmd":         "create breakpoint",
	"update_breakpoints.go:newUpdateBreakpointCmd":         "update breakpoint",
	"delete_breakpoints.go:runDeleteAllBreakpointsWithOps": "delete breakpoint",
	"delete_breakpoints.go:runDeleteBreakpointRowsWithOps": "delete breakpoint",
	"get_anomalydetector.go:newDeleteAnomalyDetectorCmd":   "delete anomaly-detector",
	"get_apps.go:newDeleteAppCmd":                          "delete app",
	"get_buckets.go:newDeleteBucketCmd":                    "delete bucket",
	"get_documents.go:newDeleteDashboardCmd":               "delete dashboard",
	"get_documents.go:newDeleteNotebookCmd":                "delete notebook",
	"get_documents.go:newDeleteTrashCmd":                   "delete trash",
	"get_documents.go:newDeleteDocumentCmd":                "delete document",
	"get_edgeconnects.go:newDeleteEdgeConnectCmd":          "delete edgeconnect",
	"get_lookups.go:newDeleteLookupCmd":                    "delete lookup",
	"get_notifications.go:newDeleteNotificationCmd":        "delete notification",
	"get_scheduling_rules.go:newDeleteSchedulingRuleCmd":   "delete scheduling-rule",
	"get_segments.go:newDeleteSegmentCmd":                  "delete segment",
	"get_settings.go:newDeleteSettingsCmd":                 "delete settings",
	"get_slos.go:newDeleteSLOCmd":                          "delete slo",
	"get_workflows.go:newDeleteWorkflowCmd":                "delete workflow",
	"restore.go:newRestoreWorkflowCmd":                     "restore workflow",
	"restore.go:newRestoreDashboardCmd":                    "restore dashboard",
	"restore.go:newRestoreNotebookCmd":                     "restore notebook",
	"restore.go:newRestoreDocumentCmd":                     "restore document",
	"restore.go:newRestoreTrashCmd":                        "restore trash",
	"usage.go:runUsageReset":                               "usage reset",
	"auth.go:":                                             "", // auth login's keyring fallback: it asks only when the host keyring fails, and auth is local
}

// TestConfirmationPromptsAreDeclared holds that every command that asks for
// confirmation on stdin is declared with markConfirms, so Parse reports it as
// RequiresInput. A host that trusted RequiresInput on an undeclared command
// would send no input, and the command would answer its own question with
// "no" and exit 0 having done nothing.
func TestConfirmationPromptsAreDeclared(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	found := map[string]bool{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		for _, decl := range f.Decls {
			enclosing := ""
			if fn, ok := decl.(*ast.FuncDecl); ok {
				enclosing = fn.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !strings.HasPrefix(sel.Sel.Name, "Confirm") {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "prompt" {
					return true
				}
				site := name + ":" + enclosing
				found[site] = true
				if _, ok := confirmationSites[site]; !ok {
					t.Errorf("%s: %s asks for confirmation in %s; declare its command with markConfirms and list it in confirmationSites",
						fset.Position(call.Pos()), sel.Sel.Name, site)
				}
				return true
			})
		}
	}
	for site := range confirmationSites {
		if !found[site] {
			t.Errorf("%s no longer asks for confirmation: remove its entry", site)
		}
	}

	root := newCommandTree(withInvocation(context.Background(), &invocation{concurrent: true})).root
	listed := map[string]bool{}
	for _, path := range confirmationSites {
		if path != "" {
			listed[path] = true
		}
	}
	walkCommands(root, func(c *cobra.Command) {
		path := commandPathRelative(c, root)
		skips, declared := c.Annotations[annotationConfirms]
		switch {
		case listed[path] && !declared:
			t.Errorf("%s asks for confirmation; declare it with markConfirms(c, <flags that skip the question>)", path)
		case declared && !listed[path]:
			t.Errorf("%s is declared with markConfirms, but no confirmationSites entry names it", path)
		}
		if !declared {
			return
		}
		flags := commandFlags(root, c)
		for _, name := range strings.Split(skips, ",") {
			if _, ok := flags[name]; !ok {
				t.Errorf("%s: markConfirms names --%s, which the command does not have", path, name)
			}
		}
		delete(listed, path)
	})
	for path := range listed {
		t.Errorf("confirmationSites names %q, which is not in the command tree", path)
	}
}
