# ecspresso

ecspresso is a deployment tool for Amazon ECS.

(pronounced same as "espresso")

# Usage

```
$ ecspresso
  -cluster string
    	ECS cluster name(required)
  -service string
    	ECS service name(required)
  -task-definition string
    	task definition path(required)
  -timeout int
    	timeout (sec) (default 180)
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

# Requirements

- aws-cli

# LICENCE

MIT

# Author

KAYAC Inc.
