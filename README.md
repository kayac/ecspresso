# ecspresso

ecspresso is a deployment tool for Amazon ECS.

(pronounced the same as "espresso" :coffee:)

ecspresso allows you to manage ECS services and task definitions as code in JSON, YAML, or Jsonnet files, enabling version control and infrastructure as code practices. You can generate configuration files from existing ECS services using `ecspresso init`, making it easy to start managing your infrastructure.

Definition files support template syntax and Jsonnet native functions to embed environment variables, Terraform state values, SSM parameters, and Secrets Manager ARNs. ecspresso supports multiple deployment strategies including rolling updates and Blue/Green deployments with both ECS native and CodeDeploy controllers. Before deploying, you can preview changes with `diff` and validate configurations with `verify`. If something goes wrong, `rollback` helps you safely revert to a previous state.

ecspresso also supports ECS Express mode for simplified deployments and provides a plugin system for extending functionality.

## Table of Contents

- [Documents](#documents)
- [Install](#install)
- [Usage](#usage)
- [Quick Start](#quick-start)
- [Configuration file](#configuration-file)
  - [Plugin functions in top-level fields](#plugin-functions-in-top-level-fields)
- [Template syntax](#template-syntax)
- [Deployment](#example-of-deployment)
  - [Rolling deployment](#rolling-deployment)
  - [Blue/Green deployment (ECS)](#bluegreen-deployment-with-ecs-deployment-controller)
  - [Blue/Green deployment (CodeDeploy)](#bluegreen-deployment-with-aws-codedeploy)
- [Scale out/in](#scale-outin)
- [Rollback](#rollback)
- [Run task](#example-of-run-task)
- [Notes](#notes)
  - [Version constraint](#version-constraint)
  - [Manage Application Auto Scaling](#manage-application-auto-scaling)
  - [Jsonnet support](#use-jsonnet-instead-of-json-and-yaml)
  - [Deploy to Fargate](#deploy-to-fargate)
  - [Fargate Spot support](#fargate-spot-support)
  - [ECS Service Connect support](#ecs-service-connect-support)
  - [EBS Volume support](#ebs-volume-support)
  - [S3 Files volume support](#s3-files-volume-support)
  - [VPC Lattice support](#vpc-lattice-support)
  - [High resolution CloudWatch metrics](#high-resolution-cloudwatch-metrics)
  - [Pause lifecycle hooks](#pause-lifecycle-hooks)
  - [ECS Express mode support](#ecs-express-mode-support)
  - [Diff and Verify](#how-to-check-diff-and-verify-servicetask-definitions-before-deploy)
  - [Manipulate ECS tasks](#manipulate-ecs-tasks)
  - [Show documentation](#show-documentation)
  - [LLM agent integration](#llm-agent-integration)
- [Plugins](#plugins)
  - [tfstate](#tfstate)
  - [CloudFormation](#cloudformation)
  - [SSM Parameter Store](#ssm-parameter-store-lookups)
  - [Secrets Manager](#resolve-secretsmanager-secret-arn)
  - [External commands](#execute-external-commands)
- [LICENSE](#license)

## Documents

- [Differences between v2 and v3](docs/v2-v3.md).
- [Differences between v1 and v2](docs/v1-v2.md).
- [ecspresso Advent Calendar 2020](https://adventar.org/calendars/5916) (Japanese)
- [ecspresso handbook](https://zenn.dev/fujiwara/books/ecspresso-handbook-v2) (Japanese)
- [Command Reference](https://zenn.dev/fujiwara/books/ecspresso-handbook-v2/viewer/reference) (Japanese)

## Install

### Homebrew (macOS and Linux)

```console
$ brew install kayac/tap/ecspresso
```

### asdf (macOS and Linux)

```console
$ asdf plugin add ecspresso
# or
$ asdf plugin add ecspresso https://github.com/kayac/asdf-ecspresso.git

$ asdf install ecspresso 2.3.0
$ asdf set -u ecspresso 2.3.0
```

### aqua (macOS and Linux)

[aqua](https://aquaproj.github.io/) is a CLI version manager.

```console
$ aqua g -i kayac/ecspresso
```

### Binary packages

[Releases](https://github.com/kayac/ecspresso/releases)

### Docker image

Docker images are available on GitHub Container Registry.

```console
$ docker pull ghcr.io/kayac/ecspresso:v2.7.0
```

The image is based on `gcr.io/distroless/static-debian12` and supports both `linux/amd64` and `linux/arm64` architectures.

```console
$ docker run --rm \
    -v ~/.aws:/root/.aws:ro \
    -v $(pwd):/work \
    -w /work \
    ghcr.io/kayac/ecspresso:v2.7.0 deploy --config ecspresso.yml
```

### CircleCI Orbs

https://circleci.com/orbs/registry/orb/fujiwara/ecspresso

```yaml
version: 2.1
orbs:
  ecspresso: fujiwara/ecspresso@2.0.4
jobs:
  install:
    steps:
      - checkout
      - ecspresso/install:
          version: v2.3.0 # or latest
          # version-file: .ecspresso-version
          os: linux # or windows or darwin
          arch: amd64 # or arm64
      - run:
          command: |
            ecspresso version
```

`version: latest` installs different versions of ecspresso for each Orb version.
- fujiwara/ecspresso@0.0.15
  - The latest release version (v2 and later)
- fujiwara/ecspresso@1.0.0
  - The latest version of v1.x
- fujiwara/ecspresso@2.0.3
  - The latest version of v2.x

Note: `version: latest` is not recommended as it may cause unexpected behavior when a new version of ecspresso is released.

Orb `fujiwara/ecspresso@2.0.2` supports `version-file: path/to/file`, which installs the ecspresso version specified in the file. This version number does not have a `v` prefix, For example, `2.0.0`.

### GitHub Actions

Action kayac/ecspresso@v2 installs an ecspresso binary for Linux(x86_64) into /usr/local/bin. This action installs ecspresso. When `args` input is provided, it runs `ecspresso` with the specified arguments after installation.

```yml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: kayac/ecspresso@v2
        with:
          version: v2.3.0 # or latest
          # version-file: .ecspresso-version
      - run: |
          ecspresso deploy --config ecspresso.yml
```

To use the latest version of ecspresso, pass the parameter "latest".

```yaml
      - uses: kayac/ecspresso@v2
        with:
          version: latest
```

`version: latest` installs different versions of ecspresso for each Action version.
- kayac/ecspresso@v1
  - The latest version of v1.x
- kayac/ecspresso@v2
  - The latest version of v2.x

Note: `version: latest` is not recommended as it may cause unexpected behavior when a new version of ecspresso is released.

Action `kayac/ecspresso@v2` supports `version-file: path/to/file`, which installs the ecspresso version specified in the file. This version number does not have a `v` prefix, For example `2.3.0`.

### Run ecspresso after installation

When the `args` input is provided, the action runs `ecspresso` with the specified arguments after installation.

```yaml
      - uses: kayac/ecspresso@v2
        with:
          version: latest
          args: deploy
        env:
          ECSPRESSO_CONFIG: ecspresso.yml
```

## Usage

```console
Usage: ecspresso <command>

Flags:
  -h, --help                      Show context-sensitive help.
      --envfile=ENVFILE,...       environment files ($ECSPRESSO_ENVFILE)
      --debug                     enable debug log ($ECSPRESSO_DEBUG)
      --ext-str=KEY=VALUE;...     external string values for Jsonnet ($ECSPRESSO_EXT_STR)
      --ext-code=KEY=VALUE;...    external code values for Jsonnet ($ECSPRESSO_EXT_CODE)
      --config="ecspresso.yml"    config file ($ECSPRESSO_CONFIG)
      --assume-role-arn=""        the ARN of the role to assume ($ECSPRESSO_ASSUME_ROLE_ARN)
      --timeout=TIMEOUT           timeout. Override in a configuration file ($ECSPRESSO_TIMEOUT).
      --filter-command=STRING     filter command ($ECSPRESSO_FILTER_COMMAND)
      --[no-]color                enable colorized output ($ECSPRESSO_COLOR)
      --log-format="text"         log format (text, json) ($ECSPRESSO_LOG_FORMAT)

Commands:
  appspec
    output AppSpec YAML for CodeDeploy to STDOUT

  delete
    delete service

  deploy
    deploy service

  deregister
    deregister task definition

  diff
    show diff between task definition, service definition with current running
    service and task definition

  docs
    show documentation for ecspresso

  exec
    execute command on task

  init --service=SERVICE
    create configuration files from existing ECS service

  refresh
    refresh service. equivalent to deploy --skip-task-definition
    --force-new-deployment --no-update-service

  register
    register task definition

  render <targets>
    render config, service definition or task definition file to STDOUT

  revisions
    show revisions of task definitions

  rollback
    rollback service

  run
    run task

  scale
    scale service. equivalent to deploy --skip-task-definition
    --no-update-service

  status
    show status of service

  tasks
    list tasks that are in a service or having the same family

  verify
    verify resources in configurations

  wait
    wait until service stable

  version
    show version
```

For more options for sub-commands, See `ecspresso sub-command --help`.

### Log format

`--log-format` option controls the log output format. The default is `text`.

**text** format outputs human-readable logs. Attributes from `slog.With` (e.g. service/cluster) appear before the message, and per-call attributes appear after the message in `[key:value]` format.

```
2024-01-01T00:00:00.000+09:00 [INFO] [myService/default] deployment created on CodeDeploy [deployment_id:d-XXXXXXXXX] [url:https://...]
```

**json** format outputs structured JSON logs suitable for machine parsing. All attributes are output as individual JSON fields.

```json
{"time":"2024-01-01T00:00:00.000+09:00","level":"INFO","msg":"deployment created on CodeDeploy","cluster":"default","service":"myService","deployment_id":"d-XXXXXXXXX","url":"https://..."}
```

## Quick Start

ecspresso allows you to easily manage your existing/running ECS services by code.

Try `ecspresso init` for your ECS service with option `--region`, `--cluster` and `--service`.

```console
$ ecspresso init --region ap-northeast-1 --cluster default --service myservice --config ecspresso.yml
2024-01-01T00:00:00.000+09:00 [INFO] [myservice/default] saving service definition [path:ecs-service-def.json]
2024-01-01T00:00:00.000+09:00 [INFO] [myservice/default] saving task definition [path:ecs-task-def.json]
2024-01-01T00:00:00.000+09:00 [INFO] [myservice/default] saving config [path:ecspresso.yml]
```

Review the generated files: `ecspresso.yml`, `ecs-service-def.json`, and `ecs-task-def.json`.

Now you can deploy the service using ecspresso!

```console
$ ecspresso deploy --config ecspresso.yml
```

### Next step

ecspresso can read service and task definition files as a template. A typical use case is to replace the image's tag in the task definition file.

Modify ecs-task-def.json as below.

```diff
-  "image": "nginx:latest",
+  "image": "nginx:{{ must_env `IMAGE_TAG` }}",
```

Then, deploy the service with environment variable `IMAGE_TAG`.

```console
$ IMAGE_TAG=stable ecspresso deploy --config ecspresso.yml
```

For more information, refer to the [Configuration file](#configuration-file) and [Template syntax](#template-syntax) sections.

## Configuration file

A configuration file for ecspresso (YAML, JSON, or Jsonnet format).

```yaml
region: ap-northeast-1 # or AWS_REGION environment variable
cluster: default
service: myservice
task_definition: taskdef.json
timeout: 5m # default 10m
ignore:
  tags:
    - ecspresso:ignore # ignore tags of service and task definition
```

`ecspresso deploy` works as below.

- Register a new task definition from `task-definition` file (JSON or Jsonnet).
  - Replace ```{{ env `FOO` `bar` }}``` syntax in the JSON file with environment variable "FOO".
    - If "FOO" is not defined, replaced by "bar"
  - Replace ```{{ must_env `FOO` }}``` syntax in the JSON file wth environment variable "FOO".
    - If "FOO" is not defined, abort immediately.
- Update service tasks by the `service_definition` file (JSON or Jsonnet).
- Wait for the service to be stable.

Configuration files and task/service definition files are read by [go-config](https://github.com/kayac/go-config) which provides template functions `env`, `must_env` and `json_escape`.

### Plugin functions in top-level fields

Plugin-provided functions (`tfstate`, `cfn` / `cloudformation`, `ssm`, `secretsmanager`) can be used in top-level configuration fields such as `cluster`, `service`, and `region`, not only inside task / service definitions. This is typically useful when ECS clusters are managed by Terraform:

```yaml
region: ap-northeast-1
cluster: "{{ tfstate `aws_ecs_cluster.main.name` }}"
service: myservice
service_definition: ecs-service-def.json
task_definition: ecs-task-def.json
plugins:
  - name: tfstate
    config:
      path: terraform.tfstate
```

```jsonnet
local tfstate = std.native('tfstate');
{
  region: 'ap-northeast-1',
  cluster: tfstate('aws_ecs_cluster.main.name'),
  service: 'myservice',
  service_definition: 'ecs-service-def.jsonnet',
  task_definition: 'ecs-task-def.jsonnet',
  plugins: [
    { name: 'tfstate', config: { path: 'terraform.tfstate' } },
  ],
}
```

To make this work, ecspresso loads the configuration file in two passes:

1. Extract only the `plugins` (and `region`) section, then run plugin setup.
2. Re-read the whole file with plugin-provided template / Jsonnet functions registered.

Notes and limitations:

- The `plugins` block itself cannot reference plugin functions — that would be a chicken-and-egg loop. Use `env` / `must_env` there instead.
- If `region` itself is resolved from a plugin function that needs AWS access (e.g. `ssm`, `cfn`), the AWS client for plugin setup falls back to the `AWS_REGION` environment variable.

> [!IMPORTANT]
> **Breaking change in v3 (not compatible with v2)**: the YAML configuration file must be valid YAML on its own — i.e. before template rendering. In particular, template literals that begin a scalar must be quoted, because YAML parses an unquoted leading `{` as a flow-mapping opener.
>
> v2 template-rendered the whole file first and parsed YAML afterwards, which silently tolerated unquoted `{{ ... }}` at the start of a scalar. v3 reads the raw file with a YAML parser in pass 1 to extract `plugins`, so this no longer works.
>
> ```yaml
> # NG in v3 (worked in v2): YAML treats leading `{` as a flow-mapping opener
> region: {{ must_env `AWS_REGION` }}
>
> # OK: quote the scalar
> region: "{{ must_env `AWS_REGION` }}"
> ```
>
> JSON and Jsonnet configurations are unaffected. Existing v2 YAML configs that already quote their template literals (the common case) continue to work without changes.

## Template syntax

ecspresso uses the [text/template standard package in Go](https://pkg.go.dev/text/template) to render template files, and parses them as YAML or JSON.

When using Jsonnet, ecspresso first renders the Jsonnet files and then parses them as text/template. As a result, template functions can only render string values using "{{ ... }}", since the template function syntax {{ }} conflicts with Jsonnet syntax. To render non-string values, consider using [Jsonnet functions](#jsonnet-functions) instead.

By default, ecspresso provides the following as template functions.

### `env`

```go-template
"{{ env `NAME` `default value` }}"
```

This replaces the placeholder with the value of the environment variable NAME. If not set, it defaults to "default value".

### `must_env`

```go-template
"{{ must_env `NAME` }}"
```

This replaces the placeholder with the value of environment variable NAME. If not set, ecspresso will panic and stop forcefully.

Defining critical values with `must_env` helps prevent unintended deployments by ensuring these values are set before execution.

### `json_escape`

```go-template
"{{ must_env `JSON_VALUE` | json_escape }}"
```

This escapes values as JSON strings, which is useful for embedding values as strings that require escaping, such as quotes.

### Plugin provided template functions

ecspresso also adds some template functions via plugins. See the [Plugins](#plugins) section.

## Example of deployment

### Rolling deployment

`ecspresso deploy` performs rolling deployment by default.

```console
$ ecspresso deploy --config ecspresso.yml
2024-01-01T00:00:00.000+09:00 [INFO] [myService/default] Starting deploy
Service: myService
Cluster: default
TaskDefinition: myService:3
Deployments:
    PRIMARY myService:3 desired:1 pending:0 running:1
Events:
2024-01-01T00:00:00.000+09:00 [INFO] [myService/default] creating a new task definition [path:myTask.json]
2024-01-01T00:00:00.000+09:00 [INFO] [myService/default] registering a new task definition
2024-01-01T00:00:00.000+09:00 [INFO] [myService/default] task definition is registered [task_definition:myService:4]
2024-01-01T00:00:00.000+09:00 [INFO] [myService/default] updating service
2024-01-01T00:00:00.000+09:00 [INFO] [myService/default] Waiting for service stable...(it will take a few minutes)
2024-01-01T00:03:10.000+09:00 [INFO] [myService/default]  PRIMARY myService:4 desired:1 pending:0 running:1
2024-01-01T00:03:16.000+09:00 [INFO] [myService/default] Service is stable now. Completed!
```

`deploy` commands has many options to customize the deployment behavior. See `ecspresso deploy --help` for more details.

```
  --dry-run                              dry run
  --tasks=-1                             desired count of tasks
  --skip-task-definition                 skip register a new task definition
  --revision=0                           revision of the task definition to run when --skip-task-definition
  --force-new-deployment                 force a new deployment of the service
  --[no-]wait                            wait for service stable
  --wait-until="deployed"                Choose whether to wait for service stable or the deployment finishes. For
                                         ECS deployment controller: "(stable|deployed)"; For CodeDeploy deployment
                                         controller: "codedeploy:*", this accepts CodeDeploy lifecycle event (e.g.,
                                         "codedeploy:AfterAllowTraffic")
  --suspend-auto-scaling                 suspend application auto-scaling attached with the ECS service
  --resume-auto-scaling                  resume application auto-scaling attached with the ECS service
  --auto-scaling-min=AUTO-SCALING-MIN    set minimum capacity of application auto-scaling attached with the ECS service
  --auto-scaling-max=AUTO-SCALING-MAX    set maximum capacity of application auto-scaling attached with the ECS service
  --rollback-events=""                   roll back when specified events happened
                                         (DEPLOYMENT_FAILURE,DEPLOYMENT_STOP_ON_ALARM,DEPLOYMENT_STOP_ON_REQUEST,...)
                                         CodeDeploy only.
  --[no-]update-service                  update service attributes by service definition
  --latest-task-definition               deploy with the latest task definition without registering a new task definition
```

### Blue/Green deployment (with ECS deployment controller)

`ecspresso deploy` supports blue/green deployment using the ECS deployment controller. Configure ecs-service-def.json as follows. For minimal settings, you can set `deploymentController.type` and `deploymentConfiguration.strategy` as shown below.

```jsonnet
{
  "deploymentController": {
    "type": "ECS"
  },
  "deploymentConfiguration": {
    "strategy": "BLUE_GREEN"
  },
  // ...
```

For more advanced settings, you can define `deploymentConfiguration`, `loadBalancers` and `serviceConnectConfiguration` in ecs-service-def.json. For example, using an application load balancer and set lifecycle hooks, you can use the following configuration:

```jsonnet
{
  "deploymentController": {
    "type": "ECS"
  },
  "deploymentConfiguration": {
    "bakeTimeInMinutes": 1,
    "deploymentCircuitBreaker": {
      "enable": false,
      "rollback": false
    },
    "lifecycleHooks": [
      {
        "hookTargetArn": "arn:aws:lambda:ap-northeast-1:123456789012:function:bg-hook",
        "lifecycleStages": [
          "PRE_SCALE_UP",
          "POST_SCALE_UP",
          "TEST_TRAFFIC_SHIFT",
          "POST_TEST_TRAFFIC_SHIFT",
          "PRODUCTION_TRAFFIC_SHIFT",
          "POST_PRODUCTION_TRAFFIC_SHIFT"
        ],
        "roleArn": "arn:aws:iam::123456789012:role/ECSServiceRole"
      }
    ],
    "maximumPercent": 200,
    "minimumHealthyPercent": 100,
    "strategy": "BLUE_GREEN"
  },
  "loadBalancers": [
    {
      "containerName": "app",
      "containerPort": 80,
      "targetGroupArn": "arn:aws:elasticloadbalancing:ap-northeast-1:123456789012:targetgroup/my-target-group/1234567890abcdef"
      "advancedConfiguration": {
        "alternateTargetGroupArn": "arn:aws:elasticloadbalancing:ap-northeast-1:123456789012:targetgroup/my-alternate-target-group/1234567890abcdef",
        "productionListenerRule": "arn:aws:elasticloadbalancing:ap-northeast-1:123456789012:listener-rule/my-production-listener-rule/1234567890abcdef",
        "roleArn": "arn:aws:iam::123456789012:role/ECSServiceRole"
      }
    }
  ],
  // ...
```

For more details, See [Amazon ECS blue/green deployments
](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-type-blue-green.html).


### Blue/Green deployment (with AWS CodeDeploy)

`ecspresso deploy` can deploy services using the CODE_DEPLOY deployment controller. Configure ecs-service-def.json as follows.

```jsonnet
{
  "deploymentController": {
    "type": "CODE_DEPLOY"
  },
  // ...
}
```

Important notes:

- ecspresso does not create or modify any CodeDeploy resources. You must separately create an application and deployment group for your ECS service in CodeDeploy.
- ecspresso automatically detects CodeDeploy deployment settings for the ECS service.
- If there are numerous CodeDeploy applications, the API calls during this detection process may cause throttling. To mitigate this, specify the CodeDeploy application_name and deployment_group_name in the config file:

```yaml
# ecspresso.yml
codedeploy:
  application_name: myapp
  deployment_group_name: mydeployment
  deployment_config_name: myDeploymentConfig # optional, override DeploymentGroup setting
```

`ecspresso deploy` creates a new deployment for CodeDeploy, and it continues on CodeDeploy.

```console
$ ecspresso deploy --config ecspresso.yml --rollback-events DEPLOYMENT_FAILURE
2024-01-01T00:00:00.000+09:00 [INFO] [myService/default] Starting deploy
Service: myService
Cluster: default
TaskDefinition: myService:5
TaskSets:
   PRIMARY myService:5 desired:1 pending:0 running:1
Events:
2024-01-01T00:00:01.000+09:00 [INFO] [myService/default] creating a new task definition [path:ecs-task-def.json]
2024-01-01T00:00:01.000+09:00 [INFO] [myService/default] registering a new task definition
2024-01-01T00:00:01.000+09:00 [INFO] [myService/default] task definition is registered [task_definition:myService:6]
2024-01-01T00:00:01.000+09:00 [INFO] [myService/default] desired count [count:1]
2024-01-01T00:00:02.000+09:00 [INFO] [myService/default] deployment created on CodeDeploy [deployment_id:d-XXXXXXXXX] [url:https://ap-northeast-1.console.aws.amazon.com/codesuite/codedeploy/deployments/d-XXXXXXXXX?region=ap-northeast-1]
```

CodeDeploy appspec hooks can be defined in a config file. ecspresso automatically creates `Resources` and `version` elements in appspec on deployment:

```yaml
cluster: default
service: test
service_definition: ecs-service-def.json
task_definition: ecs-task-def.json
appspec:
  Hooks:
    - BeforeInstall: "LambdaFunctionToValidateBeforeInstall"
    - AfterInstall: "LambdaFunctionToValidateAfterTraffic"
    - AfterAllowTestTraffic: "LambdaFunctionToValidateAfterTestTrafficStarts"
    - BeforeAllowTraffic: "LambdaFunctionToValidateBeforeAllowingProductionTraffic"
    - AfterAllowTraffic: "LambdaFunctionToValidateAfterAllowingProductionTraffic"
```

## Scale out/in

To change the desired count of a service, specify `scale --tasks`.

```console
$ ecspresso scale --tasks 10
```

`scale` command is equivalent to `deploy --skip-task-definition --no-update-service`.

## Example of deploy

ecspresso can deploy a service using a `service_definition` JSON file.

```console
$ ecspresso deploy --config ecspresso.yml
...
```

```yaml
# ecspresso.yml
service_definition: service.json
```

service.json example:

```json
{
  "role": "ecsServiceRole",
  "desiredCount": 2,
  "loadBalancers": [
    {
      "containerName": "myLoadbalancer",
      "containerPort": 80,
      "targetGroupArn": "arn:aws:elasticloadbalancing:[region]:[account-id]:targetgroup/{target-name}/201ae83c14de522d"
    }
  ]
}
```

Keys are in the same format as `aws ecs describe-services` output.

- deploymentConfiguration
- launchType
- loadBalancers
- networkConfiguration
- placementConstraint
- placementStrategy
- role
- etc.

## Rollback

`ecspresso rollback` rolls back a service.

```console
$ ecspresso rollback --config ecspresso.yml
```

By default, `ecspresso rollback` stops an active deployment in progress. If no active deployment is found, it returns an error.

To rollback by deploying the previous task definition revision (legacy behavior), use `--with-previous-task-definition`. This finds the previous revision by listing the task definition family in descending order and deploys it regardless of active deployments.

```console
$ ecspresso rollback --with-previous-task-definition
```

For services using the CodeDeploy deployment controller, if there's an active deployment, ecspresso stops it with rollback. Otherwise, it creates a new deployment with the previous task definition.

## Example of run task

```console
$ ecspresso run --config ecspresso.yml --task-def=db-migrate.json
```

If `--task-def` is not set, ecspresso will use the task definition included in the service.

Other options for RunTask API are set by service attributes (CapacityProviderStrategy, LaunchType, PlacementConstraints, PlacementStrategy and PlatformVersion).

## Notes

### Version constraint

`required_version` in the configuration file is for fixing the version of ecspresso.

```yaml
required_version: ">= 2.0.0, < 3"
```

This allows ecspresso to execute if the version is greater than or equal to 2.0.0 and less than 3. If the version does not fall within this range, execution will fail.

This feature is implemented by [go-version](github.com/hashicorp/go-version).

### Manage Application Auto Scaling

For ECS services using Application Auto Scaling, adjusting the minimum and maximum auto-scaling settings with the `ecspresso scale` command is a breeze. Simply specify either `scale --auto-scaling-min` or `scale --auto-scaling-max` to modify the settings.

```console
$ ecspresso scale --tasks 5 --auto-scaling-min 5 --auto-scaling-max 20
```

`ecspresso deploy` and `scale` can suspend and resume application auto scaling.

- `--suspend-auto-scaling` sets suspended state to true.
- `--resume-auto-scaling` sets suspended state to false.

To change the suspended state, simply use `ecspresso scale --suspend-auto-scaling` or `ecspresso scale --resume-auto-scaling`. These commands will only change the suspended state without affecting other settings.

### Use Jsonnet instead of JSON and YAML.

ecspresso supports the [Jsonnet](https://jsonnet.org/) file format.

- v1.7 and later: Jsonnet support for service and task definitions
- v2.0 and later: Jsonnet support for the configuration file
- v2.4 and later: supports [Jsonnet functions](#jsonnet-functions)

If a file has the `.jsonnet` extension, ecspresso will proceed in the following order:

1. process it as Jsonnet
2. convert it to JSON
3. load it with evaluation template syntax.

Using [Template syntax](#template-syntax) in Jsonnet files may lead to syntax errors due to conflicts with Jsonnet syntax. In such cases, consider using [Jsonnet functions](#jsonnet-functions) instead.

```jsonnet
{
  cluster: 'default',
  service: 'myservice',
  service_definition: 'ecs-service-def.jsonnet',
  task_definition: 'ecs-task-def.jsonnet',
}
```

ecspresso includes [github.com/google/go-jsonnet](https://github.com/google/go-jsonnet) as a library, so a separate installation of jsonnet is not needed.

`--ext-str` and `--ext-code` flag sets [Jsonnet External Variables](https://jsonnet.org/ref/stdlib.html#ext_vars).

```console
$ ecspresso --ext-str Foo=foo --ext-code "Bar=1+1" ...
```

```jsonnet
{
  foo: std.extVar('Foo'), // = "foo"
  bar: std.extVar('Bar'), // = 2
}
```

### Jsonnet functions

v2.4 and later supports Jsonnet native functions in Jsonnet files.

In the .jsonnet file,:

1. Define `local func = std.native('func');`
2. Use `func()`

Jsonnet functions are evaluated when rendering Jsonnet files, which helps avoid conflicts with template syntax.

#### `env`, `must_env`

`env` and `must_env` functions work the similary to template functions in JSON and YAML files. However, unlike template functions, Jsonnet functions are capable of rendering non-string values from environment variables using `std.parseInt()`, `std.parseJson()`, etc.

```jsonnet
local env = std.native('env');
local must_env = std.native('must_env');
{
  foo: env('FOO', 'default value'),
  bar: must_env('BAR'),
  bazNumber: std.parseInt(env('BAZ_NUMBER', '0')),
  booBool: std.parseJson(env('BOO_BOOL', 'false')),
}
```

#### Other plugin-provided functions

See [Plugins](#plugins) section.

### Deploy to Fargate

When deploying services to Fargate, both task definitions and service definitions require specific settings.

For task definitions,

- requiresCompatibilities (requires "FARGATE")
- networkMode (requires "awsvpc")
- cpu (required)
- memory (required)
- executionRoleArn (optional)

```json
{
  "taskDefinition": {
    "networkMode": "awsvpc",
    "requiresCompatibilities": [
      "FARGATE"
    ],
    "cpu": "1024",
    "memory": "2048",
    // ...
}
```

For service-definitions,

- launchType (requires "FARGATE")
- networkConfiguration (requires "awsvpcConfiguration")

```json5
{
  "launchType": "FARGATE",
  "networkConfiguration": {
    "awsvpcConfiguration": {
      "subnets": [
        "subnet-aaaaaaaa",
        "subnet-bbbbbbbb"
      ],
      "securityGroups": [
        "sg-11111111"
      ],
      "assignPublicIp": "ENABLED"
    }
  },
  // ...
}
```

### Fargate Spot support

1. Set `capacityProviders` and `defaultCapacityProviderStrategy` for the ECS cluster.
1. To migrate an existing service to use Fargate Spot, define `capacityProviderStrategy` in the service definition as shown below. Use `ecspresso deploy --update-service` to apply the settings to the service.

```json
{
  "capacityProviderStrategy": [
    {
      "base": 1,
      "capacityProvider": "FARGATE",
      "weight": 1
    },
    {
      "base": 0,
      "capacityProvider": "FARGATE_SPOT",
      "weight": 1
    }
  ],
  // ...
```

### ECS Service Connect support

ecspresso supports [ECS Service Connect](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html).

To configure, define `serviceConnectConfiguration` in service definitions and `portMappings` in task definitions.

For more details, see [Service Connect parameters](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html#service-connect-parameters)

### EBS Volume support

ecspresso supports [Amazon EBS Volumes](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/ebs-volumes.html).

To configure, define `volumeConfigurations` in service definitions, and `mountPoints` and `volumes` in task definitions.


```jsonnet
// ecs-service-def.json
  "volumeConfigurations": [
    {
      "managedEBSVolume": {
        "filesystemType": "ext4",
        "roleArn": "arn:aws:iam::123456789012:role/ecsInfrastructureRole",
        "sizeInGiB": 10,
        "tagSpecifications": [
          {
            "propagateTags": "SERVICE",
            "resourceType": "volume"
          }
        ],
        "volumeType": "gp3"
      },
      "name": "ebs"
    }
  ]
```

```jsonnet
// ecs-task-def.json
// containerDefinitions[].mountPoints
      "mountPoints": [
        {
          "containerPath": "/mnt/ebs",
          "sourceVolume": "ebs"
        }
      ]
// volumes
  "volumes": [
    {
      "name": "ebs",
      "configuredAtLaunch": true
    }
  ]
```

`ecspresso run` command supports EBS volumes too.

By default, EBS volumes attached to standalone tasks are deleted when the task stops. Use the `--no-ebs-delete-on-termination` option to preserve volumes.

```console
$ ecspresso run --no-ebs-delete-on-termination
```

For tasks run by ECS services, EBS volumes are always deleted when the task stops. This is an ECS specification that ecspresso cannot override.


### S3 Files volume support

ecspresso supports [Amazon S3 Files](https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-files.html) volumes.

To configure, define `mountPoints` and `volumes` with `s3filesVolumeConfiguration` in task definitions.

```jsonnet
// ecs-task-def.json
// containerDefinitions[].mountPoints
      "mountPoints": [
        {
          "containerPath": "/mnt/s3",
          "sourceVolume": "s3files"
        }
      ]
// volumes
  "volumes": [
    {
      "name": "s3files",
      "s3filesVolumeConfiguration": {
        "fileSystemArn": "arn:aws:s3files:ap-northeast-1:123456789012:file-system/fs-01234567890abcdef",
        "rootDirectory": "/"
      }
    }
  ]
```

See [Prerequisites for S3 Files](https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-files-prereq-policies.html) for IAM role and security group configuration required for S3 Files mount targets.

### VPC Lattice support

ecspresso supports [VPC Lattice](https://aws.amazon.com/vpc/lattice/) integration.

1. Define `portMappings` in the task definition. The `name` field is required.
```json
{
  "containerDefinitions": [
    {
      "name": "webserver",
      "portMappings": [
        {
          "name": "web-80-tcp",
          "containerPort": 80,
          "hostPort": 80,
          "protocol": "tcp",
          "appProtocol": "http"
        }
      ],
```

2. Define `vpcLatticeConfigurations` in the service definition. The `portName`, `roleArn`, and `targetGroupArn` fields are required.`

- The `portName` must match the `name` field of the `portMappings` in the task definition.
- The `roleArn` is the IAM role that the ECS service assumes to call the VPC Lattice API.
  - The role must have the `ecs.amazonaws.com` service principal.
  - The role should have the `AmazonECSInfrastructureRolePolicyForVpcLattice` policy or equivalent permissions.
- The `targetGroupArn` is the ARN of the VPC Lattice target group.

```json
{
  "vpcLatticeConfigurations": [
    {
      "portName": "web-80-tcp",
      "roleArn": "arn:aws:iam::123456789012:role/ecsInfrastructureRole",
      "targetGroupArn": "arn:aws:vpc-lattice:ap-northeast-1:123456789012:targetgroup/tg-009147df264a0bacb"
    }
  ],
```

ecspresso doesn't create or modify any VPC Lattice resources. You must create and associate a VPC Lattice target group with the ECS service.

See also [Use Amazon VPC Lattice to connect, observe, and secure your Amazon ECS services](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-vpc-lattice.html).

### High resolution CloudWatch metrics

ecspresso supports [high resolution CloudWatch metrics](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/cloudwatch-metrics.html) for ECS services.

To configure, define `monitoring` in the service definition. The `metricConfigurations` field specifies which metrics to collect and at what resolution.

```json
{
  "monitoring": {
    "metricConfigurations": [
      {
        "metricNames": ["CPUUtilization", "MemoryUtilization"],
        "resolutionSeconds": 20
      }
    ]
  }
}
```

- `metricNames`: The metrics to configure. Supported values are `CPUUtilization` and `MemoryUtilization`.
- `resolutionSeconds`: The resolution in seconds. Valid values are `20` (high resolution) and `60` (default).

When not specified, Amazon ECS uses the default resolution of 60 seconds. High resolution metrics enable faster auto scaling responses.

### Pause lifecycle hooks

ecspresso supports [pause lifecycle hooks](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-lifecycle-hooks.html) for ECS service deployments.

Pause hooks are available for blue/green, linear, and canary deployment strategies (not rolling).

To configure, define `lifecycleHooks` with `targetType: PAUSE` in `deploymentConfiguration` in the service definition.

```json
{
  "deploymentConfiguration": {
    "strategy": "BLUE_GREEN",
    "lifecycleHooks": [
      {
        "lifecycleStages": ["POST_TEST_TRAFFIC_SHIFT"],
        "targetType": "PAUSE",
        "timeoutConfiguration": {
          "timeoutInMinutes": 60,
          "action": "ROLLBACK"
        }
      }
    ]
  }
}
```

Use `--wait-until=paused` with `ecspresso deploy` to wait until the deployment pauses at the lifecycle hook.

```console
$ ecspresso deploy --wait-until=paused
```

After reviewing the deployment, use `ecspresso continue` to proceed or `ecspresso rollback` to roll back.

```console
# Continue the deployment to the next stage
$ ecspresso continue

# Or roll back the deployment
$ ecspresso rollback
```

`ecspresso continue` also accepts `--wait-until` to wait after continuing (default: `deployed`). Use `--wait-until=paused` to wait for the next pause hook if multiple hooks are configured.

For linear and canary deployments, pause hooks at `PRE_PRODUCTION_TRAFFIC_SHIFT` are invoked at each traffic shift step. Each step generates a unique `hookId`, so you need to run `ecspresso continue --wait-until=paused` repeatedly for each step.

### ECS Express mode support

ecspresso supports [ECS Express](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/express-service-overview.html) mode, which provides a simplified way to deploy ECS services with a single definition file instead of separate task and service definitions.

#### Supported commands

The following commands work with Express mode:

- `init --express` - Import an existing Express service into definition files
- `deploy` - Create or update Express services
- `diff` - Show differences between local and remote Express definitions
- `render expressdef` - Render an Express definition file
- `delete` - Delete an Express service
- `status` - Show Express service status including ingress paths
- `rollback` - Partial support (works only during active deployments)
- `verify` - Verify resources in Express definition
- `exec`, `refresh`, `tasks`, `wait` - Also supported

#### Unsupported commands

Some commands do not work with Express mode: `scale`, `run`, `revisions`, `register`, `deregister`, `appspec`.

#### Configuration

To use Express mode, specify `express_definition` instead of `task_definition` and `service_definition` in your config file:

```yaml
# ecspresso.yml
region: ap-northeast-1
cluster: ecspresso
service: myservice
express_definition: ecs-express-def.json
```

#### Minimal Express definition

```json
{
  "executionRoleArn": "arn:aws:iam::123456789012:role/ecsTaskExecutionRole",
  "infrastructureRoleArn": "arn:aws:iam::123456789012:role/ecsInfrastructureRoleForExpressServices",
  "primaryContainer": {
    "image": "nginx:latest"
  }
}
```

#### Full Express definition example

```jsonnet
// ecs-express-def.jsonnet
{
  cpu: '256',
  memory: '512',
  executionRoleArn: 'arn:aws:iam::123456789012:role/ecsTaskExecutionRole',
  taskRoleArn: 'arn:aws:iam::123456789012:role/ecsTaskRole',
  infrastructureRoleArn: 'arn:aws:iam::123456789012:role/ecsInfrastructureRoleForExpressServices',
  healthCheckPath: '/',
  networkConfiguration: {
    securityGroups: ['sg-00123456789abcdef'],
    subnets: ['subnet-00123456789abcdef', 'subnet-0123456789abcdef0'],
  },
  primaryContainer: {
    image: 'nginx:latest',
    containerPort: 80,
    environment: [
      { name: 'ENV', value: 'production' },
    ],
    awsLogsConfiguration: {
      logGroup: '/aws/ecs/myservice',
      logStreamPrefix: 'ecs',
    },
  },
  scalingTarget: {
    autoScalingMetric: 'AVERAGE_CPU',
    autoScalingTargetValue: 60,
    minTaskCount: 1,
    maxTaskCount: 10,
  },
  tags: [
    { key: 'Environment', value: 'production' },
  ],
}
```

#### Migration from Express mode

Existing ECS Express services can be imported as non-express (normal) mode by `ecspresso init --no-express`. This creates standard definition files (`ecspresso.yml`, `ecs-service-def.json`, and `ecs-task-def.json`).

Note: ECS Express mode cannot update `deploymentConfiguration` and `loadBalancers` in a service definition. `ecspresso init --no-express` also omits these fields.

### How to check diff and verify service/task definitions before deploy.

ecspresso supports `diff` and `verify` commands.

#### diff

Shows differences between local task/service definitions and remote (on ECS) definitions.

```diff
$ ecspresso diff
--- arn:aws:ecs:ap-northeast-1:123456789012:service/ecspresso-test/nginx-local
+++ ecs-service-def.json
@@ -38,5 +38,5 @@
   },
   "placementConstraints": [],
   "placementStrategy": [],
-  "platformVersion": "1.3.0"
+  "platformVersion": "LATEST"
 }

--- arn:aws:ecs:ap-northeast-1:123456789012:task-definition/ecspresso-test:202
+++ ecs-task-def.json
@@ -1,6 +1,10 @@
 {
   "containerDefinitions": [
     {
       "cpu": 0,
       "environment": [],
       "essential": true,
-      "image": "nginx:latest",
+      "image": "nginx:alpine",
       "logConfiguration": {
         "logDriver": "awslogs",
         "options": {
```

v2.4 or later, `ecspresso diff --external` can invoke an external command. You can use the "diff" command you like.

For example, use [difftastic](https://github.com/Wilfred/difftastic) (`difft`) command.

```console
$ ecspresso diff --external "difft --color=always"

$ ECSPRESSO_DIFF_COMMAND="difft --color=always" ecspresso diff
```

The command should exit with status 0. If it exits with a non-zero status when two files differ (for example, `diff(1)`), you need to write a wrapper command.

`ecspresso diff --jsonnet` renders the diff output in Jsonnet format instead of JSON. This is useful when you manage definitions in Jsonnet files.

```console
$ ecspresso diff --jsonnet
```

`ecspresso diff --without-service` skips the diff of the service definition and only shows the diff of the task definition.

```console
$ ecspresso diff --without-service
```


#### verify

Verify resources related with service/task definitions.

For example it checks if,
- An ECS cluster exists.
- The target groups in service definitions match the container name and port defined in the definitions.
- A task role and a task execution role exist and can be assumed by ecs-tasks.amazonaws.com.
- Container images exist at the URL defined in task definitions. (Checks only for ECR or DockerHub public images.)
- Secrets in task definitions exist and are readable.
- Log streams can be created and messages can be put into the specified CloudWatch log groups streams.

ecspresso verify tries to assume the task execution role defined in task definitions to verify these items. If it fails to assume the role, it continues to verify with the current session.

```console
$ ecspresso verify
2024-01-01T00:00:00.000+09:00 [INFO] [nginx-local/ecspresso-test] Starting verify
  TaskDefinition
    ExecutionRole[arn:aws:iam::123456789012:role/ecsTaskRole]
    --> [OK]
    TaskRole[arn:aws:iam::123456789012:role/ecsTaskRole]
    --> [OK]
    ContainerDefinition[nginx]
      Image[nginx:alpine]
      --> [OK]
      LogConfiguration[awslogs]
      --> [OK]
    --> [OK]
  --> [OK]
  ServiceDefinition
  --> [OK]
  Cluster
  --> [OK]
2024-01-01T00:00:04.000+09:00 [INFO] [nginx-local/ecspresso-test] Verify OK!
```

### Manipulate ECS tasks

ecspresso can manipulate ECS tasks using the  `tasks` and `exec` commands.

After v2.0, These operations are provided by [ecsta](https://github.com/fujiwara/ecsta) as a library. The ecsta CLI can manipulate any ECS tasks, not limited to those deployed by ecspresso.

Consider using ecsta as a CLI command.

#### tasks

The `tasks` command lists tasks related to the ecspresso configuration. When `service` is configured, it lists tasks belonging to the service plus standalone tasks of the same family (typically launched by `ecspresso run`). Tasks of other services that share the task definition family are excluded. When `service` is not configured, it lists tasks of the task definition family.

```
Usage: ecspresso tasks <command> [flags]

Common flags:
      --id=                       task ID
      --output=table              output format (table, json, tsv)

Commands:
  tasks list          list tasks (default)
  tasks find          find a task from tasks list and dump it as JSON
  tasks stop          stop a task
  tasks trace         trace a task
  tasks logs          show logs of a task
```

##### tasks find

The `find` subcommand enables task selection from a list and displays it as JSON.

The `ECSPRESSO_FILTER_COMMAND` environment variable can be set to specify a command for filtering tasks, such as [peco](https://github.com/peco/peco), [fzf](https://github.com/junegunn/fzf), etc.

```console
$ ECSPRESSO_FILTER_COMMAND=peco ecspresso tasks find
```

##### tasks stop

The `stop` subcommand allows for task selection and stopping from a list.

```console
$ ecspresso tasks stop --force
```

```
Flags:
      --force=false               stop the task without confirmation
```

##### tasks logs

The `logs` subcommand shows CloudWatch Logs of the task.

```console
$ ecspresso tasks logs -f -d 5m --container app
```

```
Flags:
  -f, --follow=false              follow logs
  -d, --duration=1m               duration of logs
  -s, --start-time=               start time of logs
      --container=                container name
```

Use `--follow` to follow logs in real time, `--duration` to specify the time range, and `--start-time` to specify an absolute start time. When `--log-format json` is set, logs are output as JSON lines.

#### exec

The `exec` command executes a command on a task.

[session-manager-plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) is required in PATH.

```
Usage: ecspresso exec <command> [flags]

Common flags:
      --id=                       task ID
      --container=                container name

Commands:
  exec run              execute command on task (default)
  exec portforward      port forwarding to a task
  exec cp <src> <dest>  copy files between local and task
```

If `--id` is not set, the command shows a list of tasks to select a task to execute.

The `ECSPRESSO_FILTER_COMMAND` environment variable works the same as with the `tasks` command.

See also the official documentation [Using Amazon ECS Exec for debugging](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html).

##### exec run

The `run` subcommand (default) executes a command on a task interactively.

```console
$ ecspresso exec run --command "ls -la"
```

```
Flags:
      --command=sh                command to execute
```

##### exec portforward

The `portforward` subcommand enables port forwarding from a local port to an ECS task's port.

```console
$ ecspresso exec portforward --port 80 --local-port 8080
```

```
Flags:
      --local-port=0              local port number
      --port=0                    remote port number
      --host=                     remote host
  -L                              short expression of local-port:host:port
```

If `--id` is not set, the command shows a list of tasks to select for port forwarding.

When `--local-port` is not specified, an ephemeral port is used as the local port.

The `-L` option is a short expression for `local-port:host:port`. For example, `-L 8080:example.com:80` is equivalent to `--local-port 8080 --host example.com --port 80`.

```console
$ ecspresso exec portforward -L 8080:example.com:80
```

##### exec cp

The `cp` subcommand copies files between local and a running ECS task.

The source and destination arguments use `taskID:/path` format for the remote side. Use `_` as the task ID to select a task interactively.

```console
# Copy a local file to the task
$ ecspresso exec cp /local/file.txt _:/remote/file.txt

# Copy a file from the task to local
$ ecspresso exec cp _:/remote/file.txt /local/file.txt

# Specify a task ID directly
$ ecspresso exec cp /local/file.txt abcdef1234567890:/remote/file.txt
```

```
Flags:
      --port=12345                port number for file transfer
      --[no-]progress             show progress bar (default true)
```

### Show documentation

The `docs` command shows the embedded documentation (this README) directly from the ecspresso binary. This command does not require AWS credentials or a configuration file.

```
Flags:
      --article="readme"          article name to display
      --list                      list available articles
      --index                     show table of contents
      --search=""                 search keyword in documents
      --json                      output in JSON format
```

Show the full README:

```console
$ ecspresso docs
```

Show the table of contents with section headings and line numbers:

```console
$ ecspresso docs --index
```

Search for sections containing a keyword (case-insensitive):

```console
$ ecspresso docs --search "fargate"
```

Output in JSON format (useful for LLM agents and other tools):

```console
$ ecspresso docs --search "deploy" --json
```

The JSON output contains structured sections with `level`, `title`, `content`, and `line` fields, making it easy for automated tools and LLM agents to consume ecspresso documentation programmatically.

List available articles:

```console
$ ecspresso docs --list
readme	ecspresso README
```

### LLM agent integration

ecspresso provides an [Agent Skill](https://agentskills.io/) for LLM agents (Claude Code, GitHub Copilot, OpenAI Codex, etc.) to use ecspresso effectively. The skill covers common workflows, command usage patterns, and best practices. Powered by [Songmu/skillsmith](https://github.com/Songmu/skillsmith).

Install the skill for your user:

```console
$ ecspresso skills install
```

This installs the skill to `~/.agents/skills/ecspresso/SKILL.md`. Compatible LLM agents automatically discover skills in this directory — no additional configuration is needed.

To share the skill with your team via the repository, use `--scope repo`:

```console
$ ecspresso skills install --scope repo
```

This installs to `.agents/skills/` in the repository root. Commit this directory so that team members' agents can use the skill without installing it individually.

Other `skills` operations:

```console
$ ecspresso skills list        # List available skills
$ ecspresso skills status      # Show installation status
$ ecspresso skills update      # Update installed skills
$ ecspresso skills uninstall   # Remove installed skills
$ ecspresso skills reinstall   # Reinstall all managed skills
```

## Plugins

ecspresso supports plugins to extend template functions and Jsonnet native functions.

### tfstate

The tfstate plugin introduces the `tfstate` and `tfstatef` template functions.

ecspresso.yml
```yaml
region: ap-northeast-1
cluster: default
service: test
service_definition: ecs-service-def.json
task_definition: ecs-task-def.json
plugins:
  - name: tfstate
    config:
      url: s3://my-bucket/terraform.tfstate
      # or path: terraform.tfstate    # path to local file
```

ecs-service-def.json
```json
{
  "networkConfiguration": {
    "awsvpcConfiguration": {
      "subnets": [
        "{{ tfstatef `aws_subnet.private['%s'].id` `az-a` }}"
      ],
      "securityGroups": [
        "{{ tfstate `data.aws_security_group.default.id` }}"
      ]
    }
  }
}
```

`{{ tfstate "resource_type.resource_name.attr" }}` will expand to the attribute value of the resource in tfstate.

`{{ tfstatef "resource_type.resource_name['%s'].attr" "index" }}` is similar to `{{ tfstatef "resource_type.resource_name['index'].attr" }}`. This function is useful for build a resource addresses with environment variables.

```
{{ tfstatef `aws_subnet.ecs['%s'].id` (must_env `SERVICE`) }}
```

#### tfstate Jsonnet function

`tfstate` Jsonnet function works the same as template function in JSON and YAML files.
`tfstatef` Jsonnet function is not provided. Use `std.format()` or interpolation instead.

```jsonnet
local tfstate = std.native('tfstate');
{
  networkConfiguration: {
    awsvpcConfiguration: {
      subnets: [
        tfstate('aws_subnet.private["%s"].id' % 'az-z'),
        tfstate(std.format('aws_subnet.private["%s"].id', 'az-b')),
      ],
      securityGroups: [
        sg.id for sg in std.objectValues(tfstate('data.aws_security_group.default'))
        // data.aws_security_group.default["first"].id
        // data.aws_security_group.default["second"].id
      ]
    }
  }
}
```

When the tfstate includes count/for_each resources, so the resource address can be specified with an index.

```jsonnet
local tfstate = std.native('tfstate');
tfstate('aws_subnet.private["%s"].id' % 'az-a') // aws_subnet.private["az-a"].id
```

To fetch all resources of count/for_each resources, use `std.objectValues()`.

```jsonnet
local tfstate = std.native('tfstate');
{
  subnets: [
    subnet.id for subnet in std.objectValues(tfstate('aws_subnet.private'))
    // aws_subnet.private["az-a"].id
    // aws_subnet.private["az-b"].id
    // aws_subnet.private["az-c"].id
  ],
}
```

#### Supported tfstate URL formats

- Local file `file://path/to/terraform.tfstate`
- HTTP/HTTPS `https://example.com/terraform.tfstate`
- Amazon S3 `s3://{bucket}/{key}`
- Terraform Cloud `remote://api.terraform.io/{organization}/{workspaces}`
  - `TFE_TOKEN` environment variable is required.
- Google Cloud Storage `gs://{bucket}/{key}`
- Azure Blog Storage `azurerm://{resource_group_name}/{storage_account_name}/{container_name}/{blob_name}`

This plugin uses [tfstate-lookup](https://github.com/fujiwara/tfstate-lookup) to load tfstate.

#### Multiple tfstate support

`func_prefix` adds prefixes to template function names for each plugin configuration, enabling support for multiple tfstate files.

```yaml
# ecspresso.yml
plugins:
   - name: tfstate
     config:
       url: s3://tfstate/first.tfstate
     func_prefix: first_
   - name: tfstate
     config:
       url: s3://tfstate/second.tfstate
     func_prefix: second_
```

In templates, functions are called with the specified prefixes.

```go-template
[
  "{{ first_tfstate `aws_s3_bucket.main.arn` }}",
  "{{ second_tfstate `aws_s3_bucket.main.arn` }}"
]
```

Similar features are also supported for Jsonnet.

```jsonnet
local first_tfstate = std.native('first_tfstate'); // func_prefix: first_
local second_tfstate = std.native('second_tfstate'); // func_prefix: second_
[
  first_tfstate('aws_s3_bucket.main.arn'),
  second_tfstate('aws_s3_bucket.main.arn'),
]
```

### CloudFormation

The cloudformation plugin introduces the `cfn_output` and `cfn_export` template functions.

An example of a CloudFormation stack template defining Outputs and Exports.

```yaml
# StackName: ECS-ecspresso
Outputs:
  SubnetAz1:
    Value: !Ref PublicSubnetAz1
  SubnetAz2:
    Value: !Ref PublicSubnetAz2
  EcsSecurityGroupId:
    Value: !Ref EcsSecurityGroup
    Export:
      Name: !Sub ${AWS::StackName}-EcsSecurityGroupId
```

Load the cloudformation plugin in a config file.

ecspresso.yml
```yaml
# ...
plugins:
  - name: cloudformation
```

`cfn_output StackName OutputKey` looks up the OutputValue of OutputKey in the StackName.
`cfn_export ExportName` looks up the exported value by name.

ecs-service-def.json
```json
{
  "networkConfiguration": {
    "awsvpcConfiguration": {
      "subnets": [
        "{{ cfn_output `ECS-ecspresso` `SubnetAz1` }}",
        "{{ cfn_output `ECS-ecspresso` `SubnetAz2` }}"
      ],
      "securityGroups": [
        "{{ cfn_export `ECS-ecspresso-EcsSecurityGroupId` }}"
      ]
    }
  }
}
```

#### Jsonnet functions `cfn_output`, `cfn_export`

Similar features are also supported for Jsonnet.


```jsonnet
local cfn_output = std.native('cfn_output');
local cfn_export = std.native('cfn_export');
{
  subnets: [
    cfn_output('ECS-ecspresso', 'SubnetAz1'),
    cfn_output('ECS-ecspresso', 'SubnetAz2'),
  ],
  securityGroups: [
    cfn_export('ECS-ecspresso-EcsSecurityGroupId'),
  ],
}
```

### SSM Parameter Store lookups

The `ssm` template function reads parameters from AWS Systems Manager (SSM) Parameter Store.

Given SSM Parameter Store has the following parameters:

- name: '/path/to/string', type: String, value: "ImString"
- name: '/path/to/stringlist', type: StringList, value: "ImStringList0,ImStringList1"
- name: '/path/to/securestring', type: SecureString, value: "ImSecureString"

This template,

```json
{
  "string": "{{ ssm `/path/to/string` }}",
  "stringlist": "{{ ssm `/path/to/stringlist` 1 }}",  *1
  "securestring": "{{ ssm `/path/to/securestring` }}"
}
```

will be rendered as:

```json
{
  "string": "ImString",
  "stringlist": "ImStringList1",
  "securestring": "ImSecureString"
}
```

#### Jsonnet functions `ssm`, `ssm_list`

The `ssm` function works the same as template function. For string list parameters, use `ssm_list` to specify the index.

```jsonnet
local ssm = std.native('ssm');
local ssm_list = std.native('ssm_list');
{
  string: ssm('/path/to/string'),
  stringlist: ssm_list('/path/to/stringlist', 1),
  securestring: ssm('/path/to/securestring'),
}
```

### Resolve secretsmanager secret ARN

The `secretsmanager_arn` template function resolves the Secrets Manager secret ARN by secret name.

```go-template
  "secrets": [
    {
      "name": "FOO",
      "valueFrom": "{{ secretsmanager_arn `foo` }}"
    }
  ]
```

will be rendered as:

```json
  "secrets": [
    {
      "name": "FOO",
      "valueFrom": "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:foo-06XQOH"
    }
  ]
```

#### Jsonnet function `secretsmanager_arn`

The `secretsmanager_arn` function works the same as template function.

```jsonnet
local secretsmanager_arn = std.native('secretsmanager_arn');
{
  secrets: [
    {
      name: "FOO",
      valueFrom: secretsmanager_arn('foo'),
    }
  ]
}
```

### Execute external commands

The `external` plugin introduces functions to execute any external commands.

For example, `jq -n "{ Now: now | todateiso8601" }` returns the current date in ISO8601 format as a JSON object.

```console
$ jq -n "{ Now: now | todateiso8601 }"
{
  "Now": "2024-10-25T16:13:22Z"
}
```

You can use this command as a template function in the definition files.

First, define the plugin in the configuration file.

ecspresso.yml
```yaml
plugins:
  - name: external
    config:
      name: jq
      command: ["jq", "-n"]
      num_args: 1
      timeout: 5
```

The `config` section defines the following parameters:

- `name`: template function name
- `command`: external command and arguments (array)
  - The command must return a JSON string or any strings to stdout.
- `num_args`: number of arguments (optional, default 0)
- `parser`: parser type "json" or "string" (optional, default "json")
- `timeout`: command execution timeout (optional, default never timeout). Accepts a duration string (`"30s"`, `"5m"`, `"1h"`) or a bare number / digit string interpreted as seconds (`30` → 30 seconds).
- `mode`: execution mode "exec" or "jsonrpc" (optional, default "exec")
  - `exec`: the command is launched once per template function call (default)
  - `jsonrpc`: the command is launched once on the first call and kept running; subsequent calls communicate over stdin/stdout using [JSON RPC 2.0](https://www.jsonrpc.org/specification). Useful for commands with high startup cost.

And use the template function in the definition files as follows.

```jsonnet
local jq = std.native('jq');
{
  today: jq('{ Now: now | todateiso8601 }').Now,
}
```

```go-template
{
  "today": "{{ (jq `{Now: now | todateiso8601}`).Now }}"
}
```

#### JSON RPC mode

When `mode: jsonrpc` is set, ecspresso launches the command once and keeps it running. Each template function call sends a [JSON RPC 2.0](https://www.jsonrpc.org/specification) request to the process's stdin and reads the response from stdout.

The external process must read newline-delimited JSON requests from stdin and write newline-delimited JSON responses to stdout.

Request format:
```json
{"jsonrpc":"2.0","method":"<name>","params":["arg0","arg1"],"id":1}
```

Response format (success):
```json
{"jsonrpc":"2.0","result":<any JSON value>,"id":1}
```

Response format (error):
```json
{"jsonrpc":"2.0","error":{"code":-32600,"message":"error message"},"id":1}
```

- `method` is the `name` field from the plugin config.
- `params` is an array of string arguments passed to the template function.
- The `id` field correlates a request with its response. A response whose `id` does not match the request is treated as an error.
- `timeout` applies per call. A call that times out fails immediately, the same as `exec` mode. The process is killed so a stalled command does not linger.
- The process is also terminated when ecspresso exits (its stdin is closed, then SIGTERM/SIGKILL).

Example configuration using a long-running Python server:

```yaml
plugins:
  - name: external
    config:
      name: lookup
      command: ["python3", "lookup_server.py"]
      num_args: 1
      mode: jsonrpc
      timeout: 10
```

## LICENSE

MIT

## Author

KAYAC Inc.
