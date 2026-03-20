---
layout: docs
title: App Engine
---

Dynatrace App Engine lets you extend the platform with custom and built-in applications. dtctl provides commands to list, inspect, and delete apps, as well as discover and execute app functions and intents.

## Listing and Viewing Apps

```bash
# List all installed apps
dtctl get apps

# Describe a specific app
dtctl describe app app-123
```

## App Functions

App functions are server-side endpoints exposed by Dynatrace apps. You can discover, inspect, and execute them directly from the CLI.

### Discover Functions

```bash
# List all available functions across all apps
dtctl get functions

# List functions for a specific app with extra detail
dtctl get functions --app dynatrace.automations -o wide
```

### Describe a Function

```bash
# View function details including parameters and schema
dtctl describe function dynatrace.automations/execute-dql-query
```

### Execute a Function

```bash
# Execute a function with a JSON payload
dtctl exec function dynatrace.automations/execute-dql-query \
  --method POST \
  --payload '{"query":"fetch logs | limit 5"}'
```

**Payload discovery:** If you're unsure what fields a function expects, try executing it with an empty payload (`--payload '{}'`). The error response will typically list the required fields and their types.

### Common Functions

| App | Function | Description |
|-----|----------|-------------|
| `dynatrace.automations` | `execute-dql-query` | Run a DQL query |
| `dynatrace.automations` | `send-email` | Send an email notification |
| `dynatrace.automations` | `send-slack-message` | Post a message to Slack |
| `dynatrace.automations` | `create-jira-issue` | Create a Jira issue |

## App Intents

Intents provide a deep-linking mechanism to navigate into specific app views with contextual data.

### Discover Intents

```bash
# List all registered intents
dtctl get intents

# Describe a specific intent to see its parameters
dtctl describe intent dynatrace.distributedtracing/view-trace
```

### Find Matching Intents

```bash
# Find intents that accept a given set of data fields
dtctl find intents --data trace_id=abc123
```

### Generate URLs

```bash
# Generate a deep-link URL and open it in the browser
dtctl open intent dynatrace.distributedtracing/view-trace \
  --data trace_id=abc123 \
  --browser
```

**Use cases:** Deep linking from alert notifications to the relevant trace or dashboard, scripted navigation for runbooks, and building custom integrations that open Dynatrace views with pre-filled context.

## Deleting Apps

```bash
# Delete an app by ID
dtctl delete app app-123
```

## Required Scopes

| Scope | Used By |
|-------|---------|
| `app-engine:apps:run` | Listing, describing, and deleting apps |
| `app-engine:functions:run` | Executing app functions |
