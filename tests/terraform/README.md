## An example of deployments by ecspresso and terraform

This example shows how to deploy an ECS service by ecspresso and terraform.

### Prerequisites

- [Terraform](https://www.terraform.io/) >= v1.4.0
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

### Usage with AWS CodeDeploy

At first, you must delete an ECS service that having "ECS" deployment controller.

```console
$ ecspresso delete --force --terminate
```

Remove `deploymentCircuitBreaker` block and change `deployment_controller` in `ecs-service-def.jsonnet` to `CODE_DEPLOY`.

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

Finally, now you can deploy the service by ecspresso using CodeDeploy.

```console
$ ecspresso deploy
```

### Cleanup

```console
$ ecspresso delete --terminate
$ terraform destroy
```
