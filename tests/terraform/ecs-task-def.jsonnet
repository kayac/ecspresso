{
  containerDefinitions: [
    {
      cpu: 0,
      essential: true,
      image: 'nginx:latest',
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-create-group': 'true',
          'awslogs-group': '{{ tfstate `aws_cloudwatch_log_group.main.name` }}',
          'awslogs-region': '{{ must_env `AWS_REGION` }}',
          'awslogs-stream-prefix': 'nginx',
        },
      },
      name: 'nginx',
      portMappings: [
        {
          appProtocol: '',
          containerPort: 80,
          hostPort: 80,
          protocol: 'tcp',
        },
      ],
    },
    {
      command: [
        'tail',
        '-f',
        '/dev/null',
      ],
      cpu: 0,
      essential: true,
      image: 'debian:bullseye-slim',
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-create-group': 'true',
          'awslogs-group': '{{ tfstate `aws_cloudwatch_log_group.main.name` }}',
          'awslogs-region': '{{ must_env `AWS_REGION` }}',
          'awslogs-stream-prefix': 'bash',
        },
      },
      name: 'bash',
      secrets: [
        {
          name: 'FOO',
          valueFrom: '{{ tfstate `aws_ssm_parameter.foo.name` }}'
        },
        {
          name: 'BAR',
          valueFrom: '{{ tfstate `aws_secretsmanager_secret.bar.arn` }}'
        },
        {
          name: 'JSON_KEY',
          valueFrom: '{{ tfstate `aws_secretsmanager_secret.json.arn` }}:key::'
        },
      ],
    },
  ],
  cpu: '256',
  ephemeralStorage: {
    sizeInGiB: 30,
  },
  executionRoleArn: '{{tfstate `aws_iam_role.ecs-task-execution.arn`}}',
  family: 'ecspresso',
  memory: '512',
  networkMode: 'awsvpc',
  requiresCompatibilities: [
    'FARGATE',
  ],
  tags: [
    {
      key: 'env',
      value: 'ecspresso',
    },
  ],
  taskRoleArn: '{{tfstate `aws_iam_role.ecs-task.arn`}}',
}
