package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Scope bounds the cases below expect.
var (
	noScopes  = []string{}
	anyScope  = []string{"*"}
	dqlScopes = []string{"storage:*:read"}
)

// TestParse holds the answers a host relies on: the files a command line
// reads and writes, whether it reads the request's input, and the scope
// bounds.
func TestParse(t *testing.T) {
	for name, tc := range map[string]struct {
		command string
		want    Parsed
	}{
		"file flag": {
			command: "create workflow -f wf.yaml",
			want: Parsed{Verb: "create", Resource: "workflow", InputFiles: []string{"wf.yaml"},
				MinimumScopes: []string{"automation:workflows:write"},
				MaximumScopes: []string{"automation:workflows:read", "automation:workflows:write"}},
		},
		"write-back": {
			command: "apply -f ./dashboard.yaml --write-id",
			want: Parsed{Verb: "apply", InputFiles: []string{"dashboard.yaml"}, OutputFiles: []string{"dashboard.yaml"},
				MinimumScopes: noScopes, MaximumScopes: anyScope},
		},
		"stdin as a file": {
			command: "apply -f -",
			want:    Parsed{Verb: "apply", RequiresInput: true, MinimumScopes: noScopes, MaximumScopes: anyScope},
		},
		"at-path body": {
			command: "exec api /platform/x -X POST -d @body.json",
			want: Parsed{Verb: "exec", Resource: "api", InputFiles: []string{"body.json"},
				MinimumScopes: noScopes, MaximumScopes: anyScope},
		},
		"inline body": {
			command: `exec api /platform/x -X POST -d '{"a":1}'`,
			want:    Parsed{Verb: "exec", Resource: "api", MinimumScopes: noScopes, MaximumScopes: anyScope},
		},
		"query from stdin": {
			command: "query",
			want:    Parsed{Verb: "query", RequiresInput: true, MinimumScopes: noScopes, MaximumScopes: dqlScopes},
		},
		"output format is not a file": {
			command: "query 'fetch logs' -o json",
			want:    Parsed{Verb: "query", MinimumScopes: noScopes, MaximumScopes: dqlScopes},
		},
		"two files": {
			command: "diff -f a.yaml -f /b.yaml",
			want:    Parsed{Verb: "diff", InputFiles: []string{"a.yaml", "b.yaml"}, MinimumScopes: noScopes, MaximumScopes: anyScope},
		},
		"empty argument flag still reads stdin": {
			command: `query --dql ""`,
			want:    Parsed{Verb: "query", RequiresInput: true, MinimumScopes: noScopes, MaximumScopes: dqlScopes},
		},
		"no file named": {
			command: "apply -f .",
			want:    Parsed{Verb: "apply", MinimumScopes: noScopes, MaximumScopes: anyScope},
		},
		"a file named -": {
			command: "apply -f ./-",
			want:    Parsed{Verb: "apply", InputFiles: []string{"-"}, MinimumScopes: noScopes, MaximumScopes: anyScope},
		},
		"an at-path to a file named -": {
			command: "exec api /platform/x -X POST -d @./-",
			want: Parsed{Verb: "exec", Resource: "api", InputFiles: []string{"-"},
				MinimumScopes: noScopes, MaximumScopes: anyScope},
		},
		"empty at-path": {
			command: "exec api /platform/x -X POST -d @",
			want:    Parsed{Verb: "exec", Resource: "api", MinimumScopes: noScopes, MaximumScopes: anyScope},
		},
		"local command": {
			command: "version",
			want:    Parsed{Verb: "version", MinimumScopes: noScopes, MaximumScopes: noScopes},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := Parse(context.Background(), ParseRequest{Command: tc.command})
			require.NoError(t, err)
			if tc.want.InputFiles == nil {
				tc.want.InputFiles = []string{}
			}
			if tc.want.OutputFiles == nil {
				tc.want.OutputFiles = []string{}
			}
			assert.Equal(t, &tc.want, got)
		})
	}
}

// TestParse_CheckScopesReadsNoInput: --check-scopes prints a verdict and does
// not run the command, so the command reads no input.
func TestParse_CheckScopesReadsNoInput(t *testing.T) {
	for _, command := range []string{
		"delete workflow wf-1 --check-scopes",
		"query --check-scopes",
		"apply -f - --check-scopes",
	} {
		t.Run(command, func(t *testing.T) {
			got, err := Parse(context.Background(), ParseRequest{Command: command})
			require.NoError(t, err)
			assert.False(t, got.RequiresInput)
		})
	}
}

