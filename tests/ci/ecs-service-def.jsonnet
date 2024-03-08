local isCodeDeploy = std.extVar('DEPLOYMENT_CONTROLLER') == 'CODE_DEPLOY';
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
    deploymentCircuitBreaker: if isCodeDeploy then null else {
      enable: false,
      rollback: false,
    },
    maximumPercent: 200,
    minimumHealthyPercent: 100,
  },
  deploymentController: {
    type: '{{ env `DEPLOYMENT_CONTROLLER` `ECS` }}',
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
  platformVersion: '1.4.0',
  schedulingStrategy: 'REPLICA',
  serviceRegistries: [],
  propagateTags: 'SERVICE',
  tags: [
    {
      key: 'deployed_at',
      value: '{{ env `NOW` `` }}',
    },
    {
      key: 'cluster',
      value: 'ecspresso-test',
    },
  ],
  volumeConfigurations: if isCodeDeploy then null else [
    {
      managedEBSVolume: {
        filesystemType: 'ext4',
        roleArn: 'arn:aws:iam::{{ must_env `AWS_ACCOUNT_ID` }}:role/ecsInfrastructureRole',
        sizeInGiB: 10,
        tagSpecifications: [
          {
            propagateTags: 'SERVICE',
            resourceType: 'volume',
          },
          {
            propagateTags: 'TASK_DEFINITION',
            resourceType: 'volume',
          },
        ],
        volumeType: 'gp3',
      },
      name: 'ebs',
    },
  ],
}
