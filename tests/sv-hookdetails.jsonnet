local hook = {
  roleArn: 'arn:aws:iam::123456789012:role/hook',
};
{
  deploymentConfiguration: {
    strategy: 'BLUE_GREEN',
    bakeTimeInMinutes: 10,
    lifecycleHooks: [
      hook {
        hookTargetArn: 'arn:aws:lambda:us-east-1:123456789012:function:hook',
        lifecycleStages: ['POST_TEST_TRAFFIC_SHIFT'],
        hookDetails: {
          baseUrl: 'https://example.com',
          'X-Header': 'Keep-Case',
          ratio: 0.5,
          count: 3,
          enabled: true,
          checks: [{ path: '/health_check', status: 200 }],
        },
      },
      hook {
        hookTargetArn: 'arn:aws:lambda:us-east-1:123456789012:function:no-details',
        lifecycleStages: ['PRE_SCALE_UP'],
      },
    ],
  },
}
