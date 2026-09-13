# Changelog

## [v2.8.7](https://github.com/kayac/ecspresso/compare/v2.8.6...v2.8.7) - 2026-09-13
- Bump google.golang.org/grpc from 1.83.1 to 1.83.2 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/1079
- Support early success criteria for rolling deployments by @fujiwara in https://github.com/kayac/ecspresso/pull/1080
- Fill deploymentConfiguration defaults in diff by @fujiwara in https://github.com/kayac/ecspresso/pull/1082

## [v2.8.6](https://github.com/kayac/ecspresso/compare/v2.8.5...v2.8.6) - 2026-09-04
- Bump golang.org/x/crypto from 0.51.0 to 0.52.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/1056
- Support --wait-until ecs:<lifecycle stage> by @draftcode in https://github.com/kayac/ecspresso/pull/1063
- Bump google.golang.org/grpc from 1.80.0 to 1.82.1 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/1062
- Harden --wait-until ecs:<lifecycle stage> by @fujiwara in https://github.com/kayac/ecspresso/pull/1064
- Add documentation for --wait-until to README by @fujiwara in https://github.com/kayac/ecspresso/pull/1066
- Note that a pause hook at an earlier stage blocks the ecs:* wait by @fujiwara in https://github.com/kayac/ecspresso/pull/1068
- Warn when a pause hook blocks the ecs:* lifecycle stage wait by @fujiwara in https://github.com/kayac/ecspresso/pull/1069
- Bump GitHub Actions by @fujiwara in https://github.com/kayac/ecspresso/pull/1072
- Bump Go module dependencies by @fujiwara in https://github.com/kayac/ecspresso/pull/1073

## [v2.8.5](https://github.com/kayac/ecspresso/compare/v2.8.4...v2.8.5) - 2026-07-07
- Add `optional: true` to tfstate plugin by @fujiwara in https://github.com/kayac/ecspresso/pull/1017
- Add App.HasDiff for library callers by @fujiwara in https://github.com/kayac/ecspresso/pull/1019
- Embed Version in source via tagpr versionFile by @fujiwara in https://github.com/kayac/ecspresso/pull/1022
- Bump github.com/fujiwara/tfstate-lookup from 1.12.0 to 1.12.1 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/1021
- Bump aws-sdk-go-v2 modules (ecs, cloudwatchlogs, s3, vpclattice) by @fujiwara in https://github.com/kayac/ecspresso/pull/1030
- Fix error handling in CircleCI orb install command by @mi-wada in https://github.com/kayac/ecspresso/pull/1043
- Support monitoring configuration for high resolution CloudWatch metrics by @fujiwara in https://github.com/kayac/ecspresso/pull/1045
- Bump golang.org/x/net from 0.53.0 to 0.55.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/1050
- Fix verify 404 for images pinned as repo:tag@sha256:digest by @fujiwara in https://github.com/kayac/ecspresso/pull/1054
- Fix panic in verify for PAUSE lifecycle hooks by @fujiwara in https://github.com/kayac/ecspresso/pull/1055

## [v2.8.4](https://github.com/kayac/ecspresso/compare/v2.8.3...v2.8.4) - 2026-05-16
- fix: skip draft releases when resolving latest in install script by @fujiwara in https://github.com/kayac/ecspresso/pull/1007
- Expose plugin runtime instances via App.PluginInstance by @fujiwara in https://github.com/kayac/ecspresso/pull/1015
- Bump dependencies by @fujiwara in https://github.com/kayac/ecspresso/pull/1016

