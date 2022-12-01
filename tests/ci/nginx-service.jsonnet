{
  deploymentConfiguration: {
    deploymentCircuitBreaker: {
      enable: true,
      rollback: true,
    },
    maximumPercent: 200,
    minimumHealthyPercent: 100,
  },
  desiredCount: 1,
  enableECSManagedTags: false,
  enableExecuteCommand: true,
  healthCheckGracePeriodSeconds: 0,
  loadBalancers: [
    {
      containerName: 'nginx',
      containerPort: 80,
      targetGroupArn: 'arn:aws:elasticloadbalancing:ap-northeast-1:{{must_env `AWS_ACCOUNT_ID`}}:targetgroup/alpha/6a301850702273d9',
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
  placementConstraints: [],
  placementStrategy: [],
  platformVersion: 'LATEST',
  schedulingStrategy: 'REPLICA',
  serviceConnectConfiguration: {
    namespace: 'ecspresso-test',
    enabled: true,
    logConfiguration: {
      logDriver: 'awslogs',
      options: {
        'awslogs-group': 'ecspresso-test',
        'awslogs-region': 'ap-northeast-1',
        'awslogs-stream-prefix': 'sc',
      },
    },
    services: [
      {
        clientAliases: [
          {
            dnsName: 'nginx.local',
            port: 80,
          },
        ],
        portName: 'nginx',
        discoveryName: 'nginx-server',
      },
    ],
  },
  serviceRegistries: [],
  propagateTags: 'SERVICE',
  tags: [
    {
      key: 'cluster',
      value: 'ecspresso-test',
    },
  ],
}