// TestParse_ScopeBounds holds the reference values of the scope bounds.
func TestParse_ScopeBounds(t *testing.T) {
	for command, want := range map[string][2][]string{
		"help":                              {noScopes, noScopes},
		"commands --full":                   {noScopes, noScopes},
		"get workflows --help":              {noScopes, noScopes},
		"exec function x -h":                {noScopes, noScopes},
		"get notebooks":                     {{"document:documents:read"}, {"document:documents:read"}},
		"get notebooks --admin-access":      {{"document:documents:admin", "document:documents:read"}, {"document:documents:admin", "document:documents:read"}},
		"query 'fetch logs'":                {noScopes, dqlScopes},
		"verify query 'fetch logs'":         {noScopes, dqlScopes},
		"wait query 'fetch logs' --for any": {noScopes, dqlScopes},
		"exec function my-app/fn":           {{"app-engine:functions:run"}, anyScope},
		"exec analyzer dt.loganalysis.LogPatternExtractor -f in.json":                                     {{"davis:analyzers:execute"}, anyScope},
		"exec analyzer dt.statistics.GenericForecastAnalyzer --query 'timeseries avg(dt.host.cpu.usage)'": {{"davis:analyzers:execute"}, anyScope},
		"exec slo slo-1":        {{"slo:slos:read", "slo:objective-templates:read"}, {"slo:objective-templates:read", "slo:slos:read", "storage:*:read"}},
		"exec api /platform/x":  {noScopes, anyScope},
		"apply -f x.yaml":       {noScopes, anyScope},
		"delete dashboard d":    {{"document:documents:delete"}, {"document:documents:delete", "document:documents:read"}},
		"restore dashboard d 3": {{"document:documents:write"}, {"document:documents:read", "document:documents:write"}},
		"create aws connection --name n --roleArn arn:x": {noScopes, {
			"extensions:configurations:read", "extensions:configurations:write", "settings:objects:read", "settings:objects:write"}},
		"describe anomaly-detector a":        {{"settings:objects:read"}, {"settings:objects:read", "storage:*:read"}},
		"exec copilot nl2dql 'errors today'": {noScopes, {"davis-copilot:conversations:execute"}},
		"exec copilot 'hello'":               {{"davis-copilot:conversations:execute"}, {"davis-copilot:conversations:execute"}},
	} {
		t.Run(command, func(t *testing.T) {
			got, err := Parse(context.Background(), ParseRequest{Command: command, MinStability: "experimental"})
			require.NoError(t, err)
			assert.Equal(t, want[0], got.MinimumScopes, "minimum")
			assert.Equal(t, want[1], got.MaximumScopes, "maximum")
		})
	}
}

// TestParse_Confirmation: a command that asks for confirmation reads the
// request's input, unless the command line skips the question.
func TestParse_Confirmation(t *testing.T) {
	for command, want := range map[string]bool{
		"delete workflow wf-1":               true,
		"delete workflow wf-1 --yes":         false,
		"delete workflow wf-1 -y":            false,
		"delete workflow wf-1 --yes=false":   true,
		"delete workflow wf-1 --plain":       false,
		"delete workflow wf-1 --agent":       false,
		"delete workflow wf-1 --dry-run":     false,
		"--dry-run delete workflow wf-1":     false,
		"delete bucket b --confirm b":        false,
		"restore dashboard d 3":              true,
		"restore dashboard d 3 --force":      false,
		"restore trash a":                    true,
		"restore trash a b":                  false, // asks for one document only
		"delete scheduling-rule r --dry-run": true,  // asks before its dry run returns
		"get workflows":                      false,
	} {
		t.Run(command, func(t *testing.T) {
			got, err := Parse(context.Background(), ParseRequest{Command: command, MinStability: "experimental"})
			require.NoError(t, err)
			assert.Equal(t, want, got.RequiresInput)
		})
	}
}

// TestParse_UsageErrors holds a refused command line to a *UsageError: the
// request's surface (blocked commands, profile, floor) applies as it does to
// Execute, and so does cobra's validation.
func TestParse_UsageErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		req  ParseRequest
		code string
		exit int
	}{
		"unsupported in a service": {ParseRequest{Command: "config view"}, "unsupported_in_service", 2},
		"outside the profile":      {ParseRequest{Command: "get buckets", Profile: "query"}, "profile_blocked", 2},
		"profile through Env": {
			ParseRequest{Command: "get buckets", Env: map[string]string{"DTCTL_PROFILE": "query"}},
			"profile_blocked", 2,
		},
		"below the floor": {ParseRequest{Command: "get breakpoints"}, "stability_blocked", 2},
		"unknown flag":    {ParseRequest{Command: "get workflows --bogus"}, "unknown_command", 2},
		// Cobra's required-flag error is untyped, so Execute reports it as a
		// generic failure (exit 1) and so does Parse.
		"required flag": {ParseRequest{Command: "apply"}, "error", 1},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(context.Background(), tc.req)
			var ue *UsageError
			require.ErrorAs(t, err, &ue)
			assert.Equal(t, tc.code, ue.Code)
			assert.Equal(t, tc.exit, ue.ExitCode)
		})
	}

	// An exception admits the below-floor command, as it does for Execute.
	_, err := Parse(context.Background(), ParseRequest{
		Command:             "get breakpoints",
		StabilityExceptions: []string{"get breakpoints"},
	})
	require.NoError(t, err)
}

// TestParse_RequestValidation: a malformed request is a plain error, never a
// *UsageError, so a host can tell its own bug from the caller's command line.
func TestParse_RequestValidation(t *testing.T) {
	for name, req := range map[string]ParseRequest{
		"no command":           {},
		"unparsable quote":     {Command: `create bucket -f "unclosed`},
		"invalid safety level": {Command: "get buckets", SafetyLevel: "anything-goes"},
		"development floor":    {Command: "get buckets", MinStability: "development"},
		"invalid exception":    {Command: "get buckets", StabilityExceptions: []string{"query --decode-snapshots --extra"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(context.Background(), req)
			require.Error(t, err)
			var ue *UsageError
			assert.NotErrorAs(t, err, &ue)
		})
	}
}

// TestEngineParse_TakesNoSlot: a parse is not refused or delayed because
// executions hold every slot.
func TestEngineParse_TakesNoSlot(t *testing.T) {
	e := New(DefaultLimits())
	e.adm.slot <- struct{}{} // the only slot, held
	defer func() { <-e.adm.slot }()

	got, err := e.Parse(context.Background(), ParseRequest{Command: "get workflows"})
	require.NoError(t, err)
	assert.Equal(t, "workflows", got.Resource)
}

func BenchmarkParse(b *testing.B) {
	req := ParseRequest{Command: "apply -f dashboard.yaml --write-id"}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}
