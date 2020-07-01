# ecspresso

ecspresso is a deployment tool for Amazon ECS.

(pronounced same as "espresso")

## Install

### Homebrew (macOS only)

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
  ecspresso: fujiwara/ecspresso@0.0.3
  jobs:
    steps:
      - checkout
      - ecspresso/install:
          version: 0.13.5
      - run:
          command: |
            ecspresso deploy --config config.yaml
```

## Usage

```
usage: ecspresso --config=CONFIG [<flags>] <command> [<args> ...]

Flags:
  --help           Show context-sensitive help (also try --help-long and --help-man).
  --config=CONFIG  config file
  --debug          enable debug log

Commands:
  help [<command>...]
    Show help.

  version
    show version

  deploy [<flags>]
    deploy service

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

  init --region=REGION --service=SERVICE [<flags>]
    create service/task definition files by existing ECS service

  diff [<flags>]
    display diff for task definition compared with latest one on ECS
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

## Scale out/in

To change desired count of the service, specify `--tasks` option.

If `--skip-task-definition` is set, task definition will not be registered.

```console
$ ecspresso deploy --config config.yaml --tasks 10 --skip-task-definition
```

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
```

ecs-service-def.json
```json
{
  "networkConfiguration": {
    "awsvpcConfiguration": {
      "subnets": [
        "{{ tfstate `aws_subnet.private-a.id` }}"
      ],
      "securityGroups": [
        "{{ tfstate `data.aws_security_group.default.id` }}"
      ]
    }
  }
}
```

`{{ tfstate "resource_type.resource_name.attr" }}` will expand to an attribute value of the resource in tfstate.

# LICENCE

MIT

# Author

KAYAC Inc.
