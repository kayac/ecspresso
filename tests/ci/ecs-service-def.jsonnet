{
  capacityProviderStrategy: [
    {
      base: 1,
      capacityProvider: 'FARGATE',
      weight: 1,
    },
    {
      base: 0,
      capacityProvider: 'FARGATE_SPOT',
      weight: 2,
    },
  ],
  deploymentConfiguration: {
    deploymentCircuitBreaker: {
      enable: true,
      rollback: true,
    },
    maximumPercent: 200,
    minimumHealthyPercent: 100,
  },
  deploymentController: {
    type: 'ECS',
  },
  desiredCount: 2,
  enableECSManagedTags: true,
  enableExecuteCommand: true,
  healthCheckGracePeriodSeconds: 0,
  launchType: '',
  loadBalancers: [
    {
      containerName: 'katsubushi',
      containerPort: 8080,
      targetGroupArn: 'arn:aws:elasticloadbalancing:ap-northeast-1:314472643515:targetgroup/katsubushi-http/f42e9121de35dcb4',
    },
    {
      containerName: 'katsubushi',
      containerPort: 8081,
      targetGroupArn: 'arn:aws:elasticloadbalancing:ap-northeast-1:314472643515:targetgroup/katsubushi-grpc/66649fccd0a61675',
    },
  ],
  networkConfiguration: {
    awsvpcConfiguration: {
      assignPublicIp: 'ENABLED',
      securityGroups: [
        'sg-0a69199a34e15147a',
        'sg-0c09a8157ba2cfa22',
      ],
      subnets: [
        'subnet-0623adfcb3093f18f',
        'subnet-0376f113bbbc25742',
        'subnet-04b750544ddd71274',
      ],
    },
  },
  pendingCount: 0,
  platformFamily: 'Linux',
  platformVersion: 'LATEST',
  propagateTags: 'SERVICE',
  runningCount: 0,
  schedulingStrategy: 'REPLICA',
  tags: [
    {
      key: 'cluster',
      value: 'ecspresso-test',
    },
  ],
}
