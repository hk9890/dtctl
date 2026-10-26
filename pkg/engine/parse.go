package engine

import (
	"context"
	"errors"

	"github.com/dynatrace-oss/dtctl/cmd"
	"github.com/dynatrace-oss/dtctl/pkg/config"
)

// ParseRequest is one command line to analyse. Each field means what it means
// on Request, so a host that restricts Execute gets the same answer from
// Parse. It has no EnvironmentURL or Token: Parse needs neither, and requiring
// them would make a host mint a credential for a call that uses none.
//
// Experimental: Parse and its types may change until a second host uses them.
type ParseRequest struct {
	Command string   // as Request.Command
	Argv    []string // as Request.Argv; takes precedence over Command

	SafetyLevel         string
	Profile             string
	MinStability        string
	StabilityExceptions []string
	NoDeprecated        bool
	// Env is as Request.Env: Profile, when set, overrides an entry for
	// DTCTL_PROFILE.
	Env map[string]string
}

// Parsed is what a command line means, without running it.
//
// Experimental, as Parse.
type Parsed struct {
	Verb     string // "get", "apply", "exec", ...
	Resource string // "" when the verb takes none (apply)

	// InputFiles and OutputFiles are the files the command reads and writes,
	// named the way Request.Files and Result.Files name them (vfs.Key), sorted,
	// de-duplicated and non-nil. A "-" value is Request.Stdin, not a file; a
	// path to a file named "-" (./-) is the key "-".
	InputFiles  []string
	OutputFiles []string
	// RequiresInput is true when the command reads Request.Stdin: a "-" file
	// argument, "-d @-", a command such as query given no argument and no
	// file, or a command that asks for confirmation (delete, restore) without
	// --plain, --agent or the flag that skips the question (--yes, --force).
	// It is false under --check-scopes, which does not run the command.
	RequiresInput bool

	// MinimumScopes are the exact scopes the command cannot run without: a
	// token that lacks one fails. [] means the command needs no particular
	// scope.
	//
	// MaximumScopes are the scopes the command can use, as patterns: "*"
	// matches any text, including none, and every other character matches
	// itself. A host restricts the command's token to the scopes the user
	// holds that match one. [] means the command has no platform access (it is
	// local, or the command line asks for help); ["*"] means it can use any
	// scope the user holds, which is also the answer for a command whose
	// scopes dtctl does not know.
	//
	// Both are always non-nil, every minimum scope matches a maximum pattern,
	// and both come from fixed lists in dtctl: never from a DQL statement, a
	// payload or an input file.
	MinimumScopes []string
	MaximumScopes []string
}

// UsageError is a command line that Execute would refuse before running the
// command. Code, Message and Suggestions are the error of the --agent envelope
// Execute prints for it, and ExitCode is the exit code it returns (2 for a
// usage error).
type UsageError struct {
	Code        string
	Message     string
	Suggestions []string
	ExitCode    int
}

func (e *UsageError) Error() string { return e.Message }

// Parse analyses a command line the way Execute would run it, and stops before
// the command runs. It reads no file, calls no platform API and needs no token.
//
// Parse and Execute use the same command tree, the same surface masks
// (profile, blocked commands, stability floor, deprecation) and the same
// validation, so a command line Execute would refuse returns a *UsageError
// carrying Execute's error. Validation that runs inside a command body is seen
// by Execute only: a nil error means the command line is well-formed, not that
// Execute will accept it. Any other error is an engine fault.
//
// Safe to call concurrently. It does not count toward any Engine's admission
// budget. It shares the invocation lock with Execute, though: while a
// serialized Execute runs (an Engine built without WithConcurrentExecution, or
// the package-level Execute), Parse waits for it to finish, and ctx bounds the
// wait.
//
// Experimental: Parse and its types may change until a second host uses them.
func Parse(ctx context.Context, req ParseRequest) (*Parsed, error) {
	argv, err := commandArgv(req.Command, req.Argv)
	if err != nil {
		return nil, err
	}
	if err := validateSurface(req.SafetyLevel, req.MinStability, req.StabilityExceptions); err != nil {
		return nil, err
	}
	env := make(map[string]string, len(req.Env)+1)
	for k, v := range req.Env {
		env[k] = v
	}
	if req.Profile != "" {
		env[config.ProfileEnvVar] = req.Profile
	}
	res, err := cmd.Parse(argv, cmd.ParseOptions{
		SafetyLevel:         config.SafetyLevel(req.SafetyLevel),
		MinStability:        config.StabilityLevel(req.MinStability),
		StabilityExceptions: req.StabilityExceptions,
		NoDeprecated:        req.NoDeprecated,
		Env:                 env,
		BlockedCommands:     unsupportedCommands,
		Context:             ctx,
	})
	var pe *cmd.ParseError
	if errors.As(err, &pe) {
		return nil, &UsageError{Code: pe.Code, Message: pe.Message, Suggestions: pe.Suggestions, ExitCode: pe.ExitCode}
	}
	if err != nil {
		return nil, err
	}
	return &Parsed{
		Verb:          res.Verb,
		Resource:      res.Resource,
		InputFiles:    res.InputFiles,
		OutputFiles:   res.OutputFiles,
		RequiresInput: res.RequiresInput,
		MinimumScopes: res.MinimumScopes,
		MaximumScopes: res.MaximumScopes,
	}, nil
}

// Parse analyses a command line with the rules the package-level Parse
// applies. It takes no execution slot: it does no I/O, and a host must not see
// a parse refused because slow executions hold every slot. On an Engine built
// without WithConcurrentExecution it still waits for a running Execute, as the
// package-level Parse documents.
func (e *Engine) Parse(ctx context.Context, req ParseRequest) (*Parsed, error) {
	return Parse(ctx, req)
}
