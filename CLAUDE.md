# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ecspresso is a deployment tool for Amazon ECS written in Go. It manages ECS services through configuration files (YAML, JSON, or Jsonnet) that define task definitions and service definitions.

## Git Workflow

- Default branch: `v2`
- Do not commit directly to `v2`. Always create a feature branch first.
- When changing `action.yml`, force-push the same commit to the `v2-action-testing` branch as well. CI for the GitHub Action runs against that branch, so pushing there is how `action.yml` changes get exercised end-to-end.

## Build Commands

```bash
# Build the binary
make

# Install to GOPATH/bin
make install

# Run all tests
make test

# Run a single test
go test -v -run TestFunctionName ./...

# Format code (run before committing)
go fmt ./...

# Build release packages
make packages
```

## Architecture

### Core Components

- **App** (`ecspresso.go`): Main application struct that holds AWS clients (ECS, CodeDeploy, CloudWatch Logs, IAM, etc.) and orchestrates operations
- **Config** (`config.go`): Configuration loading and validation. Supports YAML, JSON, and Jsonnet formats with template functions
- **CLI** (`cli.go`, `cli_parse.go`): Command-line interface using Kong. Each subcommand (deploy, run, diff, etc.) has its own option struct and handler

### Command Structure

Commands are defined as option structs in their respective files:
- `deploy.go` - Deploy/update ECS services (rolling update, Blue/Green via ECS or CodeDeploy)
- `run.go` - Run standalone ECS tasks
- `rollback.go` - Rollback to previous task definition
- `diff.go` - Show differences between local and remote definitions
- `verify.go` - Validate configurations and AWS resources
- `init.go` - Generate config from existing ECS service

### Template System

Definition files support Go template syntax via `github.com/kayac/go-config`:
- `{{ env "VAR" "default" }}` - Environment variable with default
- `{{ must_env "VAR" }}` - Required environment variable
- `{{ tfstate "resource.attr" }}` - Terraform state lookup (plugin)

### Plugin System (`plugin.go`)

Plugins extend template functions and Jsonnet native functions:
- `tfstate` - Terraform state lookups
- `cloudformation` - CloudFormation output/export lookups
- `ssm` - SSM Parameter Store lookups
- `secretsmanager` - Secrets Manager ARN resolution
- `external` - Execute external commands

### AWS SDK Usage

Uses AWS SDK for Go v2. Key clients are initialized in `New()` function based on configuration.

### Key Types

- `Service` - Wraps `types.Service` with additional fields (ServiceConnectConfiguration, VolumeConfigurations, VpcLatticeConfigurations)
- `TaskDefinitionInput` - Wraps `ecs.RegisterTaskDefinitionInput`
- `TaskDefinition` - Wraps `types.TaskDefinition`

## Testing

Tests use the standard Go testing framework. Test files are colocated with implementation files (`*_test.go`).

```bash
# Run specific test file
go test -v ./... -run TestConfig

# Run with race detection
go test -race ./...
```

- Use `github.com/google/go-cmp/cmp` for value comparisons in tests
  ```go
  if diff := cmp.Diff(want, got); diff != "" {
      t.Errorf("mismatch (-want +got):\n%s", diff)
  }
  ```
- Use `t.Context()` instead of `context.Background()` in tests

## Documentation

- When adding or changing features, always check README.md for inconsistencies and update it if needed (e.g. command examples, flag lists, log output examples)
- When adding or modifying sections in README.md, always update the Table of Contents at the top of the file to match

## Code Style

- Use `go fmt ./...` before committing
- Error messages should be lowercase and not end with punctuation
- Use `fmt.Errorf("failed to X: %w", err)` for error wrapping
- Logging uses `slog` via helper methods (LogInfo, LogWarn, LogDebug)
