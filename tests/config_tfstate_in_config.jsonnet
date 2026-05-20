// Demonstrates resolving config-level fields (cluster, service) from
// the tfstate plugin. ecspresso evaluates the jsonnet file in two
// passes: pass 1 extracts only `plugins` (lazy field access — the
// `cluster` / `service` expressions below are not touched), pass 2
// runs after plugins are initialised so std.native('tfstate') works.
local must_env = std.native('must_env');
local tfstate = std.native('tfstate');

{
  region: must_env('AWS_REGION'),
  cluster: tfstate('aws_ecs_cluster.main.name'),
  service: 'test',
  service_definition: 'ecs-service-def.jsonnet',
  task_definition: 'ecs-task-def.jsonnet',
  timeout: '10m0s',
  plugins: [
    {
      name: 'tfstate',
      config: {
        path: 'terraform.tfstate',
      },
    },
  ],
}
