---
layout: docs
title: Configuration
---

## Authentication

### OAuth Login (Recommended)

Browser-based SSO login with automatic token refresh:

```bash
dtctl auth login --context my-env --environment "https://abc12345.apps.dynatrace.com"
```

Tokens are stored securely in your OS keyring. To log out:

```bash
dtctl auth logout
```

### Token-Based Auth

For CI/CD or headless environments, use a platform API token:

```bash
dtctl config set-context my-env \
  --environment "https://abc12345.apps.dynatrace.com" \
  --token-ref my-token

dtctl config set-credentials my-token \
  --token "dt0s16.XXXXXXXX.YYYYYYYY"
```

### Creating a Platform Token

1. In Dynatrace, navigate to **Identity & Access Management > Access Tokens**
2. Select **Generate new token** and choose **Platform token**
3. Add the required scopes for your use case
4. Copy the token immediately -- it's only shown once

See the [Dynatrace Platform Tokens documentation](https://docs.dynatrace.com/docs/manage/identity-access-management/access-tokens-and-oauth-clients/platform-tokens) for detailed instructions.

### Current User Identity

Check who you're authenticated as:

```bash
dtctl auth whoami
```

Use `dtctl auth whoami -o json` for machine-readable output, or `--id-only` to get just the user ID.

## Multiple Environments

### Create Contexts

```bash
# Development
dtctl config set-context dev \
  --environment "https://dev.apps.dynatrace.com" \
  --token-ref dev-token \
  --safety-level dangerously-unrestricted

# Production (read-only)
dtctl config set-context prod \
  --environment "https://prod.apps.dynatrace.com" \
  --token-ref prod-token \
  --safety-level readonly
```

### Switch Contexts

```bash
dtctl config use-context dev

# Or use the shortcut:
dtctl ctx dev

# List all contexts
dtctl ctx
```

### One-Time Context Override

Run a single command against a different context without switching:

```bash
dtctl get workflows --context prod
```

## Per-Project Configuration

Create a `.dtctl.yaml` in your project root for team or CI/CD configuration:

```bash
dtctl config init
```

This generates a template with environment variable placeholders:

```yaml
apiVersion: dtctl.io/v1
kind: Config
current-context: production
contexts:
  - name: production
    context:
      environment: ${DT_ENVIRONMENT_URL}
      token-ref: my-token
      safety-level: readwrite-all
tokens:
  - name: my-token
    token: ${DT_API_TOKEN}
```

Commit the file to version control without secrets -- each developer or CI system provides values via environment variables.

### Config Search Order

1. `--config` flag (explicit path)
2. `.dtctl.yaml` in the current directory or any parent (walks up to root)
3. Global config (`~/.config/dtctl/config`)

## Safety Levels

Safety levels provide **client-side** protection against accidental destructive operations:

| Level | Description |
|-------|-------------|
| `readonly` | No modifications allowed |
| `readwrite-mine` | Modify your own resources only |
| `readwrite-all` | Modify all resources (default) |
| `dangerously-unrestricted` | All operations including bucket deletion |

```bash
dtctl config set-context prod \
  --environment "https://prod.apps.dynatrace.com" \
  --token-ref prod-token \
  --safety-level readonly
```

View context details including safety level:

```bash
dtctl config describe-context prod
```

Safety levels are client-side only. For actual security, configure your API tokens with minimum required scopes.

## Command Aliases

Create shortcuts for frequently used commands.

### Simple Aliases

```bash
dtctl alias set wf "get workflows"
dtctl wf
# Expands to: dtctl get workflows
```

### Parameterized Aliases

Use `$1`-`$9` for positional parameters:

```bash
dtctl alias set logs-errors "query 'fetch logs | filter status=\$1 | limit 100'"
dtctl logs-errors ERROR
# Expands to: dtctl query 'fetch logs | filter status=ERROR | limit 100'
```

### Shell Aliases

Prefix with `!` to execute through the system shell (enables pipes and external tools):

```bash
dtctl alias set wf-names "!dtctl get workflows -o json | jq -r '.workflows[].title'"
dtctl wf-names
```

### Import and Export

Share aliases with your team:

```bash
dtctl alias export -f team-aliases.yaml
dtctl alias import -f team-aliases.yaml
```

### Managing Aliases

```bash
dtctl alias list         # List all aliases
dtctl alias delete wf    # Delete an alias
```

### Alias Safety

Aliases cannot shadow built-in commands:

```bash
dtctl alias set get "query 'fetch logs'"
# Error: alias name "get" conflicts with built-in command
```

---

Previous: [Quick Start]({{ '/docs/quick-start/' | relative_url }})
