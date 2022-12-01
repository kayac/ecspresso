# ecspresso

ecspresso is a deployment tool for Amazon ECS.

(pronounced same as "espresso")

## Documents

[ecspresso handbook](https://zenn.dev/fujiwara/books/ecspresso-handbook) (Japanese)

[ecspresso Advent Calendar 2020](https://adventar.org/calendars/5916) (Japanese)

[Differences of v1 between v2](docs/v1-v2.md).

## Install

### Homebrew (macOS and Linux)

```console
$ brew install kayac/tap/ecspresso
```

### Binary packages

[Releases](https://github.com/kayac/ecspresso/releases)

### CircleCI Orbs

https://circleci.com/orbs/registry/orb/fujiwara/ecspresso

```yaml
version: 2.1
orbs:
  ecspresso: fujiwara/ecspresso@2.0.0
jobs:
  install:
    steps:
      - checkout
      - ecspresso/install:
          version: v2.0.0 # or latest
      - run:
          command: |
            ecspresso version
```

 `version: latest` installs different versions of ecspresso for each Orb version.
- fujiwara/ecspresso@0.0.15
  - The latest release version (v2 or later)
- fujiwara/ecspresso@1.0.0
  - The latest version of v1.x
- fujiwara/ecspresso@2.0.0
  - The latest version of v2.x

### GitHub Actions

Action kayac/ecspresso@v1 installs ecspresso binary for Linux into /usr/local/bin. This action runs install only.

```yml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: kayac/ecspresso@v2
        with:
          version: v2.0.0 # or latest
      - run: |
          ecspresso deploy --config ecspresso.yml
```

Pass the parameter "latest" to use the latest version of ecspresso.

```yaml
      - uses: kayac/ecspresso@v1
        with:
          version: latest
```

 `version: latest` installs different versions of ecspresso for each Action version.
- kayac/ecspresso@v1
  - The latest version of v1.x
- kayac/ecspresso@v2
  - The latest version (v1.99 or later) that includes prerelease version.
  - After v2.0.0 is released, This behavior will be changed to install stable versions only.

## Usage

```
Usage: ecspresso <command>

Flags:
  -h, --help                      Show context-sensitive help.
      --envfile=ENVFILE,...       environment files
      --debug                     enable debug log
      --ext-str=KEY=VALUE;...     external string values for Jsonnet
      --ext-code=KEY=VALUE;...    external code values for Jsonnet
      --config="ecspresso.yml"    config file
      --option=OPTION

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

## Quick Start

ecspresso can easily manage your existing/running ECS service by codes.

Try `ecspresso init` for your ECS service with option `--region`, `--cluster` and `--service`.

```console
$ ecspresso init --region ap-northeast-1 --cluster default --service myservice --config ecspresso.yml
2019/10/12 01:31:48 myservice/default save service definition to ecs-service-def.json
2019/10/12 01:31:48 myservice/default save task definition to ecs-task-def.json
2019/10/12 01:31:48 myservice/default save config to ecspresso.yml
```

Let me see the generated files ecspresso.yml, ecs-service-def.json, and ecs-task-def.json.

And then, you already can deploy the service by ecspresso!

```console
$ ecspresso deploy --config ecspresso.yml
```

## Configuration file

A configuration file of ecspresso (YAML or JSON, or Jsonnet format).

```yaml
region: ap-northeast-1 # or AWS_REGION environment variable
cluster: default
service: myservice
task_definition: taskdef.json
timeout: 5m # default 10m
```

`ecspresso deploy` works as below.

- Register a new task definition from `task-definition` file (JSON or Jsonnet).
  - Replace ```{{ env `FOO` `bar` }}``` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, replaced by "bar"
  - Replace ```{{ must_env `FOO` }}``` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, abort immediately.
- Update service tasks by the `service_definition` file (JSON or Jsonnet).
- Wait for the service to be stable.

Configuration files and task/service definition files are read by [go-config](https://github.com/kayac/go-config). go-config has template functions `env`, `must_env` and `json_escape`.

## Example of deployment

### Rolling deployment

```console
$ ecspresso deploy --config ecspresso.yml
2017/11/09 23:20:13 myService/default Starting deploy
Service: myService
Cluster: default
TaskDefinition: myService:3
Deployments:
    PRIMARY myService:3 desired:1 pending:0 running:1
Events:
2017/11/09 23:20:13 myService/default Creating a new task definition by myTask.json
2017/11/09 23:20:13 myService/default Registering a new task definition...
2017/11/09 23:20:13 myService/default Task definition is registered myService:4
2017/11/09 23:20:13 myService/default Updating service...
2017/11/09 23:20:13 myService/default Waiting for service stable...(it will take a few minutes)
2017/11/09 23:23:23 myService/default  PRIMARY myService:4 desired:1 pending:0 running:1
2017/11/09 23:23:29 myService/default Service is stable now. Completed!
```

### Blue/Green deployment (with AWS CodeDeploy)

`ecspresso deploy` can deploy service having CODE_DEPLOY deployment controller. See ecs-service-def.json below.

```json
{
  "deploymentController": {
    "type": "CODE_DEPLOY"
  },
  // ...
}
```

ecspresso doesn't create and modify any resources about CodeDeploy. You must create an application and a deployment group for your ECS service on CodeDeploy in the other way.

ecspresso finds a CodeDeploy deployment setting for the ECS service automatically.
But, if you have too many CodeDeploy applications, API calls of that finding process may cause throttling.

In this case, you may specify CodeDeploy application_name and deployment_group_name in a config file.

```yaml
# ecspresso.yml
codedeploy:
  application_name: myapp
  deployment_group_name: mydeployment
```

`ecspresso deploy` creates a new deployment for CodeDeploy, and it continues on CodeDeploy.

```console
$ ecspresso deploy --config ecspresso.yml --rollback-events DEPLOYMENT_FAILURE
2019/10/15 22:47:07 myService/default Starting deploy
Service: myService
Cluster: default
TaskDefinition: myService:5
TaskSets:
   PRIMARY myService:5 desired:1 pending:0 running:1
Events:
2019/10/15 22:47:08 myService/default Creating a new task definition by ecs-task-def.json
2019/10/15 22:47:08 myService/default Registering a new task definition...
2019/10/15 22:47:08 myService/default Task definition is registered myService:6
2019/10/15 22:47:08 myService/default desired count: 1
2019/10/15 22:47:09 myService/default Deployment d-XXXXXXXXX is created on CodeDeploy
2019/10/15 22:47:09 myService/default https://ap-northeast-1.console.aws.amazon.com/codesuite/codedeploy/deployments/d-XXXXXXXXX?region=ap-northeast-1
```

CodeDeploy appspec hooks can be defined in a config file. ecspresso creates `Resources` and `version` elements in appspec on deploy automatically.

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

To change a desired count of the service, specify `scale --tasks`.

```console
$ ecspresso scale --config ecspresso.yml --tasks 10
```

`scale` command is equivalent to `deploy --skip-task-definition --no-update-service`.

## Example of deploy

escpresso can deploy a service by `service_definition` JSON file and `task_definition`.

```console
$ ecspresso deploy --config ecspresso.yml
...
```

```yaml
# ecspresso.yml
service_definition: service.json
```

example of service.json below.

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

## Example of run task

```console
$ ecspresso run --config ecspresso.yml --task-def=db-migrate.json
```

When `--task-def` is not set, use a task definition included in a service.

Other options for RunTask API are set by service attributes(CapacityProviderStrategy, LaunchType, PlacementConstraints, PlacementStrategy and PlatformVersion).

# Notes

## Use Jsonnet instead of JSON

ecspresso v1.7 or later can use [Jsonnet](https://jsonnet.org/) file format for service and task definition.

v2.0 or later can use Jsonnet for configuration file too.

If the file extension is .jsonnet, ecspresso will process Jsonnet first, convert it to JSON, and then load it.

```jsonnet
{
  cluser: 'default',
  service: 'myservice',
  service_definition: 'ecs-service-def.jsonnet',
  task_definition: 'ecs-task-def.jsonnet',
}
```

ecspresso includes [github.com/google/go-jsonnet](https://github.com/google/go-jsonnet) as a library, we don't need the jsonnet command.

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

## Deploy to Fargate

If you want to deploy services to Fargate, task definitions and service definitions require some settings.

For task definitions,

- requiresCompatibilities (required "FARGATE")
- networkMode (required "awsvpc")
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

For service-definition,

- launchType (required "FARGATE")
- networkConfiguration (required "awsvpcConfiguration")

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

## Fargate Spot support

1. Set capacityProviders and defaultCapacityProviderStrategy to ECS cluster.
1. If you hope to migrate existing service to use Fargate Spot, define capacityProviderStrategy into service definition as below. `ecspresso deploy --update-service` applies the settings to the service.

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

## How to check diff and verify service/task definitions before deploy.

ecspresso supports `diff` and `verify` subcommands.

### diff

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

### verify

Verify resources related with service/task definitions.

For example,
- An ECS cluster exists.
- The target groups in service definitions match the container name and port defined in the definitions.
- A task role and a task execution role exist and can be assumed by ecs-tasks.amazonaws.com.
- Container images exist at the URL defined in task definitions. (Checks only for ECR or DockerHub public images.)
- Secrets in task definitions exist and be readable.
- Can create log streams, can put messages to the streams in specified CloudWatch log groups.

ecspresso verify tries to assume the task execution role defined in task definitions to verify these items. If failed to assume the role, it continues to verify with the current sessions.

```console
$ ecspresso verify
2020/12/08 11:43:10 nginx-local/ecspresso-test Starting verify
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
2020/12/08 11:43:14 nginx-local/ecspresso-test Verify OK!
```

### tasks

task command lists tasks run by a service or having the same family to a task definition.

```
Flags:
      --id=                       task ID
      --output=table              output format
      --find=false                find a task from tasks list and dump it as JSON
      --stop=false                stop the task
      --force=false               stop the task without confirmation
      --trace=false               trace the task
```

When `--find` option is set, you can select a task in a list of tasks and show the task as JSON.

`ECSPRESSO_FILTER_COMMAND` environment variable can define a command to filter tasks. For example [peco](https://github.com/peco/peco), [fzf](https://github.com/junegunn/fzf) and etc.

```console
$ ECSPRESSO_FILTER_COMMAND=peco ecspresso tasks --find
```

When `--stop` option is set, you can select a task in a list of tasks and stop the task.

### exec

exec command executes a command on task.

[session-manager-plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) is required in PATH.

```
Flags:
      --id=                       task ID
      --command=sh                command to execute
      --container=                container name
      --port-forward=false        enable port forward
      --local-port=0              local port number
      --port=0                    remote port number (required for --port-forward)
      --host=                     remote host (required for --port-forward)
```

If `--id` is not set, the command shows a list of tasks to select a task to execute.

`ECSPRESSO_FILTER_COMMAND` environment variable works the same as tasks command.

See also the official document [Using Amazon ECS Exec for debugging](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html).

### port forwarding

`ecspresso exec --port-forward` forwards local port to ECS tasks port.

```
$ ecspresso exec --port-forward --port 80 --local-port 8080
...
```

If `--id` is not set, the command shows a list of tasks to select a task to forward port.

When `--local-port` is not specified, use the ephemeral port for local port.

### suspend / resume application auto scaling

`ecspresso deploy` and `scale` can suspend / resume application auto scaling.

`--suspend-auto-scaling` sets suspended state true.
`--resume-auto-scaling` sets suspended state false.

When you want to change the suspended state simply, try `ecspresso scale --suspend-auto-scaling` or `ecspresso scale --resume-auto-scaling`. That operation will change suspended state only.

# Plugins

## tfstate

tfstate plugin introduces a template function `tfstate`.

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
      path: terraform.tfstate    # path to tfstate file
      # or url: s3://my-bucket/terraform.tfstate
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

`{{ tfstate "resource_type.resource_name.attr" }}` will expand to an attribute value of the resource in tfstate.

`{{ tfstatef "resource_type.resource_name['%s'].attr" "index" }}` is similar to `{{ tfstatef "resource_type.resource_name['index'].attr" }}`. This function is useful to build a resource address with environment variables.

```
{{ tfstatef `aws_subnet.ecs['%s'].id` (must_env `SERVICE`) }}
```

### Multiple tfstate support

`func_prefix` adds a prefix to template function names for each plugin configuration.

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

So in templates, functions are called with prefixes.

```json
[
  "{{ first_tfstate `aws_s3_bucket.main.arn` }}",
  "{{ second_tfstate `aws_s3_bucket.main.arn` }}"
]
``

## CloudFormation

cloudformation plugin introduces template functions `cfn_output` and `cfn_export`.

An example of CloudFormation stack template defines Outputs and Exports.

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

Load cloudformation plugin in a config file.

ecspresso.yml
```yaml
# ...
plugins:
  - name: cloudformation
```

`cfn_output StackName OutputKey` lookups OutputValue of OutputKey in the StackName.
`cfn_export ExportName` lookups exported value by name.

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

## ECS Service Connect

ecspresso supports [ECS Service Connect](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html).

You can define `serviceConnectConfiguration` in service definition files and `portMappings` attributes in task definition files.

For more details, see also [Service Connect parameters](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html#service-connect-parameters)

# LICENCE

MIT

# Author

KAYAC Inc.
