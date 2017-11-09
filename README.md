# ecspresso

ecspresso is a deployment tool for Amazon ECS.

(pronounced same as "espresso")

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

  status [<flags>]
    show status of service

  rollback [<flags>]
    rollback service
```

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
  - JSON file is same format as `aws ecs describe-task-definition` output.
  - Replace `{{ env "FOO" "bar" }}` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, replaced by "bar"
  - Replace `{{ must_env "FOO" }}` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, abort immediately.
- Update a service definition.
- Wait a service stable.

## Example

```
$ ecspresso deploy --config preview.yaml
2017/11/09 23:20:13 myService/default Starting deploy
Service: myService
Cluster: default
TaskDefinition: myService:3
Deployments:
    PRIMARY myService:3 desired:1 pending:0 running:1
Events:
2017/11/09 23:20:13 myService/default Creating a new task definition by task-definition/myService.json
2017/11/09 23:20:13 myService/default Registering a new task definition...
2017/11/09 23:20:13 myService/default Task definition is registered myService:4
2017/11/09 23:20:13 myService/default Updating service...
2017/11/09 23:20:13 myService/default Waiting for service stable...(it will take a few minutes)
2017/11/09 23:23:23 myService/default  PRIMARY myService:4 desired:1 pending:0 running:1
2017/11/09 23:23:29 myService/default Service is stable now. Completed!
```

# LICENCE

MIT

# Author

KAYAC Inc.
