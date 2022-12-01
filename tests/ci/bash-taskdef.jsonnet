{
  containerDefinitions: [
    {
      essential: true,
      image: 'debian:buster-slim',
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-group': 'ecspresso-test',
          'awslogs-region': 'ap-northeast-1',
          'awslogs-stream-prefix': 'bash',
        },
      },
      name: 'bash',
      command: [
        'tail',
        '-f',
        '/dev/null',
      ],
    },
  ],
  cpu: '512',
  executionRoleArn: 'arn:aws:iam::{{must_env `AWS_ACCOUNT_ID`}}:role/ecsTaskRole',
  family: 'bash',
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
