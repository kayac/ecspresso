local isCodeDeploy = std.extVar('DEPLOYMENT_CONTROLLER') == 'CODE_DEPLOY';
{
  containerDefinitions: [
    {
      cpu: 0,
      environment: [
        {
          name: 'FOO_ENV',
          value: '{{ ssm `/ecspresso-test/foo` }}',
        },
        {
          name: 'BAZ_ARN',
          value: '{{ secretsmanager_arn `ecspresso-test/baz` }}',
        },
      ],
      essential: true,
      image: 'nginx:{{ env `NGINX_VERSION` `latest` }}',
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-create-group': 'true',
          'awslogs-group': 'ecspresso-test',
          'awslogs-region': 'ap-northeast-1',
          'awslogs-stream-prefix': 'nginx',
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
          name: 'FOO_SECRETS',
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
    {
      essential: true,
      image: '{{ must_env `AWS_ACCOUNT_ID` }}.dkr.ecr.us-east-1.amazonaws.com/bash:latest',
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-group': 'ecspresso-test',
          'awslogs-region': 'ap-northeast-1',
          'awslogs-stream-prefix': 'bash',
        },
      },
      name: 'bash',
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
          name: 'BAZ_ARN',
          valueFrom: '{{ secretsmanager_arn `ecspresso-test/baz` }}',
        },
        {
          name: 'JSON_FOO',
          valueFrom: 'arn:aws:secretsmanager:ap-northeast-1:{{must_env `AWS_ACCOUNT_ID`}}:secret:ecspresso-test/json-soBS7X:foo::',
        },
        {
          name: 'JSON_VIA_SSM',
          valueFrom: '{{ secretsmanager_arn `ecspresso-test/json` }}',
        },
      ],
      mountPoints: if isCodeDeploy then null else [
        {
          containerPath: '/mnt/ebs',
          sourceVolume: 'ebs',
        },
      ],
      command: [
        'tail',
        '-f',
        '/dev/null',
      ],
    },
  ],
  cpu: '256',
  ephemeralStorage: {
    sizeInGiB: 50,
  },
  executionRoleArn: 'arn:aws:iam::{{must_env `AWS_ACCOUNT_ID`}}:role/ecsTaskRole',
  family: 'ecspresso-test',
  memory: '512',
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
  volumes: if isCodeDeploy then null else [
    {
      name: 'ebs',
      configuredAtLaunch: true,
    },
  ],
}
