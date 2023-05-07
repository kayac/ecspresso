{
  deploymentConfiguration: {
    // remove deploymentCircuitBreaker when deployment controller is CODE_DEPLOY
    deploymentCircuitBreaker: {
      enable: false,
      rollback: false,
    },
    maximumPercent: 200,
    minimumHealthyPercent: 100,
  },
  deploymentController: {
    type: 'ECS', // ECS or CODE_DEPLOY
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
      targetGroupArn: "{{ tfstate `aws_lb_target_group.http['alpha'].arn` }}",
    },
  ],
  networkConfiguration: {
    awsvpcConfiguration: {
      assignPublicIp: 'ENABLED',
      securityGroups: [
        '{{ tfstate `aws_security_group.default.id` }}',
      ],
      subnets: [
        '{{ tfstate `aws_subnet.public-a.id` }}',
        '{{ tfstate `aws_subnet.public-c.id` }}',
        '{{ tfstate `aws_subnet.public-d.id` }}',
      ],
    },
  },
  platformFamily: 'Linux',
  platformVersion: '1.4.0',
  propagateTags: 'SERVICE',
  schedulingStrategy: 'REPLICA',
  tags: [
    {
      key: 'env',
      value: 'ecspresso',
    },
  ],
}
