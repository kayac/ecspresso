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
