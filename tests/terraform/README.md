## An example of deployments by ecspresso and Terraform

This example shows how to deploy an ECS service by ecspresso and Terraform.

### Prerequisites

- [Terraform](https://www.terraform.io/) >= v1.4.0
- [ecspresso](https://github.com/kayac/ecspresso) >= v2.0.0

#### Environment variables

- `AWS_REGION` for AWS region. (e.g. `ap-northeast-1`)
- `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`, or `AWS_PROFILE` for AWS credentials.
- `AWS_SDK_LOAD_CONFIG=true` may be required if you use `AWS_PROFILE` and `~/.aws/config`.

### Usage

Terraform creates AWS resources (VPC, Subnets, ALB, IAM roles, ECS cluster, etc.) for ECS service working with ALB. And ecspresso deploys the service into the ECS cluster.

```console
$ terraform init
$ terraform apply
$ ecspresso verify
$ ecspresso deploy
```

After completing the deployment, you can access the service via ALB.

```console
$ curl -s "http://$(terraform output -raw alb_dns_name)/"
```

### Usage with AWS CodeDeploy

At first, you must delete an ECS service that having "ECS" deployment controller.

```console
$ ecspresso delete --force --terminate
```

Note: After the ECS service is deleted, wait a few minutes until the ECS service is completely removed. While `ecspresso status` reports `DRAINING`, you cannot create a new ECS service with the same name. After `ecspresso status` reports `is INACTIVE`, you can continue to the next step.

Edit `ecs-service-def.jsonnet`. Remove `deploymentCircuitBreaker` block and change `deployment_controller` to `CODE_DEPLOY`.

```diff
{
   deploymentConfiguration: {
-    deploymentCircuitBreaker: {
-      enable: false,
-      rollback: false,
-    },
     maximumPercent: 200,
     minimumHealthyPercent: 100,
   },
   deploymentController: {
-    type: 'ECS',
+    type: 'CODE_DEPLOY',
   },
```

Then deploy the service again.

```console
$ ecspresso deploy
```

After completing the deployment, you have to create a CodeDeploy application and deployment group.
Uncomment [codedeploy.tf](./codedeploy.tf) and run `terraform apply` again.

Now you can deploy the service by ecspresso using CodeDeploy!

```console
$ ecspresso deploy
```

### Cleanup

You must delete the ECS service and tasks first. And then, you can delete the resources created by Terraform.

```console
$ ecspresso delete --terminate
$ terraform destroy
```
