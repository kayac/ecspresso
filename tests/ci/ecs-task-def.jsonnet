{
  containerDefinitions: [
    {
      cpu: 1024,
      environment: [],
      essential: true,
#      image: 'nginx:{{ env `NGINX_VERSION` `latest` }}',
      image: 'mcr.microsoft.com/windows/servercore/iis:windowsservercore-ltsc2019',
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-group': 'ecspresso-test',
          'awslogs-region': 'ap-northeast-1',
          'awslogs-stream-prefix': 'windows',
        },
      },
      mountPoints: [],
      name: 'nginx',
      portMappings: [
        {
          containerPort: 80,
          hostPort: 80,
          protocol: 'tcp',
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
  cpu: '1024',
  executionRoleArn: 'arn:aws:iam::{{must_env `AWS_ACCOUNT_ID`}}:role/ecsTaskRole',
  family: 'ecspresso-test',
  memory: '2048',
  networkMode: 'awsvpc',
  placementConstraints: [],
  runtimePlatform: {
    operatingSystemFamily: 'WINDOWS_SERVER_2019_CORE',
  },
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
