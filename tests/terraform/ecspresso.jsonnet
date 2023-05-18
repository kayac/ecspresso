{
  region: '{{ must_env `AWS_REGION` }}',
  cluster: 'ecspresso',
  service: 'ecspresso',
  service_definition: 'ecs-service-def.jsonnet',
  task_definition: 'ecs-task-def.jsonnet',
  timeout: '10m0s',
  plugins: [
    {
      name: 'tfstate',
      config: {
        path: 'terraform.tfstate',
      },
    }
  ],
}
