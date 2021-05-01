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
  ecspresso: fujiwara/ecspresso@0.0.12
jobs:
  install:
    steps:
      - checkout
      - ecspresso/install:
          version: v1.1.3
      - run:
          command: |
            ecspresso version
```

### GitHub Actions

Action kayac/ecspresso@v0 installs ecspresso binary for Linux into /usr/local/bin. This action runs install only.

```yml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: kayac/ecspresso@v0
        with:
          version: v1.1.3
      - run: |
          ecspresso deploy --config config.yaml
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

ecspresso can easily manage for your existing/running ECS service by codes.

Try `ecspresso init` for your ECS service with option `--region`, `--cluster` and `--service`.

```console
$ ecspresso init --region ap-northeast-1 --cluster default --service myservice --config config.yaml
2019/10/12 01:31:48 myservice/default save service definition to ecs-service-def.json
2019/10/12 01:31:48 myservice/default save task definition to ecs-task-def.json
2019/10/12 01:31:48 myservice/default save config to config.yaml
```

Let me see the generated files config.yaml, ecs-service-def.json, and ecs-task-def.json.

And then, you already can deploy the service by ecspresso!

```console
$ ecspresso deploy --config config.yaml
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
  - JSON file is allowed both of formats as below.
    - `aws ecs describe-task-definition` output.
    - `aws ecs register-task-definition --cli-input-json` input.
  - Replace ```{{ env `FOO` `bar` }}``` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, replaced by "bar"
  - Replace ```{{ must_env `FOO` }}``` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, abort immediately.
- Update a service tasks.
  - When `--update-service` option set, update service attributes by service definition.
- Wait a service stable.

Configuration files and task/service definition files are read by [go-config](https://github.com/kayac/go-config). go-config has template functions `env`, `must_env` and `json_escape`.

## Example of deployment

### Rolling deployment

```console
$ ecspresso deploy --config config.yaml
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

Currently, ecspresso doesn't create any resources on CodeDeploy. You must create an application and a deployment group for your ECS service on CodeDeploy in the other way.

`ecspresso deploy` creates a new deployment for CodeDeploy, and it continues on CodeDeploy.

```console
$ ecspresso deploy --config config.yaml --rollback-events DEPLOYMENT_FAILURE
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
$ ecspresso scale --config config.yaml --tasks 10
```

`scale` command is equivalent to `deploy --skip-task-definition --no-update-service`.

## Example of create

escpresso can create a service by `service_definition` JSON file and `task_definition`.

```console
$ ecspresso create --config config.yaml
...
```

```yaml
# config.yaml
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

Keys are same format as `aws ecs describe-services` output.

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
$ ecspresso run --config config.yaml --task-def=db-migrate.json
```

When `--task-def` is not set, use a task definition included in a service.

Other options for RunTask API are set by service attributes(CapacityProviderStrategy, LaunchType, PlacementConstraints, PlacementStrategy and PlatformVersion).

# Notes

## Deploy to Fargate

If you want to deploy services to Fargate, task-definition and service-definition requires some settings.

For task definition,

- requiresCompatibilities (required "FARGATE")
- networkMode (required "awsvpc")
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

Shows defferencies between local task/service definitions and remote (on ECS) definitions.

```diff
$ ecspresso --config config.yaml diff
--- arn:aws:ecs:ap-northeast-1:123456789012:service/ecspresso-test/nginx-local
+++ ecs-service-def.json
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
       "weight": 2
     }
   ],
   "deploymentConfiguration": {
     "deploymentCircuitBreaker": {
       "enable": true,
       "rollback": true
     },
     "maximumPercent": 200,
     "minimumHealthyPercent": 100
   },
   "networkConfiguration": {
     "awsvpcConfiguration": {
       "assignPublicIp": "ENABLED",
       "securityGroups": [
         "sg-0a69199a34e15147a"
       ],
       "subnets": [
         "subnet-0376f113bbbc25742",
         "subnet-04b750544ddd71274",
         "subnet-0623adfcb3093f18f"
       ]
     }
   },
   "placementConstraints": [],
   "placementStrategy": [],
-  "platformVersion": "1.3.0"
+  "platformVersion": "LATEST"
 }
 
--- arn:aws:ecs:ap-northeast-1:123456789012:task-definition/ecspresso-test:202
+++ ecs-task-def.json
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
           "awslogs-group": "ecspresso-test",
           "awslogs-region": "ap-northeast-1",
           "awslogs-stream-prefix": "nginx"
         }
       },
       "mountPoints": [],
       "name": "nginx",
       "portMappings": [
         {
           "containerPort": 80,
           "hostPort": 80,
           "protocol": "tcp"
         }
       ],
       "secrets": [],
       "volumesFrom": []
     }
   ],
   "cpu": "256",
   "executionRoleArn": "arn:aws:iam::123456789012:role/ecsTaskRole",
   "family": "ecspresso-test",
   "memory": "512",
   "networkMode": "awsvpc",
   "placementConstraints": [],
   "requiresCompatibilities": [
     "EC2",
     "FARGATE"
   ],
   "taskRoleArn": "arn:aws:iam::123456789012:role/ecsTaskRole",
   "volumes": []
 }
```

### verify

Verify resources which related with service/task definitions.

For example,
- An ECS cluster exists.
- The target groups in service definitions matches container name and port defined in task definition.
- A task role and a task execution role exist and can be assumed by ecs-tasks.amazonaws.com.
- Container images exist at URL defined in task definition. (Checks only for ECR or DockerHub public images.)
- Secrets in task definition exist and be readable.
- Can create log streams, can put messages to the streams in specified CloudWatch log groups.

ecspresso verify tries to assume role to task execution role defined in task definition to verify these items. If faild to assume role, continue to verify with the current sessions.

```console
$ ecspresso --config config.yaml verify
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

task command lists tasks that run by a service or having the same family to a task definition.

```
Flags:
  --id=""                task ID
  --output=table         output format (table|json|tsv)
  --find                 find a task from tasks list and dump it as JSON
  --stop                 stop a task
  --force                stop a task without confirmation prompt
```

When `--find` option is set, you can select a task in a list of tasks and show the task as JSON.

`filter_command` in config.yaml can define a command to filter tasks. For example [peco](https://github.com/peco/peco), [fzf](https://github.com/junegunn/fzf) and etc.

```yaml
filter_command: peco
```

When `--stop` option is set, you can select a task in a list of tasks and stop the task.

### exec

exec command executes a command on task.

[session-manager-plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) is required in PATH.

```
Flags:
  --id=""                task ID
  --command="sh"         command
  --container=CONTAINER  container name
```

If `--id` is not set, the command shows a list of tasks to select a task to execute.

`filter_command` in config.yaml works ths same as tasks command.

See also the official document [Using Amazon ECS Exec for debugging](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html).

### suspend / resume application auto scaling

`ecspresso deploy` and `scale` can suspend / resume application auto scaling.

`--suspend-auto-scaling` sets suspended state true.
`--resume-auto-scaling` sets suspended state false.

When you want to change the suspended state simply, try `ecspresso scale --suspend-auto-scaling` or `ecspresso scale --resume-auto-scaling`. That operation will change suspended state only.

# Plugins

## tfstate

tfstate plugin introduces a template function `tfstate`.

config.yaml
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

## cloudformation

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

config.yaml
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
