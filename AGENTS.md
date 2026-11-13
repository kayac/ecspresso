# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Project Overview

ecspresso is a deployment tool for Amazon ECS written in Go. It manages ECS services through configuration files (YAML, JSON, or Jsonnet) that define task definitions and service definitions.

## Git Workflow

- This is the `v3` branch, where ecspresso v3 is developed. The v2 line is maintained on the `v2` branch.
- Do not commit directly to `v3`. Always create a feature branch first.

## GitHub Actions

- The GitHub Action lives in [kayac/ecspresso-action](https://github.com/kayac/ecspresso-action). `action.yml` in this repository is a deprecated forwarder to it. Keep its inputs in sync with kayac/ecspresso-action.
- All workflows under `.github/workflows/` are onboarded to the [workflow dependency lockfile](https://docs.github.com/en/actions/concepts/security/workflow-dependency-lockfiles) (`.github/workflows/actions.lock`). Reference actions by version tag (e.g. `actions/checkout@v6.0.2`) and run `gh actions-lock` to update the lockfile. Do not pin them by SHA, and do not run `pinact` on them.
- Workflows reference `action.yml` as `uses: $/`, so `gh actions-lock` also manages the action referenced in `action.yml` (`kayac/ecspresso-action@vX.Y.Z`). Releases of kayac/ecspresso-action are immutable, so the full version tag cannot be moved.

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

### End-to-end test

`tests/e2e.yml` is a [runn](https://github.com/k1LoW/runn) runbook of a service lifecycle scenario (deploy, diff, rollback, run, init, delete) against an [AWS Emulator](https://github.com/aws/aws-emulator) endpoint. Its fixtures are in `tests/e2e/`. CI runs it by `.github/workflows/e2e.yml` when the Go sources or the fixtures change.

To run it locally, start the emulator (`aws-emulator --telemetry off --auth off`), set `AWS_CA_BUNDLE` to its `ca.pem`, and run:

```bash
make && runn run tests/e2e.yml --scopes run:exec
```

- Every command in the runbook sets `AWS_ENDPOINT_URL` and dummy credentials, so that it never runs against real AWS. Keep it so when adding a step.
- The AWS CLI is used only to prepare the resources (`tests/e2e/setup.sh`). Assert the behavior with ecspresso commands.
- Each command that waits for a deployment takes 10 seconds or more. Add a step only for what needs the real API behavior (requests the service accepts, definitions read back, state transitions). Test argument handling and output formatting in unit tests.
- The emulator and runn versions are pinned in the workflow and are not updated automatically.

### Testing requests to and responses from AWS APIs

Do not start a fake HTTP server (e.g. `httptest.Server`) to test what is sent to AWS APIs. Add a middleware to the SDK client with `APIOptions` (`ecs.Options.APIOptions` or `config.WithAPIOptions`) instead. No request is sent when the middleware returns without calling the next handler.

- To return a typed output without inspecting the wire format, add a middleware to the head of the Finalize step (`stack.Finalize.Add(..., middleware.Before)`) and return the output as `middleware.FinalizeOutput.Result`. See `SDKTestingMiddleware` in `middleware_test.go`.
- To check the request body the SDK sends, or to let the SDK deserialize a canned response body, add a middleware to the tail of the Deserialize step (`stack.Deserialize.Add(..., middleware.After)`). Read the serialized body from `in.Request.(*smithyhttp.Request).GetStream()`, and return the response as `middleware.DeserializeOutput.RawResponse` (a `*smithyhttp.Response`). See `newHookDetailsTestClient` in `hookdetails_test.go`.
  - Use this when the serialization itself matters, e.g. for a field of `document.Interface` which `encoding/json` cannot handle.
  - The operation is in the `X-Amz-Target` request header (e.g. `AmazonEC2ContainerServiceV20141113.UpdateService`).
  - The client needs `Region` and `Credentials` (`aws.AnonymousCredentials{}` is enough), because the middlewares before it (endpoint resolution, signing) still run.

## Scope of `verify`

`verify` checks what the AWS APIs do not report when `deploy` calls them. `deploy` succeeds with such a problem, and it shows up later in a way that is hard to trace (e.g. tasks fail to start).

- Add checks on the facts of the user's environment: a resource referenced by the definitions exists and is accessible (images, IAM roles, secrets, log groups, target groups, Lambda functions, etc.).
- Do not add checks that copy the validation rules of the AWS APIs: allowed values, required fields, constraints on combinations of settings, and default values. The API rejects such a request immediately with a message that states the cause, so the check saves little. The rules belong to AWS and change without notice, so each copy has to be followed forever, and a stale one rejects a valid definition.
- "It fails only after the task definition is registered" is not a reason to add a check of the latter kind.
- Some long-standing checks on the definitions remain (e.g. `networkConfiguration.awsvpcConfiguration` is required for the `awsvpc` network mode, `name` and `valueFrom` are required for `secrets`). They call no API, and what they check is self-evident and unlikely to change on the AWS side. They are not a precedent for copying the rules of a new feature. `verifyEarlySuccessCriteria` and `verifyGatewayConnectorsDefinition` were removed in v3 for this reason.

## Documentation

- When adding or changing features, always check README.md for inconsistencies and update it if needed (e.g. command examples, flag lists, log output examples)
- When adding or modifying sections in README.md, always update the Table of Contents at the top of the file to match

## Code Style

- Use `go fmt ./...` before committing
- Error messages should be lowercase and not end with punctuation
- Use `fmt.Errorf("failed to X: %w", err)` for error wrapping
- Logging uses `slog` via helper methods (LogInfo, LogWarn, LogDebug)
