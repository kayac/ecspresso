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


## Usage

```
usage: ecspresso --config=CONFIG [<flags>] <command> [<args> ...]

Flags:
  --help           Show context-sensitive help (also try --help-long and --help-man).
  --config=CONFIG  config file

Commands:
  help [<command>...]
    Show help.

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
```

For more options for sub-commands, See `ecspresso sub-command --help`.

### Configuration file

YAML format.

```yaml
region: ap-northeast-1
cluster: default
service: myService
task_definition: myTask.json
timeout: 5m
```

ecspresso works as below.

- Register a new task definition from JSON file.
  - JSON file is allowed both of formats as below.
    - `aws ecs describe-task-definition` output.
    - `aws ecs register-task-definition --cli-input-json` input.
  - Replace ```{{ env `FOO` `bar` }}``` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, replaced by "bar"
  - Replace ```{{ must_env `FOO` }}``` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, abort immediately.
- Update a service definition.
- Wait a service stable.
- Run a new task.

## Example of deploy

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

## Scale out/in

To change desired count of the service, specify `--tasks` option.

If `--skip-task-definition` is set, task definition will not be registered.

```console
$ ecspresso deploy --config config.yaml --tasks 10 --skip-task-definition
```

## Example of create

escpresso can create service by `service_definition` JSON file and `task_definition`.

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

## Example of run task

```console
$ ecspresso run --config config.yaml --task-def=db-migrate.json
```

When `--task-def` is not set, use a task definition included in a service.

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

# LICENCE

MIT

# Author

KAYAC Inc.
