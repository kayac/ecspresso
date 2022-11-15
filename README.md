# ecspresso

ecspresso is a deployment tool for Amazon ECS.

(pronounced same as "espresso")

## Documents

[ecspresso handbook](https://zenn.dev/fujiwara/books/ecspresso-handbook) (Japanese)

[ecspresso Advent Calendar 2020](https://adventar.org/calendars/5916) (Japanese)

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
  ecspresso: fujiwara/ecspresso@1.0.0
jobs:
  install:
    steps:
      - checkout
      - ecspresso/install:
          version: v1.6.0 # or latest
      - run:
          command: |
            ecspresso version
```

### GitHub Actions

Action kayac/ecspresso@v1 installs ecspresso binary for Linux into /usr/local/bin. This action runs install only.

```yml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: kayac/ecspresso@v1
        with:
          version: v1.6.0
      - run: |
          ecspresso deploy --config ecspresso.yml
```

Pass the parameter "latest" to use the latest version of ecspresso.

```yaml
      - uses: kayac/ecspresso@v1
        with:
          version: latest
```

## Usage

```
usage: ecspresso [<flags>] <command> [<args> ...]

Flags:
  --help                 Show context-sensitive help (also try --help-long and
                         --help-man).
  --config=CONFIG        config file
  --debug                enable debug log
  --envfile=ENVFILE ...  environment files
  --color                enable colored output

Commands:
  help [<command>...]
    Show help.

  version
    show version

  deploy [<flags>]
    deploy service

  scale [<flags>]
    scale service. equivalent to deploy --skip-task-definition
    --no-update-service

  refresh [<flags>]
    refresh service. equivalent to deploy --skip-task-definition
    --force-new-deployment --no-update-service

  create [<flags>]
    create service

  status [<flags>]
    show status of service

  rollback [<flags>]
    rollback service

  delete [<flags>]
    delete service

  run [<flags>]
    run task

  register [<flags>]
    register task definition

  wait
    wait until service stable

  init --service=SERVICE [<flags>]
    create service/task definition files by existing ECS service

  diff
    display diff for task definition compared with latest one on ECS

  appspec [<flags>]
    output AppSpec YAML for CodeDeploy to STDOUT

  verify [<flags>]
    verify resources in configurations

  render [<flags>]
    render config, service definition or task definition file to stdout

  tasks [<flags>]
    list tasks that are in a service or having the same family

  exec [<flags>]
    execute command in a task
```

For more options for sub-commands, See `ecspresso sub-command --help`.

## Quick Start

Using ecspresso, you can easily manage your existing/running ECS service by codes.

Try `ecspresso init` for your ECS service with option `--region`, `--cluster` and `--service`.

```console
$ ecspresso init --region ap-northeast-1 --cluster default --service myservice --config ecspresso.yml
2019/10/12 01:31:48 myservice/default save service definition to ecs-service-def.json
2019/10/12 01:31:48 myservice/default save task definition to ecs-task-def.json
2019/10/12 01:31:48 myservice/default save config to ecspresso.yml
```

You will be seeing the following files generated:  ecspresso.yml, ecs-service-def.json, and ecs-task-def.json.

If you see the files, you're ready to deploy the service using ecspresso!

```console
$ ecspresso deploy --config ecspresso.yml
```

## Configuration file

YAML format.

```yaml
region: ap-northeast-1
cluster: default
service: myService
task_definition: myTask.json
timeout: 5m
```

ecspresso deploy works as below.

- Register a new task definition from JSON file.
  - JSON file is allowed in both formats as below.
    - Output format of `aws ecs describe-task-definition` command.
    - Input format of `aws ecs register-task-definition --cli-input-json` command.
  - The ```{{ env `FOO` `bar` }}``` syntax in the JSON file will be replaced by the environment variable "FOO".
    - If "FOO" is not defined, it will be replaced by "bar"
  - The ```{{ must_env `FOO` }}``` syntax in the JSON file will be replaced by the environment variable "FOO".
    - If "FOO" is not defined, abort immediately.
- Update service tasks.
  - When `--update-service` option is set, service attributes are updated using the values defined in the service definition.
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

`ecspresso create` can create a service having CODE_DEPLOY deployment controller. See ecs-service-def.json below.

```json5
{
  "deploymentController": {
    "type": "CODE_DEPLOY"
  },
  # ...
}
```

Currently, ecspresso doesn't create any resources on CodeDeploy. You must create an application and a deployment group for your ECS service on CodeDeploy in other way.

`ecspresso deploy` creates a new deployment for CodeDeploy, which starts to run on CodeDeploy.

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

CodeDeploy appspec hooks can be defined in a config file. ecspresso automatically creates `Resources` and `version` elements in appspec on deploy.

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

To change the desired count of the service, specify `scale --tasks`.

```console
$ ecspresso scale --config ecspresso.yml --tasks 10
```

`scale` command is equivalent to `deploy --skip-task-definition --no-update-service`.

## Example of create

escpresso can create a new service using `service_definition` JSON file and `task_definition`.

```console
$ ecspresso create --config ecspresso.yml
...
```

```yaml
# ecspresso.yml
service_definition: service.json
```

example of the service.json:

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

When `--task-def` is not set, the task definition included in the service is used.

Other options for RunTask API are set by service attributes(CapacityProviderStrategy, LaunchType, PlacementConstraints, PlacementStrategy and PlatformVersion).

# Notes

## Use Jsonnet instead of JSON

ecspresso v1.7 or later can use [Jsonnet](https://jsonnet.org/) file format for service and task definition.

If the file extension is .jsonnet, ecspresso will first process the file as Jsonnet, convert it to JSON, and then load it.

```yaml
service_definition: ecs-service-def.jsonnet
task_definition: ecs-task-def.jsonnet
```

As ecspresso includes [github.com/google/go-jsonnet](https://github.com/google/go-jsonnet) as a library, jsonnet command is not required.

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

When deploying to Fargate, there are some required fields for task definitions and service definitions.

For task definitions,

- requiresCompatibilities (must be "FARGATE")
- networkMode (must be "awsvpc")
- cpu (required)
- memory (required)
- executionRoleArn (optional)

```json5
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

- launchType (must be "FARGATE")
- networkConfiguration (must be "awsvpcConfiguration")

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
1. If you are to migrate existing service to use Fargate Spot, define capacityProviderStrategy into service definition as below. `ecspresso deploy --update-service` applies the settings to the service.

```json5
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
  # ...
```

## How to check diff and verify service/task definitions before deploy.

ecspresso supports `diff` and `verify` subcommands.

### diff

Shows differences between local task/service definitions and remote (on ECS) definitions.

```diff
$ ecspresso --config ecspresso.yml diff --unified
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

Verify resources related to service/task definitions.

For example,
- Whether the ECS cluster exists.
- Whether the target groups in service definitions match the container name and port defined in the definitions.
- Whether the task role and the task execution role exist and can be assumed by ecs-tasks.amazonaws.com.
- Whether the container images exist at the URL defined in task definitions. (Checks only for ECR or DockerHub public images.)
- Whether the secrets in task definitions exist and are readable.
- Whether it can create log streams, can put messages to the streams in the specified CloudWatch log groups.

ecspresso verify tries to assume-role the task execution role defined in task definitions to verify these items. If the assume-role fails, the verification continues using the current session.

```console
$ ecspresso --config ecspresso.yml verify
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

`task` subcommand lists tasks run by a service or having the same family to a task definition.

```
Flags:
  --id=""                task ID
  --output=table         output format (table|json|tsv)
  --find                 find a task from tasks list and dump it as JSON
  --stop                 stop a task
  --force                stop a task without confirmation prompt
```

When `--find` option is set, you can select a task in a list of tasks and show the task as JSON.

`filter_command` in ecspresso.yml can define a command to filter tasks. For example [peco](https://github.com/peco/peco), [fzf](https://github.com/junegunn/fzf) and etc.

```yaml
filter_command: peco
```

When `--stop` option is set, you can select a task in a list of tasks and stop the task.

### exec

`exec` subcommand executes a command on task.

[session-manager-plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) is required in PATH.

```
Flags:
  --id=""                task ID
  --command="sh"         command
  --container=CONTAINER  container name
```

If `--id` is not set, the command shows the list of tasks, which could be used to select which task to exec on.

`filter_command` in ecspresso.yml works the same as `tasks` subcommand.

See also the official document [Using Amazon ECS Exec for debugging](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html).

### port forwarding

`ecspresso exec --port-forward` forwards local port to ECS tasks port.

```
$ ecspresso exec --port-forward --port 80 --local-port 8080
...
```

If `--id` is not set, the command shows the list of tasks, which could be used to select which task to forward port.

When `--local-port` is not specified, ephemeral port will be used for the local port.

#### remort port forwarding

ecspresso also supports "remote port forwarding". `--host` flag accepts remote hostname to port forwarding.

```
$ ecspresso exec --port-forward --port 80 --local-port 8080 --host example.com
```

### suspend / resume application auto scaling

`ecspresso deploy` and `scale` can suspend / resume application auto scaling.

`--suspend-auto-scaling` sets suspended state true.
`--resume-auto-scaling` sets suspended state false.

When you simply want to change the suspended state, try `ecspresso scale --suspend-auto-scaling` or `ecspresso scale --resume-auto-scaling`. That operation will only change the suspended state.

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

`{{ tfstate "resource_type.resource_name.attr" }}` will be expanded to the attribute value of the resource in tfstate.

`{{ tfstatef "resource_type.resource_name['%s'].attr" "index" }}` is similar to `{{ tfstatef "resource_type.resource_name['index'].attr" }}`. This function is useful to build a resource address with environment variables.

```
{{ tfstatef `aws_subnet.ecs['%s'].id` (must_env `SERVICE`) }}
```

## cloudformation

cloudformation plugin introduces template functions `cfn_output` and `cfn_export`.

An example of CloudFormation stack template that defines Outputs and Exports is:

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

The cloudformation plugin needs to be loaded in the config file.

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

# LICENCE

MIT

# Author

KAYAC Inc.
