local container = import 'libs/container.libsonnet';
{
  networkMode: 'awsvpc',
  family: 'katsubushi',
  placementConstraints: [],
  requiresCompatibilities: [
    'FARGATE',
  ],
  volumes: [],
  taskRoleArn: 'arn:aws:iam::999999999999:role/ecsTaskRole',
  executionRoleArn: 'arn:aws:iam::999999999999:role/ecsTaskRole',
  ephemeralStorage: {
    sizeInGiB: std.extVar('EphemeralStorage'),
  },
  containerDefinitions: [
    container + {
      environment: [
        {
          name: 'worker_id',
          value: std.extVar('WorkerID'),
        },
      ],
    }
  ],
  cpu: '1024',
  memory: '2048',
  proxyConfiguration: {
    type: 'APPMESH',
    containerName: 'envoy',
    properties: [
      {
        name: 'IgnoredUID',
        value: '1337',
      },
      {
        name: 'IgnoredGID',
        value: '',
      },
      {
        name: 'AppPorts',
        value: '26571',
      },
      {
        name: 'ProxyIngressPort',
        value: '15000',
      },
      {
        name: 'ProxyEgressPort',
        value: '15001',
      },
      {
        name: 'EgressIgnoredIPs',
        value: '169.254.170.2,169.254.169.254',
      },
      {
        name: 'EgressIgnoredPorts',
        value: '',
      },
    ],
  },
  tags: [],
}
