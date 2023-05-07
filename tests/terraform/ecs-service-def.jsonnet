{
  deploymentConfiguration: {
    deploymentCircuitBreaker: {
      enable: false,
      rollback: false,
    },
    maximumPercent: 200,
    minimumHealthyPercent: 100,
  },
  deploymentController: {
    type: 'ECS',
  },
  desiredCount: 1,
  enableECSManagedTags: false,
  enableExecuteCommand: true,
  healthCheckGracePeriodSeconds: 0,
  launchType: 'FARGATE',
  loadBalancers: [
    {
      containerName: 'nginx',
      containerPort: 80,
      targetGroupArn: '{{or (env `TARGET_GROUP_ARN` ``) (tfstate `aws_lb_target_group.http.arn`) }}',
    },
  ],
  networkConfiguration: {
    awsvpcConfiguration: {
      assignPublicIp: 'ENABLED',
      securityGroups: [
        '{{or (env `SECURITY_GROUP_ID` ``) (tfstate `aws_security_group.default.id`) }}',
      ],
      subnets: [
        '{{or (env `SUBNET_ID_AZ_A` ``) (tfstate `aws_subnet.public-a.id`) }}',
        '{{or (env `SUBNET_ID_AZ_C` ``) (tfstate `aws_subnet.public-c.id`) }}',
        '{{or (env `SUBNET_ID_AZ_D` ``) (tfstate `aws_subnet.public-d.id`) }}',
      ],
    },
  },
  platformFamily: 'Linux',
  platformVersion: 'LATEST',
  propagateTags: 'SERVICE',
  schedulingStrategy: 'REPLICA',
  tags: [
    {
      key: 'env',
      value: 'ecspresso',
    },
  ],
}
