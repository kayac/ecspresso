{
  containerDefinitions: [
    {
      cpu: 0,
      environment: [],
      essential: true,
      image: 'nginx:{{ env `NGINX_VERSION` `latest` }}',
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-group': 'ecspresso-test',
          'awslogs-region': 'ap-northeast-1',
          'awslogs-stream-prefix': 'nginx',
        },
      },
      mountPoints: [],
      name: 'nginx',
      portMappings: [
        {
          name: 'nginx',
          containerPort: 80,
          hostPort: 80,
          protocol: 'tcp',
          appProtocol: 'http',
        },
      ],
      secrets: [
        {
          name: 'FOO',
          valueFrom: '/ecspresso-test/foo',
        },
        {
          name: 'BAR',
          valueFrom: 'arn:aws:ssm:ap-northeast-1:{{must_env `AWS_ACCOUNT_ID`}}:parameter/ecspresso-test/bar',
        },
        {
          name: 'BAZ',
          valueFrom: 'arn:aws:secretsmanager:ap-northeast-1:{{must_env `AWS_ACCOUNT_ID`}}:secret:ecspresso-test/baz-06XQOH',
        },
      ],
      volumesFrom: [],
    },
  ],
  cpu: '512',
  executionRoleArn: 'arn:aws:iam::{{must_env `AWS_ACCOUNT_ID`}}:role/ecsTaskRole',
  family: 'nginx',
  memory: '1024',
  networkMode: 'awsvpc',
  placementConstraints: [],
  requiresCompatibilities: [
    'FARGATE',
  ],
  tags: [
    {
      key: 'TaskType',
      value: 'ecspresso-test',
    },
  ],
  taskRoleArn: 'arn:aws:iam::{{must_env `AWS_ACCOUNT_ID`}}:role/ecsTaskRole',
  volumes: [],
}
