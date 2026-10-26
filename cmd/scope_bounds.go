package cmd

import (
	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/dtctl/pkg/auth"
	"github.com/dynatrace-oss/dtctl/pkg/commands"
)

// annotationLocal marks a command that never reaches the platform: it works
// on the host (config, contexts, credentials, installed tools) or on dtctl
// itself (version, the commands catalog). Parse reports maximum scopes [] for
// it and for every command below it. Namespaced like pkg/stability's keys; its
// value is "true".
//
// TestLocalCommandsArePinned lists every command it covers, so a subcommand
// that inherits the mark from its group is a decision, not an accident.
const annotationLocal = "dtctl.io/local"

// annotationRunsDQL marks a command that runs DQL besides the API of its
// resource, itself (a describe that adds recent problems from Grail, a lookup
// read through load) or through the server with the caller's token (exec slo
// evaluates the SLO's indicator query). Its maximum scopes gain
// auth.QueryScopePattern. Its value is "true".
const annotationRunsDQL = "dtctl.io/runs-dql"

// Values of annotationScopes besides scopesInputDependent.
const (
	// scopesDQL: the command runs a DQL statement the caller gives (query),
	// and the statement decides which tables it reads. Its minimum is [] and
	// its maximum auth.QueryScopePattern.
	scopesDQL = "dql"
	// scopesAny: what the command calls is decided by its payload or target,
	// not by dtctl: it runs code the caller supplies (exec function), runs an
	// analyzer with the caller's token (exec analyzer), or stores an analyzer
	// input with a query (create anomaly-detector). Beyond the catalog's
	// scopes, its minimum, it can use any scope the user holds.
	scopesAny = "any"
)

// anyScope is the maximum of a command that can use any scope the user holds.
const anyScope = "*"

// markLocal declares that cmd, and every command below it, never reaches the
// platform.
func markLocal(cmd *cobra.Command) {
	setCommandAnnotation(cmd, annotationLocal, "true")
}

// markScopesDQL declares that cmd runs a DQL statement the caller gives.
func markScopesDQL(cmd *cobra.Command) {
	setCommandAnnotation(cmd, annotationScopes, scopesDQL)
}

// markScopesAny declares that cmd can use any scope the user holds.
func markScopesAny(cmd *cobra.Command) {
	setCommandAnnotation(cmd, annotationScopes, scopesAny)
}

// markRunsDQL declares that cmd runs DQL besides the API of its resource.
func markRunsDQL(cmd *cobra.Command) {
	setCommandAnnotation(cmd, annotationRunsDQL, "true")
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

// noScopes is the bounds of a command that reaches no platform API: a local
// command, or a help request.
func noScopes() (minimum, maximum []string) { return []string{}, []string{} }

// scopeBounds returns the scope bounds of c as parsed. minimum holds the exact
// scopes the command cannot run without; maximum holds patterns of the scopes
// it can use, where "*" matches any text. Both are non-nil, and every minimum
// scope matches a maximum pattern. The values come from the catalog
// (auth.ResourceScopes) and the declarations on the command, never from its
// input.
//
// A command the catalog has no entry for reports maximum ["*"]: only a command
// declared local reports [].
func scopeBounds(c *cobra.Command, verb, resource string) (minimum, maximum []string) {
	switch {
	case isLocal(c):
		return noScopes()
	case c.Annotations[annotationScopes] == scopesInputDependent, perCallScopeCommands[verb+" "+resource]:
		return []string{}, []string{anyScope}
	case c.Annotations[annotationScopes] == scopesDQL:
		return []string{}, []string{auth.QueryScopePattern}
	}
	minimum = []string{}
	if required, req := scopesForInvocation(c, verb, resource); req == scopeRequirementKnown {
		minimum = append(minimum, required...)
	}
	usable := resourceScopes(c, verb)
	if c.Annotations[annotationScopes] == scopesAny || (len(minimum) == 0 && len(usable) == 0) {
		return minimum, []string{anyScope}
	}
	maximum = unionScopes(minimum, usable)
	if c.Annotations[annotationRunsDQL] == "true" {
		maximum = unionScopes(maximum, []string{auth.QueryScopePattern})
	}
	return minimum, maximum
}

// resourceScopes returns the scopes of the resource c works on, at the verb's
// access level and at read: a write first reads what it changes, and a delete
// resolves a name with a list. The resource is the nearest of c and its
// ancestors below the verb that has a catalog entry, so `create aws
// connection` uses the entry of aws.
func resourceScopes(c *cobra.Command, verb string) []string {
	access := auth.AccessForVerb(verb, commands.MutatingVerbs[verb])
	for n := c; n.HasParent() && n.Parent().HasParent(); n = n.Parent() {
		if auth.HasResourceScopes(n.Name()) {
			return unionScopes(auth.ScopesForResource(n.Name(), access), auth.ScopesForResource(n.Name(), auth.AccessRead))
		}
	}
	return nil
}
