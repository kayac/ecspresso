## An example of ecspresso deployment with terraform

This example shows how to deploy an ECS service by ecspresso with terraform.

### Prerequisites

- [Terraform](https://www.terraform.io/) >= v1.0.0
- [ecspresso](https://github.com/kayac/ecspresso) >= v2.0.0

#### Environment variables

- `AWS_REGION` for AWS region. (e.g. `ap-northeast-1`)
- `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`, or `AWS_PROFILE` for AWS credentials.
- `AWS_SDK_LOAD_CONFIG=true` may be required if you use `AWS_PROFILE` and `~/.aws/config`.

### Usage

```console
$ terraform init
$ terraform apply
$ ecspresso deploy
```

After completing the deployment, you can access the service via ALB.

```console
$ curl -s "http://$(terraform output -raw alb_dns_name)/"
```

### Cleanup

```console
$ ecspresso delete --terminate
$ terraform destroy
```
