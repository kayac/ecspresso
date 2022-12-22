local subnets = ['subnet-abcdef00', 'subnet-abcdef01'];
local sgs = ['sg-12345678', 'sg-23456789'];
local deploymentConfiguration = {
  maximumPercent: 200,
  minimumHealthyPercent: 50,
};

{
  deploymentConfiguration: deploymentConfiguration {
    deploymentCircuitBreaker: {
      enable: true,
      rollback: true,
    },
    alarms: {
      alarmNames: [
        'HighResponseLatencyAlarm',
      ],
      enable: true,
      rollback: true,
    },
  },
  desiredCount: 2,
  loadBalancers: [
    {
      containerName: 'test',
      containerPort: 9999,
      targetGroupArn: 'arn:aws:elasticloadbalancing:us-east-1:1111111111:targetgroup/test/12345678',
    },
  ],
  launchType: 'EC2',
  schedulingStrategy: 'REPLICA',
  networkConfiguration: {
    awsvpcConfiguration: {
      subnets: subnets,
      securityGroups: sgs,
      assignPublicIp: 'ENABLED',
    },
  },
  propagateTags: 'SERVICE',
  tags: [
    {
      key: 'cluster',
      value: 'default2',
    },
  ],
}