## [v2.8.3](https://github.com/kayac/ecspresso/compare/v2.8.2...v2.8.3) - 2026-04-28
- fix: handle unexpected response from /releases API in install script by @ppluuums-jp in https://github.com/kayac/ecspresso/pull/1002
- fix: surface non-2xx errors from /releases API in install script by @fujiwara in https://github.com/kayac/ecspresso/pull/1005
- docs: note v2-action-testing branch for action.yml CI by @fujiwara in https://github.com/kayac/ecspresso/pull/1006
- Bump github.com/aws/smithy-go from 1.25.0 to 1.25.1 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/999
- Bump fujiwara/ecsta to v0.8.2 (fix #1000) by @fujiwara in https://github.com/kayac/ecspresso/pull/1003
- Bump the aws-sdk-go-v2 group with 4 updates by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/998

## [v2.8.2](https://github.com/kayac/ecspresso/compare/v2.8.1...v2.8.2) - 2026-04-25
- fix: preserve remote ServiceArn when updating service tags by @tzmfreedom in https://github.com/kayac/ecspresso/pull/996
- Improve error handling for GetLogEvents in run-task by @ken39arg in https://github.com/kayac/ecspresso/pull/992
- Bump github.com/mattn/go-shellwords from 1.0.12 to 1.0.13 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/995
- Bump github.com/aws/smithy-go from 1.24.3 to 1.25.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/994
- Bump the aws-sdk-go-v2 group with 17 updates by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/993

## [v2.8.1](https://github.com/kayac/ecspresso/compare/v2.8.0...v2.8.1) - 2026-04-17
- Fix --force-new-deployment being ignored for service attribute changes by @fujiwara in https://github.com/kayac/ecspresso/pull/978
- Bump github.com/go-jose/go-jose/v4 from 4.1.3 to 4.1.4 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/976
- Bump aws-sdk-go-v2 and tfstate-lookup dependencies by @fujiwara in https://github.com/kayac/ecspresso/pull/986
- Bump github.com/fatih/color from 1.18.0 to 1.19.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/970
- Bump github.com/aws/smithy-go from 1.24.2 to 1.24.3 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/980
- Bump actions/setup-go from 6.3.0 to 6.4.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/974
- Bump docker/setup-qemu-action from 3.7.0 to 4.0.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/975
- Bump docker/setup-buildx-action from 3.12.0 to 4.0.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/973
- Bump golang.org/x/sys from 0.41.0 to 0.42.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/972
- Bump Songmu/tagpr to v1.18.1 by @fujiwara in https://github.com/kayac/ecspresso/pull/987
- Add S3 Files volume support documentation by @fujiwara in https://github.com/kayac/ecspresso/pull/988
- Bump fujiwara/ecsta to v0.8.1 by @fujiwara in https://github.com/kayac/ecspresso/pull/989
- Bump github.com/hashicorp/go-version from 1.8.0 to 1.9.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/991
- Bump github.com/aws/aws-sdk-go-v2/service/ecs from 1.77.0 to 1.78.0 in the aws-sdk-go-v2 group by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/990

## [v2.8.0](https://github.com/kayac/ecspresso/compare/v2.7.1...v2.8.0) - 2026-03-28
- Use structured slog attributes for log messages by @fujiwara in https://github.com/kayac/ecspresso/pull/939
- Add docs subcommand and LLM agent skill guide by @fujiwara in https://github.com/kayac/ecspresso/pull/937
- Add --list flag to docs command by @fujiwara in https://github.com/kayac/ecspresso/pull/941
- Bump actions/checkout from 6.0.0 to 6.0.2 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/935
- Bump Songmu/tagpr from 1.9.0 to 1.14.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/934
- Bump actions/setup-go from 6.1.0 to 6.2.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/933
- Bump docker/setup-buildx-action from 3.11.1 to 3.12.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/928
- Bump github.com/google/go-cmp from 0.6.0 to 0.7.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/925
- Bump github.com/fujiwara/ecsta from 0.4.5 to 0.7.4 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/924
- Bump github.com/alecthomas/kong from 1.10.0 to 1.13.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/922
- Bump the aws-sdk-go-v2 group with 17 updates by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/921
- Update olekukonko/tablewriter to v1 and ecsta to v0.8.0 by @fujiwara in https://github.com/kayac/ecspresso/pull/942
- Ignore targetGroupArn diff for CodeDeploy services by @fujiwara in https://github.com/kayac/ecspresso/pull/943
- Add --jsonnet flag to diff command by @fujiwara in https://github.com/kayac/ecspresso/pull/944
- Fix waiting for stale deployment by @fujiwara in https://github.com/kayac/ecspresso/pull/945
- Refactor tasks and exec to use subcommands by @fujiwara in https://github.com/kayac/ecspresso/pull/950
- Bump github.com/hashicorp/go-version from 1.7.0 to 1.8.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/949
- Bump github.com/goccy/go-yaml from 1.19.0 to 1.19.2 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/948
- feat: support --with-service/--without-service flag for diff command by @neilkuan in https://github.com/kayac/ecspresso/pull/962
- Fix duplicate ECS service deployments when service attributes change by @fujiwara in https://github.com/kayac/ecspresso/pull/961
- Bump github.com/fujiwara/tfstate-lookup from 1.8.1 to 1.10.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/947
- Bump google.golang.org/grpc from 1.76.0 to 1.79.3 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/965
- Add skills subcommand using skillsmith by @fujiwara in https://github.com/kayac/ecspresso/pull/966
- Bump go.opentelemetry.io/otel/sdk from 1.38.0 to 1.40.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/964
- Bump dependencies: AWS SDK, Go modules, and GitHub Actions by @fujiwara in https://github.com/kayac/ecspresso/pull/967
- Bump docker/login-action from 3.6.0 to 3.7.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/954
- Reduce binary size by disabling unused tfstate backends by @fujiwara in https://github.com/kayac/ecspresso/pull/968

## [v2.7.1](https://github.com/kayac/ecspresso/compare/v2.7.0...v2.7.1) - 2026-02-05
- add debug logs by @fujiwara in https://github.com/kayac/ecspresso/pull/930
- fix verify to parse IAM Trust Policy with array format Principal.Service by @fujiwara in https://github.com/kayac/ecspresso/pull/932

## [v2.7.0](https://github.com/kayac/ecspresso/compare/v2.6.5...v2.7.0) - 2025-12-13
- Docker image by @fujiwara in https://github.com/kayac/ecspresso/pull/913
- release docker images by @fujiwara in https://github.com/kayac/ecspresso/pull/915
- Bump actions/setup-go from 5.5.0 to 6.1.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/911
- Bump actions/checkout from 4.2.2 to 6.0.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/910
- Bump Songmu/tagpr from 1.8.4 to 1.9.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/881
- Bump Go modules by @fujiwara in https://github.com/kayac/ecspresso/pull/916
- Supports Express mode with definition files. by @fujiwara in https://github.com/kayac/ecspresso/pull/905
- fix: modernize by @fujiwara in https://github.com/kayac/ecspresso/pull/917
- update to tfstate-lookup v1.8.1 by @fujiwara in https://github.com/kayac/ecspresso/pull/918
- docs: update README with Express mode, table of contents, and Docker image by @fujiwara in https://github.com/kayac/ecspresso/pull/919

## [v2.6.5](https://github.com/kayac/ecspresso/compare/v2.6.4...v2.6.5) - 2025-11-28
- use service.CurrentServiceDeployment for finding current deployment. by @fujiwara in https://github.com/kayac/ecspresso/pull/903
- Supports ECS Express mode by @fujiwara in https://github.com/kayac/ecspresso/pull/902
- Bye-bye remaining `interface{}` by @mi-wada in https://github.com/kayac/ecspresso/pull/907
- verify: Support digest-based image reference by @mi-wada in https://github.com/kayac/ecspresso/pull/908
- Refactor verify parseImageURL by @fujiwara in https://github.com/kayac/ecspresso/pull/909

## [v2.6.4](https://github.com/kayac/ecspresso/compare/v2.6.3...v2.6.4) - 2025-11-21
- Improve deployment strategy log message to support all strategies by @fujiwara in https://github.com/kayac/ecspresso/pull/891
- fix(diff): When MaximumPercent or MinimumHealthyPercent is nil, set default by @mi-wada in https://github.com/kayac/ecspresso/pull/895
- fix tests of #896 - Bump golang.org/x/crypto from 0.39.0 to 0.45.0 by @fujiwara in https://github.com/kayac/ecspresso/pull/900
- Bump golang.org/x/crypto from 0.39.0 to 0.45.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/896
- fix tiny typo: "use can set" -> "you can set" by @mi-wada in https://github.com/kayac/ecspresso/pull/893
- doc: fix asdf command in README.md by @waneal in https://github.com/kayac/ecspresso/pull/898
- Supports `aws login` - update to aws-sdk-go-v2 v1.40.0 by @fujiwara in https://github.com/kayac/ecspresso/pull/901

## [v2.6.3](https://github.com/kayac/ecspresso/compare/v2.6.2...v2.6.3) - 2025-10-31
- Update aws-sdk-go-v2/service/ecs to v1.67.0 by @fujiwara in https://github.com/kayac/ecspresso/pull/889
- show serviceRevisionsSummaries in waiting. by @fujiwara in https://github.com/kayac/ecspresso/pull/890

## [v2.6.2](https://github.com/kayac/ecspresso/compare/v2.6.1...v2.6.2) - 2025-10-26
- Add support for waiting on specific CodeDeploy lifecycle events by @moznion in https://github.com/kayac/ecspresso/pull/884
- refactor parsing --wait-until=codedeploy: by @fujiwara in https://github.com/kayac/ecspresso/pull/886
- all: run modernize happy by @zchee in https://github.com/kayac/ecspresso/pull/883

## [v2.6.1](https://github.com/kayac/ecspresso/compare/v2.6.0...v2.6.1) - 2025-09-21
- Immutable release by @fujiwara in https://github.com/kayac/ecspresso/pull/878
- add args input to run ecspresso after installation by @fujiwara in https://github.com/kayac/ecspresso/pull/880

## [v2.6.0](https://github.com/kayac/ecspresso/compare/v2.5.0...v2.6.0) - 2025-07-24
- add retry option for better reliability by @bary822 in https://github.com/kayac/ecspresso/pull/847
- Fix service connect configuration removal on deployment by @itkq in https://github.com/kayac/ecspresso/pull/856
- Supports ECS Blue Green deployment strategy. by @fujiwara in https://github.com/kayac/ecspresso/pull/861
- Bump golang.org/x/oauth2 from 0.16.0 to 0.27.0 by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/862
- docs: Add rollback command documentation to README by @fujiwara in https://github.com/kayac/ecspresso/pull/863
- Refactor verify command by @fujiwara in https://github.com/kayac/ecspresso/pull/864
- Bump the aws-sdk-go-v2 group across 1 directory with 15 updates by @dependabot[bot] in https://github.com/kayac/ecspresso/pull/865
- Update to tfstate lookup v1.7.0 by @fujiwara in https://github.com/kayac/ecspresso/pull/866
- Fix deployment log message for ECS B/G Deploy by @csenet in https://github.com/kayac/ecspresso/pull/867
- enables to remove load balancers from ECS service. by @fujiwara in https://github.com/kayac/ecspresso/pull/868
- Change the default of deploy --wait-until from "stable" to "deployed" by @fujiwara in https://github.com/kayac/ecspresso/pull/869
- show any deployment statuses when the status changed. by @fujiwara in https://github.com/kayac/ecspresso/pull/872
- fix: findActiveECSDeployment by @fujiwara in https://github.com/kayac/ecspresso/pull/873
- refresh waits until deployed internally by @fujiwara in https://github.com/kayac/ecspresso/pull/874

## [v2.5.0](https://github.com/kayac/ecspresso/compare/v2.4.6...v2.5.0) - 2025-05-10
- Pin external actions to mitigate supply chain attacks by @mashiike in https://github.com/kayac/ecspresso/pull/816
- Bump github.com/golang-jwt/jwt/v4 from 4.5.1 to 4.5.2 by @dependabot in https://github.com/kayac/ecspresso/pull/819
- Bump github.com/golang-jwt/jwt/v5 from 5.2.1 to 5.2.2 by @dependabot in https://github.com/kayac/ecspresso/pull/818
- Bump golang.org/x/crypto from 0.32.0 to 0.35.0 by @dependabot in https://github.com/kayac/ecspresso/pull/826
- Feature Request: new option: wait for Service Deployment, not for Service Stable by @sugitak in https://github.com/kayac/ecspresso/pull/821
- Add deploy --wait-until option. by @fujiwara in https://github.com/kayac/ecspresso/pull/828
- set WaitUntil for internal use. by @fujiwara in https://github.com/kayac/ecspresso/pull/830
- Bump the aws-sdk-go-v2 group across 1 directory with 16 updates by @dependabot in https://github.com/kayac/ecspresso/pull/812
- Add external plugin by @fujiwara in https://github.com/kayac/ecspresso/pull/760
- Add deployment_config_name to config of CodeDeploy. by @fujiwara in https://github.com/kayac/ecspresso/pull/829
- switch a logger to log/slog. by @fujiwara in https://github.com/kayac/ecspresso/pull/834
- Add JSON format support for logs and outputs by @fujiwara in https://github.com/kayac/ecspresso/pull/835
- replace time.Sleep with sleepContext for handling context cancellation by @fujiwara in https://github.com/kayac/ecspresso/pull/836
- refactor: improve verify module to use contextualized state by @fujiwara in https://github.com/kayac/ecspresso/pull/837
- feat: add JSON support for AppSpec by @fujiwara in https://github.com/kayac/ecspresso/pull/838

## [v2.4.6](https://github.com/kayac/ecspresso/compare/v2.4.5...v2.4.6) - 2025-03-01
- Bump golang.org/x/crypto from 0.24.0 to 0.31.0 by @dependabot in https://github.com/kayac/ecspresso/pull/779
- update golang.org/x/net v0.34.0 by @fujiwara in https://github.com/kayac/ecspresso/pull/782
- Bump github.com/fujiwara/tfstate-lookup from 1.3.2 to 1.4.2 by @dependabot in https://github.com/kayac/ecspresso/pull/753
- Bump the aws-sdk-go-v2 group across 1 directory with 16 updates by @dependabot in https://github.com/kayac/ecspresso/pull/786
- use hosted arm runner. by @fujiwara in https://github.com/kayac/ecspresso/pull/791
- Bump github.com/hashicorp/go-slug from 0.15.0 to 0.16.3 by @dependabot in https://github.com/kayac/ecspresso/pull/790
- update modules by @fujiwara in https://github.com/kayac/ecspresso/pull/793
- fix nightly build by @fujiwara in https://github.com/kayac/ecspresso/pull/801
- fix: add support for AWS China ECR URLs by @litanyofmadness in https://github.com/kayac/ecspresso/pull/806
- test: add test case for AWS China ECR image by @fujiwara in https://github.com/kayac/ecspresso/pull/808
- remove healthCheckGracePeriodSeconds validation by @ijin in https://github.com/kayac/ecspresso/pull/805
- use Songmu/tagpr for release management. by @fujiwara in https://github.com/kayac/ecspresso/pull/809

## [v2.4.5](https://github.com/kayac/ecspresso/compare/v2.4.4...v2.4.5) - 2024-12-09
- Update aws-sdk-go-v2/service/ecs to v1.51.0 and go mod tidy by @t-kikuc in https://github.com/kayac/ecspresso/pull/774
- Supports AvailabilityZoneRebalancing. by @fujiwara in https://github.com/kayac/ecspresso/pull/778

## [v2.4.4](https://github.com/kayac/ecspresso/compare/v2.4.3...v2.4.4) - 2024-11-19
- Supports VPC Lattice integration. by @fujiwara in https://github.com/kayac/ecspresso/pull/773

## [v2.4.3](https://github.com/kayac/ecspresso/compare/v2.4.2...v2.4.3) - 2024-11-06
- fix panic on run --dry-run by @fujiwara in https://github.com/kayac/ecspresso/pull/767
- Bump github.com/golang-jwt/jwt/v4 from 4.5.0 to 4.5.1 by @dependabot in https://github.com/kayac/ecspresso/pull/766
- fix null pointer exception by @fujiwara in https://github.com/kayac/ecspresso/pull/768

## [v2.4.2](https://github.com/kayac/ecspresso/compare/v2.4.1...v2.4.2) - 2024-10-21
- returns error if --watch-container is not found. by @fujiwara in https://github.com/kayac/ecspresso/pull/750
- Bump github.com/aws/aws-sdk-go-v2/service/ecs to v1.47.4 by @fujiwara in https://github.com/kayac/ecspresso/pull/758

## [v2.4.1](https://github.com/kayac/ecspresso/compare/v2.4.0...v2.4.1) - 2024-08-30
- Fix typo: `task` -> `tasks` by @t-kikuc in https://github.com/kayac/ecspresso/pull/743
- fix resolving name of service connect namespace. by @fujiwara in https://github.com/kayac/ecspresso/pull/745

## [v2.4.0](https://github.com/kayac/ecspresso/compare/v2.3.6...v2.4.0) - 2024-08-06
- Bump github.com/aws/smithy-go from 1.20.2 to 1.20.3 by @dependabot in https://github.com/kayac/ecspresso/pull/711
- Add Jsonnet native functions by @fujiwara in https://github.com/kayac/ecspresso/pull/702
- run --revision and --latest-task-definition are exclusive by @fujiwara in https://github.com/kayac/ecspresso/pull/719
- remove fallback to ssm.GetParmater API by @fujiwara in https://github.com/kayac/ecspresso/pull/720
- bump versions by @fujiwara in https://github.com/kayac/ecspresso/pull/721
- add disables colorized output option by @ch1aki in https://github.com/kayac/ecspresso/pull/718
- adds test for #718 by @fujiwara in https://github.com/kayac/ecspresso/pull/725
- Add diff --external. Runs external diff command. by @fujiwara in https://github.com/kayac/ecspresso/pull/727
- Add ignore.tags into a configuration. by @fujiwara in https://github.com/kayac/ecspresso/pull/728
- Fix/retry registry by @fujiwara in https://github.com/kayac/ecspresso/pull/729
- Clarify README by @ijin in https://github.com/kayac/ecspresso/pull/731
- Exit non-zero status when deployment is rolled back. by @fujiwara in https://github.com/kayac/ecspresso/pull/733
- Bump the aws-sdk-go-v2 group across 1 directory with 15 updates by @dependabot in https://github.com/kayac/ecspresso/pull/735
- Bump github.com/schollz/progressbar/v3 from 3.13.1 to 3.14.6 by @dependabot in https://github.com/kayac/ecspresso/pull/734
- Bump github.com/goccy/go-yaml from 1.9.5 to 1.12.0 by @dependabot in https://github.com/kayac/ecspresso/pull/724
- Bump github.com/opencontainers/image-spec from 1.0.2 to 1.1.0 by @dependabot in https://github.com/kayac/ecspresso/pull/686
- Bump github.com/kayac/go-config from 0.6.0 to 0.7.0 by @dependabot in https://github.com/kayac/ecspresso/pull/648

## [v2.3.6](https://github.com/kayac/ecspresso/compare/v2.3.5...v2.3.6) - 2024-07-17
- --rm-dist has been deprecated in favor of --clean by @shogo82148 in https://github.com/kayac/ecspresso/pull/712
- fix: verify ssm secrets from SSM parameters. by @fujiwara in https://github.com/kayac/ecspresso/pull/713
- Add pidMode mapping to tdToTaskDefinitionInput function by @ch1aki in https://github.com/kayac/ecspresso/pull/715
- Bump goreleaser/goreleaser-action from 5 to 6 by @dependabot in https://github.com/kayac/ecspresso/pull/709
- Bump github.com/hashicorp/go-retryablehttp from 0.7.1 to 0.7.7 by @dependabot in https://github.com/kayac/ecspresso/pull/707
- Add IpcMode into TaskDefinitionInput. by @fujiwara in https://github.com/kayac/ecspresso/pull/716
- Bump github.com/Azure/azure-sdk-for-go/sdk/azidentity from 1.3.1 to 1.6.0 by @dependabot in https://github.com/kayac/ecspresso/pull/701

## [v2.3.5](https://github.com/kayac/ecspresso/compare/v2.3.4...v2.3.5) - 2024-06-21
- update aws-sdk-go-v2/service/ecs to v1.43.1 by @stkhr in https://github.com/kayac/ecspresso/pull/704
- Bump the aws-sdk-go-v2 group across 1 directory with 13 updates by @dependabot in https://github.com/kayac/ecspresso/pull/705

## [v2.3.4](https://github.com/kayac/ecspresso/compare/v2.3.3...v2.3.4) - 2024-05-23
- Add exec -L flag. by @fujiwara in https://github.com/kayac/ecspresso/pull/690
- Bump golang.org/x/net from 0.17.0 to 0.23.0 by @dependabot in https://github.com/kayac/ecspresso/pull/689
- Supports arm64 by actions.yml by @fujiwara in https://github.com/kayac/ecspresso/pull/693
- fix: panic when secrets.name or valueFrom is missing by @fujiwara in https://github.com/kayac/ecspresso/pull/697
- fix: add missing space between words in run command log messages by @nao23 in https://github.com/kayac/ecspresso/pull/698

## [v2.3.3](https://github.com/kayac/ecspresso/compare/v2.3.2...v2.3.3) - 2024-03-29
- Allow to specify os,arch of binary to install in CircleCI's orb by @tomiyan in https://github.com/kayac/ecspresso/pull/666
- Add revision option to deploy command by @tksx1227 in https://github.com/kayac/ecspresso/pull/672
- Fix wait for rollbacked deployment with CodeDeploy. by @fujiwara in https://github.com/kayac/ecspresso/pull/673
- refactor test/ci by @fujiwara in https://github.com/kayac/ecspresso/pull/674
- Set shorten waiter max delay. by @fujiwara in https://github.com/kayac/ecspresso/pull/675
- Bump google.golang.org/protobuf from 1.30.0 to 1.33.0 by @dependabot in https://github.com/kayac/ecspresso/pull/676
- Bump actions/setup-go from 4 to 5 by @dependabot in https://github.com/kayac/ecspresso/pull/653
- fix: use GetParameters instead of GetParameter to simulate actual ECS' behavior by @aereal in https://github.com/kayac/ecspresso/pull/678
- do rollbackTaskDefinition if a rollbacking deployment was completed. by @fujiwara in https://github.com/kayac/ecspresso/pull/679
- Go 1.22 by @fujiwara in https://github.com/kayac/ecspresso/pull/680
- fallback to ssm.GetParameter if failed to ssm.GetParameters by @fujiwara in https://github.com/kayac/ecspresso/pull/681

## [v2.3.2](https://github.com/kayac/ecspresso/compare/v2.3.1...v2.3.2) - 2024-01-19
- update aws-sdk-go-v2/service/ecs to v1.37.0 by @fujiwara in https://github.com/kayac/ecspresso/pull/656
- Fix typo by @itkq in https://github.com/kayac/ecspresso/pull/654
- fix verify ECR images in another region. by @fujiwara in https://github.com/kayac/ecspresso/pull/660
- Supports managing EBS Volumes by ECS services/tasks. by @fujiwara in https://github.com/kayac/ecspresso/pull/659
- verify multiple tag specifications for ManagedEBSVolume by @fujiwara in https://github.com/kayac/ecspresso/pull/661
- Add caching for secretsmanager_arn function by @fujiwara in https://github.com/kayac/ecspresso/pull/662

## [v2.3.1](https://github.com/kayac/ecspresso/compare/v2.3.0...v2.3.1) - 2023-12-25
- update tfstate-lookup v1.1.6 by @fujiwara in https://github.com/kayac/ecspresso/pull/646
- add tfstate testing by @fujiwara in https://github.com/kayac/ecspresso/pull/647

## [v2.3.0](https://github.com/kayac/ecspresso/compare/v2.2.4...v2.3.0) - 2023-12-21
- docs: add the installation guide with aqua by @suzuki-shunsuke in https://github.com/kayac/ecspresso/pull/616
- Bump golang.org/x/net from 0.14.0 to 0.17.0 by @dependabot in https://github.com/kayac/ecspresso/pull/617
- update aws-sdk-go-v2/service/ecs v1.33.0 by @fujiwara in https://github.com/kayac/ecspresso/pull/624
- Bump goreleaser/goreleaser-action from 4 to 5 by @dependabot in https://github.com/kayac/ecspresso/pull/610
- add run --client-token by @fujiwara in https://github.com/kayac/ecspresso/pull/631
- add secretsmanager plugin by @fujiwara in https://github.com/kayac/ecspresso/pull/618
- diff command works whenever a remote service or a task definition are not found. by @fujiwara in https://github.com/kayac/ecspresso/pull/632
- Enables to override timeout in a configuration file by --timeout. by @fujiwara in https://github.com/kayac/ecspresso/pull/633
- ECSPRESSO_FILTER_COMMAND moves to cli flag. by @fujiwara in https://github.com/kayac/ecspresso/pull/634
- fix diff output compared with nil by @fujiwara in https://github.com/kayac/ecspresso/pull/635
- Bump github.com/alecthomas/kong from 0.7.0 to 0.8.1 by @dependabot in https://github.com/kayac/ecspresso/pull/623
- Bump github.com/fatih/color from 1.13.0 to 1.16.0 by @dependabot in https://github.com/kayac/ecspresso/pull/629
- Bump google.golang.org/grpc from 1.49.0 to 1.56.3 by @dependabot in https://github.com/kayac/ecspresso/pull/619
- update aws-sdk-go-v2 and ecsta by @fujiwara in https://github.com/kayac/ecspresso/pull/636
- go 1.21 by @fujiwara in https://github.com/kayac/ecspresso/pull/640
- Default plugins (ssm and secretsmanager) by @fujiwara in https://github.com/kayac/ecspresso/pull/641
- Refactoring options by @fujiwara in https://github.com/kayac/ecspresso/pull/643
- V2.3 by @fujiwara in https://github.com/kayac/ecspresso/pull/642
- Bump golang.org/x/crypto from 0.14.0 to 0.17.0 by @dependabot in https://github.com/kayac/ecspresso/pull/644

## [v2.2.4](https://github.com/kayac/ecspresso/compare/v2.2.3...v2.2.4) - 2023-10-06
- fix: conversion typo by @testwill in https://github.com/kayac/ecspresso/pull/607
- Bump actions/checkout from 3 to 4 by @dependabot in https://github.com/kayac/ecspresso/pull/609
- update tfstate-lookup v1.1.4 by @fujiwara in https://github.com/kayac/ecspresso/pull/613
- fix typo escpresso -> ecspresso by @Kiryuanzu in https://github.com/kayac/ecspresso/pull/612

## [v2.2.3](https://github.com/kayac/ecspresso/compare/v2.2.2...v2.2.3) - 2023-08-04
- add init --task-definition flag. by @fujiwara in https://github.com/kayac/ecspresso/pull/593
- Remove runningCount and pendingCount from generated service definition. by @fujiwara in https://github.com/kayac/ecspresso/pull/594
- Add next step and template syntax section by @fujiwara in https://github.com/kayac/ecspresso/pull/600
- An output of task-definition of init command to stable. by @fujiwara in https://github.com/kayac/ecspresso/pull/601

## [v2.2.2](https://github.com/kayac/ecspresso/compare/v2.2.1...v2.2.2) - 2023-07-19
- Bump goreleaser/goreleaser-action from 3 to 4 by @dependabot in https://github.com/kayac/ecspresso/pull/577
- add "Supported tfstate URL format" by @fujiwara in https://github.com/kayac/ecspresso/pull/588
- update to ecsta v0.3.2 by @fujiwara in https://github.com/kayac/ecspresso/pull/589
- update to ecsta v0.3.3 by @fujiwara in https://github.com/kayac/ecspresso/pull/590
- Fix DesiredCount ignoring by @HASHIMOTO-Takafumi in https://github.com/kayac/ecspresso/pull/591
- Add tests for #591 by @fujiwara in https://github.com/kayac/ecspresso/pull/592

## [v2.2.1](https://github.com/kayac/ecspresso/compare/v2.2.0...v2.2.1) - 2023-06-19
- fix typo `--autos-caling` -> `--auto-scaling` by @sinsoku in https://github.com/kayac/ecspresso/pull/570
- use t.Setenv() in tests. by @fujiwara in https://github.com/kayac/ecspresso/pull/576
- update ecsta v0.3.1 by @fujiwara in https://github.com/kayac/ecspresso/pull/583
- Fix/verify errors on create log group by @fujiwara in https://github.com/kayac/ecspresso/pull/584
- Bump actions/setup-go from 3 to 4 by @dependabot in https://github.com/kayac/ecspresso/pull/578

## [v2.2.0](https://github.com/kayac/ecspresso/compare/v2.1.0...v2.2.0) - 2023-05-26
- Update README for asdf plugin by @koluku in https://github.com/kayac/ecspresso/pull/544
- Bump golang.org/x/text from 0.3.7 to 0.3.8 by @dependabot in https://github.com/kayac/ecspresso/pull/517
- create a log group when awslogs-create-group=="true" on verify by @fujiwara in https://github.com/kayac/ecspresso/pull/541
- Bump golang.org/x/net from 0.0.0-20220909164309-bea034e7d591 to 0.7.0 by @dependabot in https://github.com/kayac/ecspresso/pull/518
- Add example with terraform by @fujiwara in https://github.com/kayac/ecspresso/pull/556
- Fix typo: latst -> latest by @KOBA789 in https://github.com/kayac/ecspresso/pull/558
- Add deregister --revision=latest and --delete flag. by @fujiwara in https://github.com/kayac/ecspresso/pull/560
- Fix/tests/terraform by @fujiwara in https://github.com/kayac/ecspresso/pull/557
- Fix: Unable to update service with tags. by @fujiwara in https://github.com/kayac/ecspresso/pull/551
- implements deploy/scale --auto-scaling-(min|max) option. by @fujiwara in https://github.com/kayac/ecspresso/pull/550
- Install Ecspresso into toolchain cache directory by @goruha in https://github.com/kayac/ecspresso/pull/566
- Install Ecspresso into toolchain cache directory, show installed versions by @fujiwara in https://github.com/kayac/ecspresso/pull/567
- Refactor options type by @fujiwara in https://github.com/kayac/ecspresso/pull/565
- Bump github.com/schollz/progressbar/v3 from 3.11.0 to 3.13.1 by @dependabot in https://github.com/kayac/ecspresso/pull/535
- Bump github.com/aws/aws-sdk-go-v2/credentials from 1.13.15 to 1.13.24 by @dependabot in https://github.com/kayac/ecspresso/pull/561
- Bump github.com/aws/aws-sdk-go-v2/service/ecr from 1.17.17 to 1.18.11 by @dependabot in https://github.com/kayac/ecspresso/pull/564
- Bump github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 from 1.18.18 to 1.19.11 by @dependabot in https://github.com/kayac/ecspresso/pull/563
- fix nil pointer dereference by @fujiwara in https://github.com/kayac/ecspresso/pull/569

## [v2.1.0](https://github.com/kayac/ecspresso/compare/v2.0.5...v2.1.0) - 2023-03-31
- change log level in verifyLogConfiguration by @fujiwara in https://github.com/kayac/ecspresso/pull/529
- update to ecsta@v0.3.0 by @fujiwara in https://github.com/kayac/ecspresso/pull/531
- fix: verify SecretsManager JSON Key by @m22r in https://github.com/kayac/ecspresso/pull/533
- Add `--assume-role-arn` option to the top level of the CLI by @moznion in https://github.com/kayac/ecspresso/pull/530
- exit 1 when run a task failed to start. by @fujiwara in https://github.com/kayac/ecspresso/pull/537
- Add --assume-role-arn option by @fujiwara in https://github.com/kayac/ecspresso/pull/538
- fix map2str to stable. by @fujiwara in https://github.com/kayac/ecspresso/pull/542
- fix documents links by @fujiwara in https://github.com/kayac/ecspresso/pull/543
- ecspresso revisons --revision (current|latest|[number]) by @fujiwara in https://github.com/kayac/ecspresso/pull/539

## [v2.0.5](https://github.com/kayac/ecspresso/compare/v2.0.3...v2.0.5) - 2023-03-03
- nightly branch based on v2 by @fujiwara in https://github.com/kayac/ecspresso/pull/501
- Wait a service stable after create service. by @fujiwara in https://github.com/kayac/ecspresso/pull/502
- fix(action): authenticate the API calls for increasing API rate limit by @aereal in https://github.com/kayac/ecspresso/pull/499
- fix v2 action testing by @fujiwara in https://github.com/kayac/ecspresso/pull/503
- Fix supend / resume autoscaling by @fujiwara in https://github.com/kayac/ecspresso/pull/497
- returns ErrNotFound when CodeDeploy resources are not found. by @fujiwara in https://github.com/kayac/ecspresso/pull/504
- Fix for confusing flags of run. by @fujiwara in https://github.com/kayac/ecspresso/pull/505
- add delete --terminate flag. by @fujiwara in https://github.com/kayac/ecspresso/pull/512
- Fujiwara/logutils@v1.1.1 by @fujiwara in https://github.com/kayac/ecspresso/pull/516
- update aws-sdk-go-v2/config to fix SSO configuration issue by @enm10k in https://github.com/kayac/ecspresso/pull/520
- Output JSON message without using logger. by @fujiwara in https://github.com/kayac/ecspresso/pull/514
- Cache results of verified resources. by @fujiwara in https://github.com/kayac/ecspresso/pull/515
- add delete --terminate to test cases. by @fujiwara in https://github.com/kayac/ecspresso/pull/526
- bump go version 1.20 by @fujiwara in https://github.com/kayac/ecspresso/pull/527

## [v2.0.4](https://github.com/kayac/ecspresso/compare/v2.0.3...v2.0.4) - 2023-02-03
- nightly branch based on v2 by @fujiwara in https://github.com/kayac/ecspresso/pull/501
- Wait a service stable after create service. by @fujiwara in https://github.com/kayac/ecspresso/pull/502
- fix(action): authenticate the API calls for increasing API rate limit by @aereal in https://github.com/kayac/ecspresso/pull/499
- fix v2 action testing by @fujiwara in https://github.com/kayac/ecspresso/pull/503
- Fix supend / resume autoscaling by @fujiwara in https://github.com/kayac/ecspresso/pull/497
- returns ErrNotFound when CodeDeploy resources are not found. by @fujiwara in https://github.com/kayac/ecspresso/pull/504
- Fix for confusing flags of run. by @fujiwara in https://github.com/kayac/ecspresso/pull/505
- add delete --terminate flag. by @fujiwara in https://github.com/kayac/ecspresso/pull/512
