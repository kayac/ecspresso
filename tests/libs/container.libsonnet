{
  name: 'katsubushi',
  mountPoints: [],
  portMappings: [
    {
      protocol: 'tcp',
      containerPort: 11212,
      hostPort: 11212,
    },
  ],
  logConfiguration: {
    logDriver: 'awslogs',
    options: {
      'awslogs-group': 'fargate',
      'awslogs-region': 'us-east-1',
      'awslogs-stream-prefix': 'katsubushi',
    },
  },
  image: 'katsubushi/katsubushi:{{ env `TAG` `latest` }}',
  dockerLabels: {
    name: 'katsubushi',
  },
  cpu: 256,
  ulimits: [
    {
      softLimit: 100000,
      name: 'nofile',
      hardLimit: 100000,
    },
  ],
  memory: 16,
  essential: true,
  volumesFrom: [],
}
