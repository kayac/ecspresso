---
name: ecspresso
description: ECS deployment tool - deploy, manage, and troubleshoot ECS services
license: MIT
compatibility:
  - claude
  - codex
  - agents
allowed_tools:
  - Bash
  - Read
---

# ecspresso Skill for LLM Agents

## Overview

ecspresso is a deployment tool for Amazon ECS. You can use it to deploy, manage, and troubleshoot ECS services through configuration files.

## Learning about ecspresso

Use the `docs` subcommand to look up how ecspresso works. This command requires no AWS credentials or config file.

```bash
# Browse the table of contents to find relevant sections
ecspresso docs --index --json

# Search for a topic by keyword
ecspresso docs --search "fargate" --json

# Read the full documentation
ecspresso docs --json

# List available articles (README and migration guides)
ecspresso docs --list --json

# Read a specific article
ecspresso docs --article v2-v3 --json
```

Always use `--json` for structured output that is easier to parse.

### Version upgrades

When upgrading ecspresso across major versions (v1 to v2, or v2 to v3), always read the embedded migration guide first to learn about breaking changes:

```bash
# Migration guide from v1 to v2
ecspresso docs --article v1-v2 --json

# Migration guide from v2 to v3
ecspresso docs --article v2-v3 --json
```

## Common workflows

### Check current state before making changes

```bash
# Show service status (running tasks, deployments, events)
ecspresso status --config ecspresso.yml

# Show differences between local definitions and running service
ecspresso diff --config ecspresso.yml

# Verify that resources referenced in definitions exist and are valid
ecspresso verify --config ecspresso.yml
```

### Deploy

```bash
# Preview what will change (dry-run)
ecspresso deploy --config ecspresso.yml --dry-run

# Deploy the service
ecspresso deploy --config ecspresso.yml

# Deploy and wait until stable
ecspresso deploy --config ecspresso.yml --wait-until stable
```

### Rollback

```bash
# Rollback to the previous task definition
ecspresso rollback --config ecspresso.yml

# Rollback with dry-run
ecspresso rollback --config ecspresso.yml --dry-run
```

### Scale

```bash
# Scale to a specific number of tasks
ecspresso scale --config ecspresso.yml --tasks 5
```

### Run a one-off task

```bash
# Run a standalone task using the service's task definition
ecspresso run --config ecspresso.yml

# Run and watch logs
ecspresso run --config ecspresso.yml --watch-container app
```

### Inspect tasks

```bash
# List running tasks as JSON
ecspresso tasks --config ecspresso.yml --output json
```

### Render configuration for inspection

```bash
# Render resolved task definition (with template variables expanded)
ecspresso render --config ecspresso.yml taskdef

# Render resolved service definition
ecspresso render --config ecspresso.yml servicedef
```

## Configuration structure

ecspresso uses a config file (default: `ecspresso.yml`) that references a task definition file and optionally a service definition file.

```
ecspresso.yml          # Main config: region, cluster, service, file paths
ecs-task-def.json      # ECS task definition (JSON, YAML, or Jsonnet)
ecs-service-def.json   # ECS service definition (JSON, YAML, or Jsonnet)
```

Definition files support template syntax: `{{ env "VAR" "default" }}`, `{{ must_env "VAR" }}`, and plugin functions like `{{ tfstate "resource.attr" }}`.

### Jsonnet is recommended over JSON/YAML

When creating or modifying definition files, prefer the Jsonnet (`.jsonnet`) format over JSON or YAML. Jsonnet has several advantages:

- **Comments**: Jsonnet supports `//` and `/* */` comments. JSON does not allow comments at all.
- **Trailing commas**: No need to worry about trailing comma errors.
- **Variables and functions**: You can define local variables, reuse values, and compute derived values with `std.parseInt()`, `std.parseJson()`, etc.
- **No template syntax conflicts**: In JSON/YAML, Go template syntax `{{ }}` can cause readability issues. In Jsonnet, use native functions like `std.native('env')('VAR', 'default')` instead, which are cleaner and can return non-string types.
- **No separate install needed**: ecspresso bundles the Jsonnet library.

Example Jsonnet config:

```jsonnet
// ecspresso.jsonnet
{
  region: 'ap-northeast-1',
  cluster: 'default',
  service: 'myservice',
  service_definition: 'ecs-service-def.jsonnet',
  task_definition: 'ecs-task-def.jsonnet',
}
```

Example Jsonnet task definition:

```jsonnet
// ecs-task-def.jsonnet
local env = std.native('env');
local must_env = std.native('must_env');
local tfstate = std.native('tfstate');
{
  family: 'myservice',
  cpu: '256',
  memory: '512',
  networkMode: 'awsvpc',
  requiresCompatibilities: ['FARGATE'],
  executionRoleArn: tfstate('aws_iam_role.ecs_execution.arn'),
  containerDefinitions: [
    {
      name: 'app',
      image: must_env('APP_IMAGE'),
      cpu: 0,
      portMappings: [{ containerPort: 8080 }],
      environment: [
        { name: 'ENV', value: env('ENV', 'production') },
      ],
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-group': '/ecs/myservice',
          'awslogs-region': 'ap-northeast-1',
          'awslogs-stream-prefix': 'app',
        },
      },
    },
  ],
}
```

Use `ecspresso init --jsonnet` to generate Jsonnet files from an existing service. For more details, run `ecspresso docs --search "jsonnet" --json`.

## Tips

- Always run `ecspresso diff` before `ecspresso deploy` to understand what will change.
- Use `--dry-run` on destructive operations (`deploy`, `rollback`, `delete`, `scale`) to preview the action.
- The `verify` command checks IAM roles, container images, secrets, and log groups referenced in definitions.
- When you need to learn more about a specific feature, use `ecspresso docs --search "<keyword>" --json` to find the relevant documentation section.
- When upgrading ecspresso to a new major version, read the migration guide via `ecspresso docs --article <version>-<next-version> --json` (e.g. `v2-v3`) before changing any configuration.
